# Geoson — High-Performance OGC Geo Engine (Design Spec)

**Date:** 2026-07-16
**Status:** Approved design
**Goal:** A drop-in GeoServer replacement built as horizontally scalable microservices, faster than GeoServer/MapServer/MapProxy, feature-complete for OGC publishing, open-source.

---

## 1. Problem & Goals

GeoServer is a Java monolith: slow startup, heavy memory, no per-protocol scaling, single-process rendering. MapServer is faster but feature-poor and not cloud-native. Organizations publish services via WMS/WFS/WMTS URLs with vendor params (`CQL_FILTER`, `TILED`, `viewparams`, …); any missing parameter breaks existing clients.

**Goals:**

1. **Drop-in compatibility** — existing GeoServer clients, URLs, vendor params, CQL filters, exception formats, and REST automation work unchanged.
2. **Per-service horizontal scaling** — scale WMS to 4 replicas and WFS to 1 with one compose flag.
3. **Best-tool-per-job** — Rust for CPU-heavy rendering/processing, Go for network-heavy request handling, Martin-style MVT serving, DuckDB + GeoParquet for read-only analytics data.
4. **Feature parity** — stores, styles, users/groups, GeoFence-style rules, tile caching, format conversion, seeding.
5. **Open-source quality** — docs, clean architecture, benchmarks vs GeoServer/MapServer.

**Non-goals (v1):** OIDC/LDAP (v2), Kubernetes/Helm (v1.5 — Compose + Swarm ship in v1), WCS, CSW, clustering of catalog writes (single Postgres primary is fine).

---

## 2. Architecture Overview

Two shared toolkits + thin stateless services. Each service is its own container with independent replica count behind Traefik.

```
                        ┌──────────────┐
   clients ──────────▶  │   Traefik    │  (LB, TLS, sticky-free)
 (QGIS, Leaflet,        └──────┬───────┘
  OpenLayers, curl)            │
                        ┌──────▼───────┐
                        │   gateway    │  Go — OWS front door
                        └──┬─┬─┬─┬─┬───┘
              ┌────────────┘ │ │ │ └─────────────┐
        ┌─────▼────┐ ┌───────▼─┐ ┌▼────────┐ ┌───▼─────┐
        │   wms    │ │   wfs   │ │  tiles  │ │   wps   │
        │  (Rust)  │ │  (Go)   │ │ (Rust)  │ │ (Rust)  │
        └─────┬────┘ └────┬────┘ └───┬─────┘ └───┬─────┘
              │           │          │           │
        ┌─────▼───────────▼──────────▼───────────▼─────┐
        │  catalog (Go)   auth (Go)   convert (Rust)   │
        └─────┬───────────────┬────────────┬───────────┘
        ┌─────▼─────┐  ┌──────▼──────┐  ┌──▼───┐  ┌──────┐
        │ Postgres  │  │    Redis    │  │ NATS │  │MinIO*│
        │ +PostGIS  │  │(tile idx,   │  │(events│ │(opt) │
        │           │  │ sessions)   │  │ jobs) │ │      │
        └───────────┘  └─────────────┘  └──────┘  └──────┘
```

### Shared toolkits (kill duplication)

- **`libs/ogc-kit` (Go):** KVP/XML request parsing, OGC Filter + CQL/ECQL parser → SQL (PostGIS + DuckDB dialects), GML 2/3/3.2 + GeoJSON encoders, OWS exception formatting, GetCapabilities builders, SRS/axis-order handling.
- **`libs/geo-core` (Rust):** geometry ops (`geo`, `geozero`), SLD 1.0/1.1 parser + evaluator, MapLibre style JSON interpreter, GeoCSS→SLD compiler, CQL parser (same grammar as Go), raster reading (GDAL bindings for GeoTIFF/COG), rendering primitives (`tiny-skia`).
- **CQL duplication risk** (Go + Rust both parse filters) is controlled by a **shared golden test corpus** (`tests/filter-corpus/`, ~500 cases: input filter → expected AST JSON + expected SQL per dialect). Both implementations must pass identical corpus in CI.

---

## 3. Services

### 3.1 `gateway` (Go)

Single front door. Responsibilities:

