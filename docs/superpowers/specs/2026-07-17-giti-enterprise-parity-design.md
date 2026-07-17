# Giti Enterprise Parity — Design & Roadmap

Status: approved 2026-07-17. Supersedes the S11–S14 backlog in `task.md` and
`docs/feature-parity.md` (those fold into the E-sprints below).

## Goal

Take Giti from a working OGC MVP to a **drop-in enterprise replacement for
GeoServer** — matching GeoServer's full admin surface (per
<https://docs.geoserver.org/main/en/user/>) and REST API, while staying faster
and better-looking. The user's explicit pain: the backend already serves OGC and
has multiple store connectors, but the **admin UI exposes almost none of it**, and
several enterprise capabilities (full layer editing, SLD editor, tile-cache
config, security management, importer, monitoring, CSW/WCS) have no UI and some
have no backend. This program closes that gap.

## Guiding decisions (locked with user)

1. **UI-first sequencing.** Each sprint first surfaces/serves a complete
   enterprise workflow end-to-end (backend endpoint → typed `src/api` client →
   modern UI page), verified live through Traefik. We do not build backend
   capability with no UI, nor UI with no backend.
2. **Modernized enterprise UX**, not a pixel clone. Same *capabilities and
   workflows* as GeoServer (so a GeoServer admin finds everything), rendered in a
   cleaner modern shell: type-picker wizards, side-panel forms, command palette,
   toasts, RBAC-aware nav, Safavi palette.
3. **Connect-anywhere = the full GeoServer source list.** All existing vector +
   Shapefile/Directory + raster (GeoTIFF/COG/WorldImage/GDAL pack, ECW optional)
   + cascade WMS/WMTS/WFS + MS SQL Server + CSV + KML/KMZ.
4. **Every enterprise sprint gets a detailed implementation plan now**
   (`docs/superpowers/plans/2026-07-17-sprint-E<n>-*.md`).

## Architecture principle

No new services. The microservice skeleton (gateway/catalog/auth/wms/wfs/tiles/
wps/convert/frontend over PostGIS/Redis/NATS/MinIO/Traefik) is sufficient. The
program is three kinds of work:

- **UI layer** — a real GeoServer-grade admin in the existing Next.js 15 app.
- **Catalog API v1 expansion** — the clean JSON API the UI consumes: store CRUD +
  connection-test + introspect, layer/featuretype PATCH, style validate/generate/
  legend, gridset/blobstore/quota config, security management, audit/status.
  (The GeoServer-compat `/rest` API stays for third parties; API v1 is the UI's
  ergonomic surface.)
- **Net-new backend** — connectors (MS SQL, CSV, KML/KMZ, cascade WMS/WMTS),
  engines (SLD validate, JDBC user service, importer jobs, CSW, WCS, bbox
  reproject), and config stores (gridsets, blobstores, disk quota, audit log).

Cross-cutting conventions (unchanged): `/healthz`+`/readyz`, `GITI_*` env,
`X-Giti-*` headers, Traefik explicit router priorities, `host=self` store,
CQL→parameterized-SQL parity (`ogc-kit` Go ↔ `geo-core` Rust golden corpus),
GOWORK=off + `GOPROXY=https://goproxy.cn` in Go Dockerfiles.

## GeoServer surface → sprint mapping

Grounded in the GeoServer user-manual TOC. ✅ already in Giti · 🔵 this program.

