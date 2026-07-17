# Sprint E8 — New Connectors (Connect-Everywhere Backend) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** Enable the rest of GeoServer's data-source list: MS SQL Server, CSV, KML/KMZ, cascade WMS/WMTS/WFS proxy stores, and the raster driver pack (GeoTIFF/COG confirmed, WorldImage, ImageMosaic, ImagePyramid, ArcGrid + GDAL formats; ECW optional). Each registers a `Meta()` so it appears in the E1 type picker.

**Architecture:** Each vector connector implements `connect.Connector` (Validate + Introspect) + `Described` (Meta). Cascade stores proxy remote services (wms/tiles gain proxy paths). Raster path uses async-tiff (COG fast path) + GDAL fallback; register coverage-store types. No new services.

**Tech Stack:** Go (catalog connectors: go-mssqldb, encoding/csv, KML parse), Rust (wms/tiles cascade + raster via async-tiff/GDAL), optional GDAL cgo.

## Global Constraints

Same as prior. Each new connector: `register(type, conn)` + `registerMeta(...)` so E1 picker shows it with the right param schema. Cascade fetches honor E6 URL checks. ECW behind a build tag (proprietary SDK) — off by default.

## File Structure

- `services/catalog/internal/connect/{mssql,csv,kml,cascade_wfs}.go`.
- `services/wms/src/cascade.rs` (remote WMS GetMap proxy), `services/tiles/src/cascade.rs` (remote WMTS proxy).
- `services/wms/src/raster/{cog,gdal}.rs` — render + driver registry.
- `services/catalog/internal/connect/raster_meta.go` — coverage-store metas.

## Task 1: MS SQL Server connector (Go)

- [ ] `mssql.go`: implement Connector via `github.com/microsoft/go-mssqldb`; Validate = ping; Introspect = query `sys.geometry_columns`-equivalent (`INFORMATION_SCHEMA` + `geometry`/`geography` columns); parse SRID. `registerMeta` (Vector, params host/port/database/user/passwd/schema).
- [ ] **Test (gated):** against a test SQL Server (docker) — introspect returns a spatial table. If CI lacks SQL Server, gate behind `GITI_TEST_MSSQL_URL`.
- [ ] Ensure WFS/WMS SQL generation dialect works for MSSQL geometry (`.STAsText()`, `.STAsBinary()`), or transcode via a dialect shim.
- [ ] Build; commit `feat(catalog): MS SQL Server connector (E8)`.

## Task 2: CSV + KML/KMZ connectors (Go)

- [ ] `csv.go`: delimited text → point features (configurable lat/lon or WKT column); Introspect = header columns; `registerMeta` (Vector, params url/path, latField, lonField, wktField, delimiter).
- [ ] `kml.go`: parse KML/KMZ (unzip KMZ) → features; Introspect = folders/placemarks schema; `registerMeta` (Vector, param url/path).
- [ ] **Test:** a sample CSV yields point features; a KMZ yields placemark features.
- [ ] Build; commit `feat(catalog): CSV + KML/KMZ connectors (E8)`.

## Task 3: Cascade WFS store (Go)

- [ ] `cascade_wfs.go`: store type "WFS-NG"; Validate = remote GetCapabilities; Introspect = FeatureTypeList → resources. Published layers proxy GetFeature to the remote (WFS resolver detects cascade store → forwards + reprojects). `registerMeta` (Cascade, params url, version, user, passwd).
- [ ] **Test (gated):** against a known WFS URL (or a stub) → introspect lists types.
- [ ] Commit `feat(catalog,wfs): cascade WFS store (E8)`.

## Task 4: Cascade WMS/WMTS proxy (Rust)

- [ ] `wms/src/cascade.rs`: store type "WMS" (Cascade); GetMap of a cascaded layer proxies to remote WMS (bbox/crs passthrough, honor URL checks). `tiles/src/cascade.rs`: store type "WMTS"; tile requests proxy remote WMTS. Register coverage/cascade metas via catalog `raster_meta.go`/cascade meta.
- [ ] **Test (gated):** proxy a remote WMS GetMap returns an image.
- [ ] Build wms+tiles; commit `feat(wms,tiles): cascade WMS/WMTS proxy stores (E8)`.

## Task 5: Raster driver pack (Rust + catalog metas)

- [ ] Confirm GeoTIFF/COG render path (async-tiff fast path; GDAL fallback). Add coverage-store types + metas: GeoTIFF, WorldImage, ImageMosaic, ImagePyramid, ArcGrid, and GDAL-generic (DTED/EHdr/ENVI/HFA/NITF/RST/RPFTOC/SRP/VRT). ECW/JP2ECW behind `--features ecw` build tag.
- [ ] `raster_meta.go`: `registerMeta` each (Raster, kind coveragestore, param url/path).
- [ ] wms raster render: open via driver → warp to request CRS/bbox → encode PNG/JPEG.
- [ ] **Test:** render a GeoTIFF coverage → PNG; a VRT → PNG.
- [ ] Build; live verify publish + WMS GetMap of a GeoTIFF.
- [ ] Commit `feat(wms,catalog): raster driver pack (E8)`.

## Task 6: Wire into E1 picker + acceptance

- [ ] Confirm `GET /api/v1/store-types` now lists MS SQL, CSV, KML/KMZ, WFS/WMS/WMTS cascade, and raster types — the E1 wizard renders their forms automatically (no UI change needed beyond icons).
- [ ] **Acceptance:** publish a layer from SQL Server, a CSV, a KMZ, a GeoTIFF, and a cascaded remote WMS — each serving WMS/WFS(where applicable)/preview live through Traefik.
- [ ] Commit `chore: E8 connect-everywhere connectors complete`.

## Self-Review

Spec E8 coverage: MS SQL(T1), CSV/KML/KMZ(T2), cascade WFS(T3)/WMS/WMTS(T4), raster pack + ECW-opt(T5), picker wiring(T6). Every connector registers `Meta()` so E1 surfaces it. Gated tests where external services required. Dialect shim noted for MSSQL SQL generation.