- URL compatibility: `/geoserver/{workspace}/{service}`, `/geoserver/ows`, `/geoserver/gwc/service/wmts`, virtual services per workspace/layer.
- KVP + POST XML parsing, service/version/request negotiation (case-insensitive params, defaulting rules matching GeoServer).
- OWS exception rendering: `ServiceExceptionReport` (WMS 1.1.1), `ows:ExceptionReport` (1.3.0/WFS 2.0), `application/json` exceptions — byte-format matched to GeoServer.
- AuthN check (JWT/basic → `auth` service), rate limiting, request logging, Prometheus metrics.
- Routes to internal services over HTTP/2 (proto: plain REST + protobuf internal DTOs where hot).

### 3.2 `catalog` (Go + Postgres)

System of record for configuration.

- Entities: workspaces, namespaces, stores, feature types, coverages, layers, layer groups, styles, settings — mirroring GeoServer's model so REST compat is natural.
- **GeoServer REST compat:** `/geoserver/rest/workspaces`, `/rest/datastores`, `/rest/layers`, `/rest/styles`, `/rest/layergroups`, `/rest/security`, XML + JSON bodies, same shapes as GeoServer (validated against geoserver-rest python lib and Terraform provider).
- Store connectors (v1): **PostGIS** (read/write, SQL views, filter pushdown), **Shapefile / GeoPackage / GeoJSON** (file upload + register), **GeoTIFF/COG** (+ image mosaic), **GeoParquet via DuckDB** (read-only, fast scans).
- Publishes config-change events on NATS (`catalog.layer.updated`, …) → services drop caches, tiles invalidates affected tiles.
- Internal clean API (`/api/v1`) consumed by frontend; `/rest` is the compat surface.

### 3.3 `auth` (Go)

- Users, groups, roles (local accounts, argon2id), JWT access tokens + refresh, GeoServer basic-auth accepted at gateway for compat.
- **GeoFence-style rule engine:** ordered rules `(user|group, service, workspace, layer, access ALLOW/DENY/LIMIT)` with LIMIT supporting CQL read/write filters, attribute restriction, and geometry (spatial clip) limits. Evaluated per request; decisions cached in Redis with event-driven invalidation.
- Admin endpoints under `/rest/security/*` (compat) + `/api/v1/auth/*`.

### 3.4 `wms` (Rust)

- WMS **1.1.1 + 1.3.0** (axis order handled per version): GetCapabilities, GetMap, GetFeatureInfo, GetLegendGraphic, DescribeLayer.
- Output: PNG/PNG8, JPEG, WebP, GIF, `image/vnd.jpeg-png`; `TRANSPARENT`, `BGCOLOR`, `DPI`/`FORMAT_OPTIONS`.
- Vendor params: `CQL_FILTER`, `FILTER`, `SLD`, `SLD_BODY`, `STYLES`, `ENV`, `TIME`/`ELEVATION`, `TILED`+`TILESORIGIN`, `viewparams`, `propertyName`, `FEATURE_COUNT`, `INFO_FORMAT` (text/plain, html, geojson, gml).
- Rendering: vector via `tiny-skia` pipeline (labels with collision detection, SLD rule scale-denominators), rasters via GDAL (COG range reads, overviews, resampling).
- Styling: SLD 1.0/1.1 native; MapLibre style JSON accepted (subset mapped to render pipeline); GeoCSS compiled to SLD at style-save time in catalog.
- Data access: SQL pushdown (bbox && geometry, CQL→WHERE) to PostGIS/DuckDB; file stores read via geo-core readers.

### 3.5 `wfs` (Go)

- WFS **1.0.0 / 1.1.0 / 2.0.0**: GetCapabilities, DescribeFeatureType, GetFeature (paging `startIndex/count`, `sortBy`, `resultType=hits`), GetPropertyValue, **WFS-T** (Insert/Update/Delete, per-store transaction support), stored queries (`GetFeatureById`).
- Output formats: GML 2/3.1/3.2, `application/json` (GeoJSON), CSV, shapefile (zip). Streaming encoders — constant memory on million-feature responses.
- Filters: OGC Filter 1.0/1.1/2.0 XML + `CQL_FILTER`, spatial + temporal operators, pushdown to PostGIS/DuckDB SQL.

