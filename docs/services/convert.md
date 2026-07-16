# convert

File ingest + format conversion. Go.

## Endpoints
- `POST /api/v1/convert/import?workspace=demo` — multipart form field `file`.
  Copies the upload into the shared data volume, then registers a store +
  featuretype via catalog `/rest` (auto-publishing a layer). Streams progress as
  Server-Sent Events; final event `{"done":true,"layer":"...","workspace":"..."}`.
- `POST /api/v1/convert/cog` — GeoTIFF→COG. Stub in v1
  (`{"status":"pending"}`); real conversion lands in the S12 raster driver pack.

## Supported uploads
Shapefile (.shp), GeoPackage (.gpkg), GeoJSON (.geojson/.json), CSV (.csv),
GeoTIFF (.tif/.tiff). Store type detected from extension.

## Shared volume
Uploads land in `GITI_DATA_DIR` (volume `gitidata`). catalog mounts it
read-only to validate file stores; wfs/wms mount it read-only to read published
file layers. The Dockerfile pre-chowns the mount so the non-root service can write.

## Env
GITI_HTTP_ADDR, GITI_CATALOG_URL, GITI_DATA_DIR
