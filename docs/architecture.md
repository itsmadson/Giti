# Geoson Architecture

Full approved design: [design spec](superpowers/specs/2026-07-16-geoson-engine-design.md).

## Services

| Service | Language | Role | Status |
|---|---|---|---|
| gateway | Go | OWS front door: parsing, negotiation, exceptions, routing | done (Sprint 3) — [docs](services/gateway.md) |
| catalog | Go | config system of record, GeoServer `/rest` compat | done (Sprint 2) — [docs](services/catalog.md) |
| auth | Go | users/groups/roles, GeoFence rules | done (Sprint 4) — [docs](services/auth.md) |
| wfs | Go | WFS 1.0/1.1/2.0, filters, WFS-T | done (Sprint 5) — [docs](services/wfs.md) |
| wms | Rust | WMS 1.1.1/1.3.0 rendering (vector) | done (Sprint 6) — [docs](services/wms.md); raster in S7/S12 |
| tiles | Rust | WMTS/XYZ/TMS, MVT, cache + seeding | done (Sprint 7) — [docs](services/tiles.md) |
| wps | Go | WPS 1.0 process engine (PostGIS ops) | done (Sprint 8) — [docs](services/wps.md) |
| convert | Go | ingest + format conversion | done (Sprint 8) — [docs](services/convert.md) |
| frontend | Next.js 15 | admin UI | planned (Sprint 9) |

## Infra

Traefik (LB) · Postgres+PostGIS (catalog + data) · Redis (caches/sessions) ·
NATS JetStream (events + jobs) · MinIO (optional object storage).

## Conventions

- Health: `/healthz` liveness (`200 ok`), `/readyz` readiness (JSON, 200/503) — every service.
- Listen address: `GEOSON_HTTP_ADDR` (default `:8080`).
- Stateless request path; state only in Postgres/Redis/NATS/object storage.
- Docker build context: repo root, `-f services/<name>/Dockerfile`.
- Go tests across all workspace modules: `go test github.com/geoson/geoson/...`
  (directory patterns like `./libs/...` do not cross module boundaries in workspace mode).