| GeoServer area | Giti sprint |
|---|---|
| Web admin: Welcome/About/Status | E7 |
| Data: Workspaces/Stores/Layers/Layer Groups/Browse | E1 (stores) · E2 (layers) · E4 (groups) |
| Vector stores (Shapefile/Directory/GeoPackage/Properties) | E1 UI (✅ backend) · Properties skipped |
| Raster stores (GeoTIFF/WorldImage/ImageMosaic/ArcGrid/GDAL/Pyramid/CoverageView) | E8 |
| Database stores (PostGIS/SQL Server/Oracle/MySQL/Db2, JNDI, SQL Views, FID gen, session scripts) | E1 UI + E2 (SQL Views/FID) · SQL Server/CSV E8 · Oracle/MySQL/Db2 v2 |
| Cascaded data (WFS/WMS/WMTS/stored queries) | E8 |
| Styling: Styles + SLD/CSS/YSLD/MBStyle, cookbook, i18n, legend | E3 |
| WMS (settings/reference/time/output/vendor/GetLegendGraphic/decorations) | ✅ S6 + E3 legend + E9 settings |
| WFS (settings/output/vendor/axis/schema-map) | ✅ S5 + E9 settings |
| OGC API - Features | E9 |
| WCS 2.0 (+ EO) | E9 |
| WMTS | ✅ S7 + E5 config |
| WPS (ops/security/request-builder/cookbook/clustering) | ✅ S8 + E6 security UI |
| CSW (+ ISO metadata profile, DirectDownload) | E9 |
| Filter/ECQL/functions | ✅ engine + E2/E4 filter builder UI |
| Server config: Status/Contact/Service Metadata/Global/Image Processing/Raster Access/REST config/CRS handling/Virtual Services/i18n/Demos/Tools | E7 (settings/status/tools) · E9 (virtual services/CRS) |
| Data directory structure/migrate | ✅ (Postgres-backed catalog) |
| REST (all resources + config API) | E9 parity audit + OpenAPI |
| Security (settings/auth/passwords/UGR/data/services/file-sandbox/CSP/URL-checks/REST-sec/role-system) | E6 |
| Security tutorials (LDAP/AD/CAS/X.509/OIDC/Digest/HTTP-header) | v2 (OIDC/LDAP first) |
| GeoWebCache (Tile Layers/Defaults/Gridsets/DiskQuota/BlobStores/seed/GWC-REST/S3/multidim) | E5 |
| Extensions: Importer | E7 |
| Extensions: Monitoring/Control-flow | E7 (monitor) · E10 (control-flow throttle) |
| Extensions: KML, Vector Tiles, MapML, DXF/Excel/OGR/GeoPackage output, Printing | E4 (KML/output/print) · ✅ MVT S7 |
| Extensions: GeoFence (+ WPS integration, internal server) | ✅ S4 + E6 (admin/service rules) |
| Extensions: COG/DuckDB/GeoParquet (community) | ✅ Giti differentiators |
| App-schema complex features, Pregeneralized features, NetCDF/GRIB, MongoDB, WPS-download/JDBC | v2 backlog |

## Enterprise sprints (E1–E10)

Each sprint's detailed plan lives in `docs/superpowers/plans/`. Summary + exit
criteria below. Exit criteria are all **verified live through Traefik at
`http://localhost`** unless noted.

### E1 — Admin shell + Stores (connect-anywhere UI)
- Modern enterprise shell: left-nav IA (Data / Styling / Services / Tile Caching /
  Security / Monitor / Settings), breadcrumbs, command palette (⌘K), side-panel
  drawer forms, toast notifications, optimistic tables with search + pagination,
  RBAC-aware nav (hide by role), skeleton loaders.
- "New Data Source" **type picker** grouped Vector / Raster / Cascade, and a
  **per-type connection wizard** (PostGIS remote+self, Shapefile, Directory,
  GeoPackage, GeoJSON, GeoParquet; raster + SQL Server + CSV + KML/KMZ + cascade
  shown as forms, backend enabled in E8).
- **Test connection** before save; Stores list/create/edit/delete; introspect →
  select tables → batch publish.
- Backend: catalog API v1 `POST/PUT/DELETE /stores`, `POST /stores/.../test`,
  `GET /stores/{ws}/{store}/tables` (have), expose every registered connector +
  its param schema via `GET /store-types`.
- **Exit:** create each supported vector store type from the UI, test connection,
  introspect, publish a table; edit + delete a store; command palette navigates.

### E2 — Layer management
- Layers list: type icon, title, name, store, enabled, native SRS; search +
  paginate at 300+ scale.
- Full Layer edit (tabbed drawer): **Data** (name/title/abstract/keywords/metadata
  links/enabled/advertised), **Publishing** (default + alternate styles,
  queryable/opaque, WMS path, rendering buffer, interpolation), **CRS** (native +
  declared SRS, SRS handling Force/Reproject/Keep, **bbox compute-from-data /
  from-SRS / reproject**, native + lat/lon bounding boxes), **Feature Type
  Details** (property/type/nullable/min-max table), **SQL View** editor for DB
  stores (parameterized query → virtual table), Feature-ID generation policy.