### 3.6 `tiles` (Rust + Go cache layer)

- Protocols: **WMTS 1.0** (KVP + RESTful), **XYZ** (`/gwc/service/tms`… + slippy), **TMS**; GetCapabilities mirrors GeoWebCache structure.
- Vector tiles: MVT/PBF straight from PostGIS (`ST_AsMVT`) and GeoParquet/DuckDB — Martin-style architecture (Rust, per-connection pipelining).
- Raster tiles: internally calls `wms` render for cache-miss metatiles (metatiling 4×4, like GWC).
- Cache: content-addressed tile blobs on shared volume or MinIO; Redis index (existence, TTL, layer-version). Event-driven invalidation from catalog; `truncate` by layer/bbox/zoom compat endpoints.
- Seeding: seed/reseed/truncate jobs over NATS, parallel workers, resumable.
- Gridsets: EPSG:3857, EPSG:4326 defaults + custom gridset registry (GWC-compatible definitions).

### 3.7 `wps` (Rust)

- WPS **1.0**: GetCapabilities, DescribeProcess, Execute (sync + async with status polling URLs).
- Job execution over NATS queue; workers horizontally scalable; results stored to MinIO/volume with TTL.
- v1 process set: buffer, clip, intersection, union, dissolve, simplify, reproject, centroid, convex hull, area/length stats, vector→raster heatmap. Process registry designed for pluggable additions.

### 3.8 `convert` (Rust + GDAL)

- Ingest pipeline: upload (shapefile/GPKG/GeoJSON/CSV/GeoTIFF) → validate → import to PostGIS or register as file/COG store → auto-publish layer.
- Conversions: any↔any vector formats, GeoTIFF→COG, vector→GeoParquet.
- Async jobs (NATS), progress events to frontend via SSE.

### 3.9 `frontend` (Next.js 15)

Stack: Next.js 15 App Router, React 19, TypeScript, Tailwind v4, framer-motion, lucide-react, next-themes, MapLibre GL. English-first, Persian second, `[locale]` routing (fa = RTL), dark + light themes. Folder structure exactly as specified:

```
src/
  app/[locale]/
    (app)/            # authenticated shell (header + AuthGuard)
      map/            # MapLibre workspace — layer preview
      dashboard/…     # one route per section, own component per route
    login/
  api/                # ALL backend calls, feature-first
    client.ts         # fetch wrapper (base URL, token, errors)
    auth/{api,types,store}.ts
    map/layers/{api,types}.ts · map/wms.ts
    dashboard/<feature>/{api,types}.ts
  components/
    layout/ · auth/ · map/ · dashboard/{pages,settings}/ · ui/ · icons/
  i18n/ · lib/ · styles/globals.css
```

Dashboard sections (each own folder + component): overview (service health, req/s, cache hit-rate), workspaces, stores (add/edit connectors, file upload), layers (publish, styling editor with live WMS preview), layer groups, styles (SLD/MapLibre editor + legend preview), tile cache (seed/truncate, gridsets), security (users, groups, roles, GeoFence rules table), WPS jobs, conversions, settings (contact info, limits, logging).

---

## 4. Drop-in Compatibility Strategy

The compat guarantee is enforced by a **golden-file harness**, not by hope:

1. `tests/compat/` spins up real GeoServer (official docker image) + Geoson with identical data (sample PostGIS DB + shapefiles + rasters).
2. A corpus of recorded requests (every service, version, vendor param, CQL variant, exception path) is fired at both.
3. Responses diffed: XML canonicalized then compared structurally; images compared perceptually (SSIM threshold); JSON deep-diffed with tolerance rules (coordinate precision).
4. CI gate: no release if compat corpus regresses.

Compat surface checklist: URL paths (incl. virtual services `/geoserver/{ws}/{layer}/wms`), case-insensitive KVP, all vendor params listed per service above, exception XML byte formats, GetCapabilities structure, `/rest` API shapes, basic-auth.

---

## 5. Scaling & HA

