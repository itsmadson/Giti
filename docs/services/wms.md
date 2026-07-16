# wms

Web Map Service. Rust render engine (tiny-skia + image).

## Operations
- GetCapabilities (1.1.1 / 1.3.0)
- GetMap: LAYERS, STYLES, BBOX, WIDTH, HEIGHT, CRS/SRS, FORMAT, TRANSPARENT,
  BGCOLOR, CQL_FILTER
- GetFeatureInfo: I/J (or X/Y) pixel → world point query; INFO_FORMAT
  text/plain or application/json (GeoJSON)
- GetLegendGraphic: 20×20 swatch from the style

## Rendering
- Vector: PostGIS features via sqlx (`ST_AsBinary(ST_Transform(geom,4326))`,
  spatial-index bbox pushdown), decoded with geozero, drawn with tiny-skia
  (fill+stroke polygons, stroke lines, circle points).
- Output: PNG (default), JPEG, WebP via the `image` crate.
- Axis order: WMS 1.3.0 + geographic CRS (EPSG:4326/4269) → BBOX is lat/lon,
  swapped internally to lon/lat; 1.1.1 → lon/lat.

## Styling (SLD 1.0/1.1)
Polygon/Line/Point/Text/Raster symbolizers; Css/SvgParameter fill/stroke/
stroke-width/opacity → RGBA. Style resolved from STYLES param, else the layer's
default style (workspace style shadows global), else a geometry-type fallback.

## Filter parity
CQL_FILTER compiles through `geo_core::filter` — **byte-identical** to the Go
WFS side, proven by the shared golden corpus `tests/filter-corpus/corpus.json`.
`X-Giti-CQL-Read` (gateway auth) is ANDed into GetMap/GetFeatureInfo filters.

## Not yet implemented (tracked)
- Raster (GeoTIFF/COG) rendering — folds into S7 metatile render + S12 raster
  driver pack (async-tiff fast path preferred over GDAL per spec §10 R&D).
- cosmic-text label placement with rstar collision — follow-up.

## Env
GITI_HTTP_ADDR, GITI_DATABASE_URL