- Backend: `PATCH /featuretypes`, `PATCH /layers`, bbox compute + reproject
  (PostGIS `ST_Extent`/`ST_Transform`), metadata/keyword columns, SQL-view store
  resource, CRS lookup (`GET /srs/{code}` name + bounds).
- **Exit:** edit a layer's title/keywords/styles; switch declared SRS + reproject;
  compute bbox from data; define a SQL view and publish it; feature-type details
  render from live introspection.

### E3 — Styles + SLD editor
- Styles list (search/paginate at 365-scale, workspace column, bulk remove).
- Style Editor: code editor (CodeMirror) with **SLD/CSS/YSLD/MBStyle** syntax
  highlight + format switch; **Validate** button (server-side parse w/ error line
  markers); **Generate default** from geometry type; **Copy from existing**;
  **Upload** file; **Legend** preview (GetLegendGraphic); **live layer preview**
  applying the style; **Layer Attributes** tab.
- Backend: catalog `POST /styles/validate` (SLD/CSS/YSLD/MBStyle → parse via
  wms/ogc-kit), `POST /styles/generate` (geom → default SLD), style CRUD (have),
  wms `GetLegendGraphic` (have) surfaced.
- **Exit:** create/edit an SLD with validation errors surfaced inline; generate a
  default point/line/polygon style; assign as layer default and see it in preview
  + legend; author a CSS style and validate.

### E4 — Layer Groups + Layer Preview + output formats
- Layer Group CRUD: **Data** (name/title/abstract/workspace/bounds compute + CRS/
  mode single|opaque|named|EO), **layers+styles ordering** (drag reorder),
  **Publishing**, **Tile Caching** tab.
- Enterprise **Layer Preview** page: per-layer OpenLayers/MapLibre embed + format
  links (GeoJSON/GML/KML/KMZ/CSV/shapefile-zip/GeoTIFF), "All formats" dropdown.
- Output formats: KML/KMZ, DXF, Excel, GeoPackage, and **Printing/PDF** (map
  compose → PDF) surfaced.
- Backend: layergroup endpoints (have) + bounds compute; wfs output formats
  (KML/KMZ/CSV/shape-zip); print/PDF endpoint.
- **Exit:** build a multi-layer group with ordered styles + computed bounds,
  preview it; download a layer as GeoJSON/KML/shape-zip; print a map to PDF.

### E5 — Tile caching (GeoWebCache parity)
- Tile Layers list (cached layers + status), Caching Defaults, **Gridsets** CRUD
  (EPSG:3857/4326 + custom), **BlobStores** CRUD (file/MinIO-S3), **Disk Quota**
  (policy + usage), per-layer **Tile Caching** tab (create-cached, metatiling,
  gutter, image formats, expire server/client, gridset subset, **parameter
  filters**), **seed / truncate / reseed** jobs UI with progress, GWC REST parity.
- Backend: gridset registry CRUD, blobstore config store, disk-quota LRU policy
  (have partial), param-filter model, seed/truncate job API + progress (NATS).
- **Exit:** define a custom gridset + S3 blobstore; enable caching on a layer with
  metatiling + param filters; run a seed job and watch progress; hit a cached
  tile; enforce disk quota eviction.

### E6 — Security (users, roles, rules)
- Users/Groups/Roles UI + **user-group services** and **role services** (default
  XML + **JDBC** backed), Authentication chain view, **Password** policies +
  encryption, **Data rules** (GeoFence ALLOW/DENY/LIMIT), **Admin rules**
  (workspace-admin scoping), **Service rules** (per-OWS enable), **WPS security**,
  file-browsing sandbox, **URL checks**, CSP settings, REST security.
- Backend: JDBC user/group + role service, admin-rule + service-rule engine
  (S4.1), password-policy model, URL-check list, CSP config.
- **Exit:** create a user + group + role via UI; back a user/group service with
  JDBC; author a data rule limiting a layer by CQL and see it enforced on WFS/WMS;
  scope a workspace admin; disable a service via service rule.

### E7 — Importer + Server status + Monitoring + Settings
- **Import Data** wizard: source (upload file/zip, server directory, database) →
  auto-detect + preview → per-item target (workspace/store/SRS/style) → **batch
  publish** with progress; re-runnable import contexts.
- **Server Status** / **About** / **GeoServer(Giti) Logs** viewer / **Process
  Status** (running tasks). **Global Settings** (contact info, service metadata,
  proxy base URL, logging), **Image Processing** / **Raster Access** tuning.
