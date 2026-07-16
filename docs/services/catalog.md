# catalog

Configuration system of record. Go + Postgres.

## Endpoints
- `/giti/rest/*`, `/rest/*` — GeoServer-compatible config API (XML + JSON)
  - workspaces, datastores, coveragestores, featuretypes, coverages, layers,
    styles (incl. raw SLD upload/download), layergroups
- `/api/v1/workspaces`, `/api/v1/layers` — clean JSON API for the frontend
- `/healthz`, `/readyz`

## Store types
PostGIS (live validation + introspection), Shapefile, GeoPackage, GeoJSON,
GeoTIFF/COG (magic-byte validation), GeoParquet (DuckDB schema introspection).

## Behavior notes
- Creating a featuretype auto-publishes a VECTOR layer (default style `generic`);
  coverages auto-publish RASTER layers (style `raster`) — matching GeoServer.
- Store creation validates connections for known store types (400 on failure).
- Seeded global styles: generic, point, line, polygon, raster.

## Events
Every mutation publishes `catalog.<entity>.<created|updated|deleted>`
(JSON `{"name","workspace"}`) on NATS. Consumers: tiles (cache invalidation),
wms/wfs (config cache drop).

## Env
`GITI_HTTP_ADDR`, `GITI_DATABASE_URL`, `GITI_NATS_URL`
