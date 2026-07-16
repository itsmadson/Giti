# wfs

Web Feature Service. Go + PostGIS pushdown.

## Operations
- GetCapabilities (1.0.0 / 1.1.0 / 2.0.0)
- DescribeFeatureType (XSD from table schema)
- GetFeature / GetPropertyValue: `typeNames`/`typeName`, `CQL_FILTER`, `FILTER`
  (OGC Filter XML), `BBOX`, `propertyName`, `sortBy`, `startIndex`,
  `count`/`maxFeatures`, `resultType=hits`, `featureID` / GetFeatureById
- Transaction (WFS-T): Insert / Update / Delete

## Output formats
`application/json` (GeoJSON) · GML2 (1.0 default) · GML3.2 (2.0 default) · `csv`

## Filters
CQL_FILTER and OGC Filter XML both compile through `libs/ogc-kit/filter` to
**parameterized** PostGIS SQL (bind params only — injection-safe). Spatial ops:
BBOX, INTERSECTS/WITHIN/CONTAINS/DISJOINT/…, DWITHIN. Golden corpus at
`tests/filter-corpus/corpus.json` (Rust WMS reuses it in Sprint 6).

## Auth integration
Honors gateway headers `X-Geoson-Workspace/Layer/Version` and enforces
`X-Geoson-CQL-Read` (GetFeature) / `X-Geoson-CQL-Write` (Update/Delete) by
ANDing them into every query.

## Data access
Shares the catalog Postgres. Resolves layers from `resources`/`stores`/`layers`
tables. Store connection `host=self` means "this same database" (also honored by
catalog's PostGIS connector for self-referential stores). Non-self PostGIS stores
are dialed per store, pooled and cached.

## Env
GEOSON_HTTP_ADDR, GEOSON_DATABASE_URL
