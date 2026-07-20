# Giti — a high-performance, OGC geospatial engine

**Giti** (Persian: گیتی, "the world") is a fast, horizontally-scalable, drop-in
alternative to GeoServer, built as focused microservices in **Rust** (CPU-heavy
rendering) and **Go** (network-heavy services) over **PostGIS**, with a modern
**Next.js** admin console.

Publish data, serve standard OGC services, style layers visually, and manage the
whole thing from a clean web UI — faster and lighter than the Java stack.

> Status: active development. Core OGC serving + the enterprise admin console are
> working. See `docs/feature-parity.md` for the full GeoServer parity matrix.

---

## Features

**OGC services**
- **WMS** 1.1.1 / 1.3.0 — GetMap, GetFeatureInfo, GetCapabilities, GetLegendGraphic
  - **On-the-fly reprojection** — request any CRS; data reprojected via PostGIS
  - **SLD** styling with rule filters, scale (zoom) ranges, and **labels**
    (Vazirmatn + rustybuzz shaping → correct Persian/Arabic + RTL), halo, collision
- **WFS** 1.0 / 1.1 / 2.0 — GetFeature, DescribeFeatureType, output formats
- **WMTS / XYZ / TMS** vector tiles (MVT via `ST_AsMVT`) with a content-addressed cache
- **WPS** 1.0 processes; **OGC API - Features** (GeoJSON collections/items)
- GeoServer-compatible URLs, vendor params (`CQL_FILTER`, `STYLES`, …) and `/rest` API

**Data & catalog**
- Connect-anywhere store wizard: PostGIS, Shapefile, GeoPackage, GeoJSON,
  GeoParquet, CSV, KML/KMZ (+ MS SQL / cascade WMS-WMTS entries)
- **Upload GeoJSON** → ingested into PostGIS → instantly servable
- Full layer editing: title/keywords/metadata, native+declared SRS, bbox compute,
  feature-type details; **layer groups**, workspaces, stores

**Styling**
- **Visual style builder** — thematic rules (e.g. `pop > 100000 → red`), colors,
  point marks, **labels** (font/fill/opacity/halo), zoom ranges — no SLD by hand
- Multiple styles per layer; request one with `&STYLES=<name>`; SLD/CSS/YSLD/MBStyle
  code editor with validate

**Admin console** (Next.js 15, App Router)
- Modern shell: grouped nav, command palette (⌘K), drawers, toasts
- Stores / Layers / Layer Groups / Styles / Tile Cache / Security / Settings / Status
- **Layer preview** on an interactive map with selectable basemaps (OSM / Carto /
  Esri / Google) and live style switching
- English + Persian (RTL), dark / light themes

**Security**
- Users / groups / roles, JWT + Basic auth, GeoFence-style data rules
  (ALLOW / DENY / LIMIT + CQL)

---

## Architecture

```
                         ┌──────────── Traefik ────────────┐
   browser ── HTTP ──▶   │  routes /giti/* , /api/v1, /tiles │
                         └───┬───────┬───────┬───────┬──────┘
                     gateway │       │       │       │ frontend (Next.js)
                    (Go)     ▼       ▼       ▼       ▼
              wms (Rust)  wfs(Go) tiles(Rust) catalog(Go) auth(Go)
              wps (Go)    convert(Go)
                     └────────── PostGIS · Redis · NATS · MinIO ──────────┘
```

Best-tool-per-job:
- **Rust** — `wms` (tiny-skia raster render, SLD, rustybuzz text shaping),
  `tiles` (MVT + cache).
- **Go** — `gateway`, `catalog`, `auth`, `wfs`, `wps`, `convert`.
- **PostGIS** — data + spatial ops pushed down to the database.
- Shared filter engine (`ogc-kit` Go ↔ `geo-core` Rust) keeps CQL→SQL byte-identical.

---

## Quick start (Docker)

Prebuilt images are published to **GitHub Container Registry** by CI.

```bash
git clone https://github.com/itsmadson/giti.git
cd giti/deploy/compose
cp .env.example .env          # edit credentials
docker compose -f docker-compose.ghcr.yml up -d
```

Open the console at **http://localhost** — default admin `admin` / `geoserver`
(change it in Security). Scale any service: `docker compose up -d --scale wms=4`.

To build from source instead:

```bash
cd deploy/compose
docker compose up -d --build
```

---

## Container images (GHCR)

CI builds and pushes one image per service on every push to `main` and on tags:

```
ghcr.io/itsmadson/giti-gateway     ghcr.io/itsmadson/giti-catalog
ghcr.io/itsmadson/giti-auth        ghcr.io/itsmadson/giti-wms
ghcr.io/itsmadson/giti-wfs         ghcr.io/itsmadson/giti-tiles
ghcr.io/itsmadson/giti-wps         ghcr.io/itsmadson/giti-convert
ghcr.io/itsmadson/giti-frontend
```

Tags: `latest` (main) + the git SHA; semver tags on releases.

---

## Endpoints

| Purpose | URL |
|---|---|
| Admin console | `http://localhost/` |
| WMS | `http://localhost/giti/wms` |
| WFS | `http://localhost/giti/wfs` |
| WMTS | `http://localhost/giti/gwc/service/wmts` |
| XYZ vector tiles | `http://localhost/tiles/{ws:layer}/{z}/{x}/{y}.pbf` |
| WPS | `http://localhost/giti/wps` |
| OGC API - Features | `http://localhost/api/v1/ogc/features` |
| REST (GeoServer-compat) | `http://localhost/giti/rest` |
| Clean JSON API | `http://localhost/api/v1` |

---

## Development

- Go 1.26 workspace (`go.work`), Rust workspace (Cargo), Next.js 15 in `frontend/`.
- Tests: `go test ./...` and `cargo test`.
- Frontend: `cd frontend && npm install && npm run dev`.
- Design docs and sprint plans in `docs/superpowers/`; parity matrix in
  `docs/feature-parity.md`.

---

## License

Apache-2.0 — see [LICENSE](LICENSE).
