# gateway

OWS front door. Go.

## URL forms
`/geoserver/ows` · `/geoserver/{wms|wfs|wps}` · `/geoserver/{ws}/{svc}` ·
`/geoserver/{ws}/{layer}/{svc}` · `/geoserver/gwc/service/wmts`

## Behavior
- KVP (case-insensitive) + POST XML parsing, OGC version negotiation
- GeoServer-exact exception formats (WMS 1.1.1 DTD report, WMS 1.3.0,
  OGC 1.2.0 for WFS 1.0, ows 1.0/1.1 reports, JSON via EXCEPTIONS=application/json)
- Proxies to backends with X-Geoson-Workspace / X-Geoson-Layer / X-Geoson-Version headers
- `/metrics` Prometheus (`geoson_gateway_requests_total`, `geoson_gateway_request_seconds`)
- Per-IP token-bucket rate limit (plain HTTP 429; not an OWS exception so LBs see throttling)

## Env
GEOSON_HTTP_ADDR, GEOSON_WMS_URL, GEOSON_WFS_URL, GEOSON_TILES_URL, GEOSON_WPS_URL,
GEOSON_RATE_LIMIT (req/s per IP, 0=off), GEOSON_RATE_BURST
