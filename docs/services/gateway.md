# gateway

OWS front door. Go.

## URL forms
`/giti/ows` · `/giti/{wms|wfs|wps}` · `/giti/{ws}/{svc}` ·
`/giti/{ws}/{layer}/{svc}` · `/giti/gwc/service/wmts`

## Behavior
- KVP (case-insensitive) + POST XML parsing, OGC version negotiation
- GeoServer-exact exception formats (WMS 1.1.1 DTD report, WMS 1.3.0,
  OGC 1.2.0 for WFS 1.0, ows 1.0/1.1 reports, JSON via EXCEPTIONS=application/json)
- Proxies to backends with X-Giti-Workspace / X-Giti-Layer / X-Giti-Version headers
- `/metrics` Prometheus (`giti_gateway_requests_total`, `giti_gateway_request_seconds`)
- Per-IP token-bucket rate limit (plain HTTP 429; not an OWS exception so LBs see throttling)

## Env
GITI_HTTP_ADDR, GITI_WMS_URL, GITI_WFS_URL, GITI_TILES_URL, GITI_WPS_URL,
GITI_RATE_LIMIT (req/s per IP, 0=off), GITI_RATE_BURST
