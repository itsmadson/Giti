# auth

Authentication + GeoFence-style authorization. Go + Postgres + Redis.

## Endpoints
- `POST /api/v1/auth/login` → JWT (8h)
- `GET /check` (internal; gateway calls per OWS request)
- `/rest/security/usergroup/*`, `/rest/security/roles*` — GeoServer compat
- `/rest/geofence/rules` — GeoFence-style rules (priority order, first ALLOW/DENY wins,
  LIMIT rules accumulate CQL/attribute constraints; wildcards `*`)

## Enforcement flow
Gateway forwards Authorization + service/request/workspace/layer context to `/check`.
Deny: anonymous → 401 + `WWW-Authenticate: Basic`; authenticated → OWS exception.
Allow: downstream services receive `X-Giti-User`, `X-Giti-Roles`,
`X-Giti-CQL-Read/Write` (data-level limits applied by wfs/wms).

## Defaults
- Seeded admin: `admin` / `geoserver` (GeoServer default — change immediately)
- No matching rule → `GITI_AUTH_DEFAULT` (ALLOW; set DENY to lock down)
- Decisions cached in Redis 60s; security mutations bump `authz:gen` (instant invalidation)

## Env
GITI_HTTP_ADDR, GITI_DATABASE_URL, GITI_REDIS_URL, GITI_JWT_SECRET,
GITI_AUTH_DEFAULT

## Ops note
All Traefik routers use explicit priorities (gateway=1, catalog=10, auth=20) —
Traefik's default rule-length priority breaks the /rest split otherwise.