- **All request-path services stateless.** State lives in Postgres/Redis/NATS/object storage only.
- Compose: `docker compose up -d --scale wms=4 --scale wfs=1 --scale tiles=2` — Traefik discovers replicas via docker provider, round-robins.
- Swarm: `docker stack deploy` stack file with `deploy.replicas`, `max_replicas_per_node`, rolling updates, restart policies — multi-node HA in v1.
- Tile seeding coordination via Redis locks (no duplicate metatile renders across replicas).
- Health: `/healthz` (liveness) + `/readyz` (deps) per service; graceful drain on SIGTERM.
- Observability: Prometheus metrics per service, Grafana dashboards (req/s, latency p50/p99, render time, cache hit rate, DB pool), structured JSON logs, optional OTel traces.
- Config: env vars + single `geoson.yaml`; secrets via docker secrets.

---

## 6. Repository Layout (monorepo)

```
geoson/
  services/
    gateway/  catalog/  auth/  wfs/        # Go
    wms/  tiles/  wps/  convert/           # Rust (cargo workspace)
  libs/
    ogc-kit/                               # Go shared toolkit
    geo-core/                              # Rust shared crates
  frontend/                                # Next.js 15
  deploy/
    compose/            # docker-compose.yml + compose.scale.yml + traefik
    swarm/              # stack file
  tests/
    filter-corpus/      # golden CQL/Filter cases (lang-agnostic JSON)
    compat/             # GeoServer diff harness
  docs/                 # architecture, ops/scaling, API, per-service docs
  task.md               # sprint plan — resume point across sessions
```

---

## 7. Error Handling Principles

- Every client-facing error → correct OWS exception format for the negotiated service/version (never a bare 500 with stack trace).
- Internal service-to-service errors carry typed codes; gateway maps to OWS exceptions.
- Data-store failures degrade per-layer (capabilities still serve; broken layer returns layer-specific exception) — matching GeoServer behavior.
- WPS/convert jobs: failures recorded with message, retryable, visible in frontend.

## 8. Testing Strategy

- **Unit:** per-lib (parsers, encoders, SLD evaluation, rule engine) — TDD.
- **Filter corpus:** Go + Rust both pass identical golden cases (CI).
- **Compat harness:** structural/perceptual diff vs real GeoServer (CI, release gate).
- **Integration:** compose-based e2e (publish layer via /rest → GetMap → GetFeature → tile → truncate).
- **Load:** k6 scenarios; published benchmark vs GeoServer + MapServer (same data, same hardware) in docs.

## 9. Execution Plan — 10 Sprints (`task.md`)

Work proceeds sprint-by-sprint; `task.md` checkboxes are the resume point for any new session.

1. **Skeleton & infra** — monorepo layout, compose (Traefik/Postgres/Redis/NATS/MinIO), CI, docs scaffold, healthz conventions.
2. **Catalog + stores** — data model, `/rest` compat, PostGIS/file/COG/GeoParquet connectors, NATS events.
3. **Gateway** — OWS parsing, version negotiation, exception formats, routing, metrics.
4. **Auth + GeoFence** — users/groups/roles, JWT, rule engine, `/rest/security`.
5. **WFS** — all versions, filters/CQL pushdown, GML/GeoJSON/CSV/SHP streaming, WFS-T, filter corpus (Go side).
6. **WMS** — render engine, SLD/MapLibre/GeoCSS, GetMap/FeatureInfo/Legend, rasters, filter corpus (Rust side).
7. **Tiles** — WMTS/XYZ/TMS, MVT from PostGIS+DuckDB, cache + invalidation + seeding, gridsets.
8. **WPS + convert** — process engine, job queue, v1 processes, ingest/conversion pipeline.
9. **Frontend** — full admin per §3.9, i18n fa/en, themes, map workspace.
10. **Compat + scale + release** — GeoServer diff harness green, load benchmarks, scaling/ops docs, README, LICENSE, open-source polish.

## 10. Open Decisions (defaults chosen, changeable)

- License: **Apache-2.0** (permissive, org-friendly).
- Internal transport: REST/JSON with protobuf on hot paths only if profiling demands.
- GeoCSS: compiled to SLD at save time (no runtime GeoCSS evaluation).
