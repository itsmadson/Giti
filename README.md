# Giti

High-performance, horizontally scalable OGC geo engine — a drop-in GeoServer
replacement built as microservices (Go + Rust + Next.js).

- WMS 1.1.1/1.3.0 · WFS 1.0/1.1/2.0 · WMTS 1.0/XYZ/TMS · WPS 1.0
- GeoServer-compatible URLs, vendor params (`CQL_FILTER`, `TILED`, …),
  exception formats, and `/rest` config API
- GeoFence-style security, tile caching + seeding, format conversion
- Stores: PostGIS, Shapefile/GeoPackage/GeoJSON, GeoTIFF/COG, GeoParquet (DuckDB)
- Scale any service independently: `docker compose up -d --scale wms=4`

**Design spec:** `docs/superpowers/specs/2026-07-16-giti-engine-design.md`
**Task tracker / resume point:** `task.md`
**Getting started:** `docs/dev/getting-started.md`

License: Apache-2.0
