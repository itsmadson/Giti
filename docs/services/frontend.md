# frontend

Geoson admin console. Next.js 15 (App Router) + React 19 + TypeScript + Tailwind v4.

## Stack
next-themes (dark/light), framer-motion, lucide-react, MapLibre GL. Fonts loaded
at runtime via Google Fonts `<link>` (no build-time network): Space Grotesk
(display), Inter (body), JetBrains Mono (data), Vazirmatn (Persian).

## Design
Signature motif: a hairline **graticule** (lat/lon grid) behind content + a teal
"**prime meridian**" bar (framer-motion `layoutId`) marking the active nav item.
Palette: ink-navy base, teal primary (map depth), amber (tile heat/warnings).

## i18n
`src/app/[locale]/` — `en` (LTR) and `fa` (RTL). `[locale]` drives `dir` and the
dictionary (`src/i18n/`). Header toggles locale + theme.

## Structure (spec §3.9)
- `app/[locale]/(app)/` — authenticated shell (AuthGuard + Sidebar + Header).
  - `/map` — MapLibre workspace (basemap + toggleable Geoson MVT layers).
  - `/dashboard/*` — one route per section, one component each
    (`components/dashboard/pages/`). Overview/Workspaces/Layers wired to the
    backend; stores/styles/tile-cache/security/wps/conversions/settings follow
    the same component-per-route pattern (placeholders reading their service API).
- `app/[locale]/login` — public.
- `src/api/` — ALL backend calls, feature-first: `client.ts` (fetch wrapper +
  bearer token), `auth/{api,types,store}`, `dashboard/<feature>/{api,types}`.

## Backend wiring
Same origin as the APIs (behind Traefik). Login `POST /api/v1/auth/login` → JWT
in localStorage; catalog `GET /api/v1/workspaces` / `/api/v1/layers`; workspace
create `POST /geoserver/rest/workspaces`; map tiles `/tiles/{layer}/{z}/{x}/{y}.pbf`.

## Ops
Traefik catch-all router **priority 1** (must be an explicit low number — `0`
means "unset" and Traefik then computes a length-based priority that can beat the
API routers). API routers: gateway=1, tiles=5, catalog=10, convert=15, auth=20.
Healthcheck uses `127.0.0.1` (server binds IPv4; `localhost`→::1 would refuse).

## Env
NEXT_PUBLIC_API_BASE (default "" = same origin)