- **Monitor**: per-request **audit log** (Activity) + **Reports** (aggregate
  stats, slow requests, by-layer/by-service).
- Backend: importer job engine over convert service, request-audit middleware in
  gateway → store + query API, status/logs/settings endpoints.
- **Exit:** import a shapefile zip and a directory of files to published layers via
  the wizard; view live server status + logs; see requests appear in the audit log
  and a report aggregate; edit global contact/settings and see them in
  GetCapabilities.

### E8 — New connectors (connect-everywhere backend)
- **MS SQL Server** (go-mssqldb + geometry parse), **CSV** (delimited → points),
  **KML/KMZ** (vector read), **cascade WMS + WMTS + WFS** proxy stores, **raster
  driver pack** (GeoTIFF/COG confirmed render, WorldImage, ImageMosaic,
  ImagePyramid, ArcGrid + GDAL formats), **ECW/JP2ECW** optional proprietary
  build flag. Wire each into the E1 type picker (forms already present).
- Backend: connectors implement the `connect.Connector` interface (Validate +
  Introspect); wms/tiles gain cascade proxy paths; raster render path (async-tiff
  + GDAL fallback) confirmed.
- **Exit:** publish a layer from SQL Server, a CSV, a KMZ, a GeoTIFF, and a
  cascaded remote WMS — each serving WMS/WFS(where applicable)/preview live.

### E9 — CSW + WCS + OGC API + REST parity
- **CSW 2.0.2** (GetCapabilities/GetRecords/GetRecordById, ISO metadata profile).
- **WCS 2.0** (GetCapabilities/DescribeCoverage/GetCoverage over raster stores).
- **OGC API - Features** (collections/items JSON, HTML landing).
- **REST parity audit**: every GeoServer `/rest` resource present with JSON+XML
  (workspaces/stores/layers/styles/layergroups/security/fonts/GWC/settings…),
  **Virtual Services** (per-workspace OWS endpoints), **CRS handling** (custom CRS,
  coordinate operations, EPSG editing), published **OpenAPI** doc + docs page.
- **Exit:** CSW GetRecords returns catalog records; WCS GetCoverage returns a
  GeoTIFF subset; OGC API-Features `/collections/{id}/items` returns GeoJSON; a
  third-party REST client round-trips a full publish via documented OpenAPI.

### E10 — Compat, benchmarks, ops, release
- Absorbs old S10: **GeoServer golden-diff harness** (dockerized GeoServer vs Giti,
  canonical XML diff + SSIM image diff on Capabilities/GetMap/GetFeature),
  **k6 load benchmarks** vs GeoServer + MapServer (published in docs),
  **control-flow throttling**, UI **RBAC pass**, **i18n completeness** (fa/en),
  Swarm/K8s stack, README/LICENSE polish, **open-source release**.
- **Exit:** golden-diff harness green on the compat corpus; benchmark report
  published showing Giti faster; clean release artifacts + docs.

## Explicitly skipped (v2 / not-parity)

App-schema complex features, Pregeneralized features, Oracle/MySQL/Db2/MongoDB
stores, NetCDF/GRIB, WPS download/JDBC extensions, J2EE/CAS/LDAP/AD/X.509 auth
tutorials (OIDC + LDAP prioritized for v2), GeoServer-specific Java internals
(control-flow beyond throttle, printing beyond basic PDF). Tracked but out of the
E1–E10 parity scope.

## Sequencing & dependencies

E1 (shell) is the foundation every later sprint's UI builds on → first. E2 depends
on E1 stores. E3 (styles) and E2 (layers) feed E4 (groups/preview). E5, E6, E7 are
largely independent admin domains (parallelizable). E8 backend connectors slot
into E1's type picker anytime after E1. E9 (extra OGC + REST audit) and E10
(compat/release) come last. Recommended order: E1→E2→E3→E4→E5→E6→E7→E8→E9→E10.

## Success definition

A GeoServer administrator sits down at Giti and finds every workflow they know —
add any data source, publish and fully edit layers, author styles, group layers,
configure tile caching, manage users/roles/rules, import data, monitor requests,
consume the REST/OGC APIs — in a faster, better-looking system, with a
golden-diff harness proving OGC compatibility and benchmarks proving speed.
