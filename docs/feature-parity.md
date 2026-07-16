# Giti ↔ GeoServer Feature Parity

Goal: match GeoServer's full admin surface, faster. This tracks every GeoServer
feature against Giti status. ✅ done · 🔵 planned (sprint) · ➕ backlog (added
for full parity) · ⏭️ intentionally skipped (obsolete/Java-only).

## Data sources — Vector

| GeoServer source | Giti | Notes |
|---|---|---|
| PostGIS (+ JNDI) | ✅ S2 | JNDI = Java pooling; Giti uses pgxpool per store |
| Shapefile | ✅ S2 | |
| Directory of shapefiles | ✅ S2 | store type "Directory" over folder |
| GeoPackage | ✅ S2 | |
| GeoJSON | ✅ S2 | Giti extra (GeoServer needs plugin) |
| GeoParquet (DuckDB) | ✅ S2 | Giti differentiator |
| CSV | ➕ S11 | delimited text → points; GDAL/OGR CSV driver |
| Microsoft SQL Server (+ JNDI, jTDS) | ➕ S11 | new connector (go-mssqldb + geometry parse) |
| Web Feature Server (NG) cascade | ➕ S11 | proxy remote WFS as a store |
| Properties | ⏭️ | GeoTools Java test format; no real-world use |

## Data sources — Raster (nearly all are GDAL drivers)

Our raster path already uses GDAL (spec §10). These become **driver enablement +
catalog store type**, not new engines. GDAL driver name in parens.

| GeoServer source | Giti | GDAL driver |
|---|---|---|
| GeoTIFF / COG | ✅ S2 (validate), 🔵 S6 (render) | GTiff (+ async-tiff fast path) |
| ImageMosaic | 🔵 S6 / ➕ S12 | GDAL VRT/mosaic index |
| ImagePyramid | ➕ S12 | overview pyramids |
| ArcGrid | ➕ S12 | AAIGrid |
| AIG (Arc/Info Binary Grid) | ➕ S12 | AIG |
| DTED | ➕ S12 | DTED |
| EHdr | ➕ S12 | EHdr |
| ENVIHdr | ➕ S12 | ENVI |
| ERDASImg | ➕ S12 | HFA |
| NITF | ➕ S12 | NITF |
| RST (Idrisi) | ➕ S12 | RST |
| RPFTOC | ➕ S12 | RPFTOC |
| SRP (ASRP/USRP) | ➕ S12 | SRP |
| VRT | ➕ S12 | VRT |
| WorldImage | ➕ S12 | PNG/JPEG + world file |
| GeoPackage (mosaic) | ➕ S12 | GPKG raster |
| ECW / JP2ECW | ➕ S12 (opt) | ECW — needs proprietary ERDAS SDK; optional build |

Enabling these is one registry entry + GDAL `Open` per format; GDAL reads geo
metadata. S12 = "raster driver pack".

## Data sources — Cascading

| GeoServer | Giti | Notes |
|---|---|---|
| WMS (cascade remote) | ➕ S13 | remote-WMS store; wms proxies GetMap |
| WMTS (cascade remote) | ➕ S13 | remote-WMTS store; tiles proxies |

## Tile Caching (GeoWebCache)

| GeoServer | Giti | |
|---|---|---|
| Tile Layers | 🔵 S7 | |
| Caching Defaults | 🔵 S7 | |
| Gridsets | 🔵 S7 | EPSG:3857/4326 + custom registry |
| Disk Quota | ➕ S7 | LRU eviction on cache size cap |
| BlobStores | 🔵 S7 | volume + MinIO backends |

## Security

| GeoServer | Giti | |
|---|---|---|
| Users, Groups, Roles | ✅ S4 | |
| Passwords | ✅ S4 | argon2id |
| Authentication | ✅ S4 | JWT + Basic; OIDC/LDAP = v2 |
| Data rules (GeoFence Data Rules) | ✅ S4 | rule engine ALLOW/DENY/LIMIT |
| Admin rules (GeoFence Admin Rules) | ➕ S4.1 | workspace-admin scoping |
| Services (per-service enable) | ➕ S4.1 | service-level rule shortcut |
| WPS security | 🔵 S8 | |
| Settings (contact/global) | 🔵 S9 | frontend settings |

## Monitor

| GeoServer | Giti | |
|---|---|---|
| Activity | ✅ S3 (metrics) + ➕ S14 | Prometheus now; per-request audit log = S14 |
| Reports | ➕ S14 | request stats endpoint |

## Data (workflows)

| GeoServer | Giti | |
|---|---|---|
| Workspaces / Stores / Layers / Layer Groups / Styles | ✅ S2 | |
| Layer Preview | 🔵 S9 | frontend MapLibre |
| Import Data | 🔵 S8 | convert service ingest |

## OGC services (protocols)

| | Giti | |
|---|---|---|
| WFS 1.0/1.1/2.0 + WFS-T | ✅ S5 | |
| WMS 1.1.1/1.3.0 | 🔵 S6 | |
| WMTS/XYZ/TMS | 🔵 S7 | |
| WPS 1.0 | 🔵 S8 | |

## Sprint map (updated)

S1–S5 done. S6 WMS · S7 Tiles · S8 WPS+convert · S9 Frontend · S10 compat+bench.
Backlog for full GeoServer parity (after core 10):
- **S11 — Extra vector stores**: CSV, MS SQL Server, cascading WFS.
- **S12 — Raster driver pack**: GDAL drivers (ArcGrid…VRT, mosaic, pyramid, ECW opt).
- **S13 — Cascading WMS/WMTS**: remote-service proxy stores.
- **S14 — Monitoring**: request audit log + reports (Activity/Reports parity).
- **S4.1 — Admin rules**: GeoFence admin/service rules (fold into S4 follow-up).

None of these are architectural changes — all are connectors/drivers/handlers on
the existing microservice skeleton. Core OGC serving (S6–S10) stays the priority.
