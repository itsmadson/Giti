# OGC compliance status

Audited against the OGC service specs (WMS 1.1.1/1.3.0, WFS 1.0/1.1/2.0,
WMTS 1.0, WPS 1.0) and the GeoServer parameter references. ✅ implemented ·
🟡 partial · ⬜ not yet. This is an ongoing audit — mandatory operations and the
common parameter surface are covered; niche/optional bits are tracked below.

## WMS (`/giti/wms`) — 1.1.1 & 1.3.0

| Operation | Status |
|---|---|
| GetCapabilities | ✅ (advertises EPSG:4326/3857/900913 per layer) |
| GetMap | ✅ |
| GetFeatureInfo | ✅ |
| GetLegendGraphic | ✅ |
| DescribeLayer | ✅ |

**GetMap parameters:** SERVICE, VERSION, REQUEST, LAYERS, STYLES, `SRS`(1.1.1)/`CRS`(1.3.0),
BBOX (+1.3.0 geographic axis swap), WIDTH, HEIGHT, FORMAT (png/jpeg/webp),
TRANSPARENT, BGCOLOR, CQL_FILTER, **SLD_BODY** ✅ · EXCEPTIONS ✅ (XML default + INIMAGE/BLANK + application/json) ·
SLD(URL) ✅ · TIME/ELEVATION ✅ (per-layer dimension column, instant + range) · FILTER(XML) ✅.
On-the-fly **reprojection** for any advertised CRS ✅.

**GetFeatureInfo:** QUERY_LAYERS, I/J (1.3.0) & X/Y (1.1.1), INFO_FORMAT
(json/plain), FEATURE_COUNT, **BUFFER** ✅ · CQL_FILTER ✅ · PROPERTYNAME ⬜.

## WFS (`/giti/wfs`) — 1.0/1.1/2.0

| Operation | Status |
|---|---|
| GetCapabilities | ✅ |
| DescribeFeatureType | ✅ |
| GetFeature | ✅ |
| GetPropertyValue (2.0) | ✅ |
| Transaction (WFS-T) | ✅ insert/update/delete |
| LockFeature / GetFeatureWithLock | ✅ (advisory locks) |
| Stored queries (List/Describe + GetFeatureById) | ✅ |

**GetFeature parameters:** SERVICE, VERSION, REQUEST, TYPENAME(S), FEATUREID,
COUNT/MAXFEATURES, **SRSNAME** ✅ (output reprojection), BBOX(+CRS), FILTER(XML),
CQL_FILTER, OUTPUTFORMAT (GeoJSON, GML2/3/3.2), RESULTTYPE=hits, STARTINDEX,
SORTBY, PROPERTYNAME. NAMESPACES ⬜.

## WMTS / XYZ / TMS (`/giti/gwc/service/wmts`, `/tiles/...`)

| Operation | Status |
|---|---|
| GetCapabilities | ✅ |
| GetTile | ✅ (LAYER, STYLE, TILEMATRIXSET, TILEMATRIX, TILEROW, TILECOL, FORMAT) |
| GetFeatureInfo | ✅ (proxies WMS GFI at tile pixel) |

REST + KVP encodings; XYZ (`/tiles/{layer}/{z}/{x}/{y}.pbf`) and TMS also served.

## WPS (`/giti/wps`) — 1.0

| Operation | Status |
|---|---|
| GetCapabilities | ✅ |
| DescribeProcess | ✅ |
| Execute (KVP + XML POST, sync + async) | ✅ |

## OGC API - Features (`/api/v1/ogc/features`)

Landing, conformance, collections, items (GeoJSON, bbox + limit) ✅.
CQL2/CQL `filter`, `offset` paging, numberMatched/Returned ✅.

## Also served

CSW ✅ (GetCapabilities/GetRecords/GetRecordById/DescribeRecord, Dublin Core) · WCS ⬜ (tracked in the enterprise roadmap; both need the raster/metadata
paths). GeoServer-compat `/rest` config API ✅.

## Priority backlog (next OGC gaps)

1. WMS `TIME`/`ELEVATION` dimensions + `FILTER` (XML) + `SLD` (URL) + full `EXCEPTIONS` formats.
2. WFS `LockFeature`/stored queries; `GetPropertyValue` completion; output `srsName` in GML `@srsName`.
3. WMTS `GetFeatureInfo`.
4. WCS 2.0 + CSW 2.0.2.
