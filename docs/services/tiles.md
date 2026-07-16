# tiles

Tile service: WMTS / XYZ / TMS with MVT vector tiles + raster-via-WMS, cached.
Rust.

## Endpoints
- `GET /wmts` — WMTS KVP: `REQUEST=GetCapabilities` | `GetTile`
  (LAYER, TILEMATRIXSET→gridset, TILEMATRIX=z, TILEROW=y, TILECOL=x, FORMAT)
- `GET /wmts/{layer}/{tms}/{z}/{y}/{x}.{ext}` — WMTS RESTful
- `GET /tiles/{layer}/{z}/{x}/{y}.{ext}` — XYZ (top-left origin)
- `GET /gwc/service/tms/1.0.0/{layer}/{z}/{x}/{y}.{ext}` — TMS (y flipped)
- `POST /api/v1/tiles/seed` — pre-render `{layer,gridset,zoomStart,zoomStop,format}` (z ≤ 5 in v1)
- `POST /api/v1/tiles/truncate` — invalidate all tiles for `{layer}`

## Gridsets
EPSG:3857 (Web Mercator, 256px) and EPSG:4326 (plate carrée, 2 cols at z0).
Pure tile→bbox math in `grid.rs`.

## Vector tiles
`ext` pbf/mvt → MVT straight from PostGIS `ST_AsMVT` / `ST_AsMVTGeom` with the
tile envelope pushed down (spatial index). Empty tile → HTTP 204. Content-type
`application/vnd.mapbox-vector-tile`.

## Raster tiles
`ext` png/jpg → proxy `wms:8080/wms` GetMap for the tile bbox (axis-ordered per
CRS), cached.

## Cache
Content-addressed blobs under `GEOSON_TILE_CACHE_DIR` (shared `tilecache`
volume), keyed by SHA-256 of `layer/gridset/z/x/y/fmt/generation`. Redis holds
an existence index (TTL) + a per-layer generation counter. Bumping the
generation changes every tile key → instant invalidation. Works filesystem-only
when Redis is absent.

## Invalidation
NATS subscriber on `catalog.layer.*` / `catalog.featuretype.*` bumps the tile
generation for the changed layer — edit a layer in catalog, its tiles drop.

## Env
GEOSON_HTTP_ADDR, GEOSON_DATABASE_URL, GEOSON_REDIS_URL, GEOSON_NATS_URL,
GEOSON_WMS_URL, GEOSON_TILE_CACHE_DIR
