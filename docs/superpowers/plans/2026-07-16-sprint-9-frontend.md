# Sprint 9 — Frontend (Next.js 15 Admin) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A distinctive, working Giti admin console: Next.js 15 App Router with `[locale]` fa/en (RTL/LTR), dark+light themes, auth (login → JWT), a dashboard shell (graticule motif + meridian active-marker), Overview/Workspaces/Layers pages wired to the real backend, and a full-bleed MapLibre map workspace. Runs in Docker behind Traefik.

**Architecture:** `frontend/` = the Next.js app (spec §3.9 folder structure exactly). All backend calls go through `src/api/` (feature-first): `client.ts` fetch wrapper (base URL from env, bearer token, error normalization), `auth/{api,types,store}`, `dashboard/<feature>/{api,types}`. i18n via a lightweight dictionary provider (`src/i18n/`), no next-intl dep — `[locale]` param drives `dir` + dictionary. Theme via next-themes + Tailwind v4 CSS tokens. The design signature (graticule grid + meridian marker) lives in `styles/globals.css` + shell components.

**Tech Stack:** Next.js 15 (App Router), React 19, TypeScript, Tailwind v4, framer-motion, lucide-react, next-themes, maplibre-gl. Fonts: Space Grotesk (display), Inter (body), JetBrains Mono (data), Vazirmatn (Persian).

## Global Constraints

- Exact folder structure from spec §3.9 (routes under `src/app/[locale]/`, ALL backend calls under `src/api/`, one component per dashboard route under `components/dashboard/pages/`).
- Locales: `en` (LTR), `fa` (RTL). `[locale]` drives `<html dir>` and dictionary. English-first, Persian second.
- Dark + light themes via next-themes; tokens in `styles/globals.css` (light/dark + Persian palette per spec).
- Backend base URL from `NEXT_PUBLIC_API_BASE` (default `` = same origin; behind Traefik the frontend and APIs share the host). Auth: `POST /api/v1/auth/login`; catalog: `/api/v1/workspaces`, `/api/v1/layers`; GeoServer REST under `/giti/rest`.
- Design tokens (locked): base `#0B1220`, surface `#111A2B`, border `#1E2A3E`, text `#E6EDF6`, muted `#8595AD`, primary teal `#2DD4BF`, amber `#F59E0B`, ok `#34D399`, err `#F87171`. Light: base `#F5F8FC`, surface `#FFFFFF`, border `#DCE5F0`, text `#0B1220`, muted `#5A6B85`.
- Verification is build-based (frontend, not TDD-per-fn): `npm run build` type-checks + compiles; `npm run lint` passes; live e2e = login + pages render against the running stack.
- Commit after every task, Conventional Commits.

## File Structure (spec §3.9)

```
frontend/
  package.json  next.config.ts  tsconfig.json  postcss.config.mjs  Dockerfile  .dockerignore
  src/
    app/
      layout.tsx                      # root: fonts, providers
      [locale]/
        layout.tsx                    # sets dir + dictionary provider
        page.tsx                      # redirect -> /dashboard
        (app)/
          layout.tsx                  # AuthGuard + header + sidebar shell
          map/page.tsx                # MapLibre workspace
          dashboard/
            page.tsx                  # overview
            workspaces/page.tsx
            layers/page.tsx
            stores/page.tsx           # stub (same pattern)
            styles/page.tsx           # stub
            tile-cache/page.tsx       # stub
            security/page.tsx         # stub
            wps/page.tsx              # stub
            conversions/page.tsx      # stub
            settings/page.tsx         # stub
        login/page.tsx                # public
    api/
      client.ts
      auth/{api.ts,types.ts,store.ts}
      dashboard/
        overview/{api.ts,types.ts}
        workspaces/{api.ts,types.ts}
        layers/{api.ts,types.ts}
    components/
      layout/{Sidebar.tsx,Header.tsx,Shell.tsx}
      auth/{AuthGuard.tsx,LoginForm.tsx}
      map/MapWorkspace.tsx
      dashboard/pages/{Overview.tsx,Workspaces.tsx,Layers.tsx}
      ui/{Card.tsx,StatCard.tsx,DataTable.tsx,Badge.tsx,Button.tsx}
      icons/index.tsx                 # semantic name -> lucide icon
    i18n/{config.ts,provider.tsx,dictionaries/{en.ts,fa.ts}}
    lib/{utils.ts,basemaps.ts}
    styles/globals.css
```

---

### Task 1: Scaffold — Next.js 15, Tailwind v4, deps, fonts, tokens

**Files:**
- Create: `frontend/package.json`, `frontend/next.config.ts`, `frontend/tsconfig.json`, `frontend/postcss.config.mjs`, `frontend/next-env.d.ts`, `frontend/.gitignore`, `frontend/src/app/layout.tsx`, `frontend/src/styles/globals.css`, `frontend/src/app/[locale]/page.tsx`
- Remove: `frontend/.gitkeep`

**Interfaces:**
- Produces: a buildable Next 15 app. Root `layout.tsx` loads the four fonts as CSS variables (`--font-display`, `--font-body`, `--font-mono`, `--font-fa`) and wraps children in `<ThemeProvider>`. `globals.css` defines the Giti design tokens (light/dark) + the graticule utility.

- [x] **Step 1: package.json** (Next 15, React 19, Tailwind v4, deps):

```json
{
  "name": "giti-frontend",
  "private": true,
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start -p 8080",
    "lint": "next lint"
  },
  "dependencies": {
    "next": "15.1.6",
    "react": "19.0.0",
    "react-dom": "19.0.0",
    "next-themes": "^0.4.4",
    "framer-motion": "^11.15.0",
    "lucide-react": "^0.469.0",
    "maplibre-gl": "^4.7.1"
  },
  "devDependencies": {
    "typescript": "^5.7.3",
    "@types/node": "^22.10.5",
    "@types/react": "^19.0.7",
    "@types/react-dom": "^19.0.3",
    "tailwindcss": "^4.0.0",
    "@tailwindcss/postcss": "^4.0.0",
    "postcss": "^8.5.1",
    "eslint": "^9.18.0",
    "eslint-config-next": "15.1.6"
  }
}
```

- [x] **Step 2:** `next.config.ts`, `tsconfig.json` (paths `@/*`→`src/*`), `postcss.config.mjs` (`{ plugins: { "@tailwindcss/postcss": {} } }`), `next-env.d.ts`, `.gitignore` (node_modules, .next).
- [x] **Step 3:** `globals.css` — `@import "tailwindcss";` then `@theme` / `:root` + `.dark` token blocks with the locked palette, a `.graticule` background utility (repeating 1px hairline grid using `--border`), and font-family bindings. RTL: `[dir=rtl]` uses `--font-fa`.
- [x] **Step 4:** root `app/layout.tsx` — `next/font/google` for Inter, Space_Grotesk, JetBrains_Mono, Vazirmatn → CSS vars; `<ThemeProvider attribute="class" defaultTheme="dark">`. `app/[locale]/page.tsx` redirects to `./dashboard`.
- [x] **Step 5:** `npm install` then `npm run build` → succeeds (empty-ish app). **Commit** `git commit -m "feat(frontend): next 15 scaffold, tailwind v4, fonts, design tokens"`

---

### Task 2: i18n — config, provider, en/fa dictionaries, RTL

**Files:**
- Create: `frontend/src/i18n/config.ts`, `frontend/src/i18n/provider.tsx`, `frontend/src/i18n/dictionaries/en.ts`, `frontend/src/i18n/dictionaries/fa.ts`
- Create: `frontend/src/app/[locale]/layout.tsx`

**Interfaces:**
- Produces:
  - `config.ts`: `export const locales = ["en","fa"] as const; export type Locale = typeof locales[number]; export const dir = (l: Locale) => l === "fa" ? "rtl" : "ltr"; export const dictionaries: Record<Locale, Dict>`.
  - `type Dict` = a flat record of UI strings (nav labels, actions, page titles). Both en.ts and fa.ts export the same keys.
  - `provider.tsx`: `"use client"` `I18nProvider` (React context) + `useT()` hook returning `t(key)` and `locale`.
  - `[locale]/layout.tsx`: validates the param, sets `<div dir=...>` wrapper, provides the dictionary.

- [x] **Step 1:** `config.ts` with `Dict` type (keys: `app.name`, `nav.overview/workspaces/stores/layers/styles/tileCache/security/wps/conversions/settings/map`, `action.signIn/signOut/refresh/create/delete`, `login.title/username/password`, `overview.title/health/requests/cacheHit`, `common.loading/empty/error`).
- [x] **Step 2:** `dictionaries/en.ts` + `fa.ts` (Persian translations, same keys).
- [x] **Step 3:** `provider.tsx` context + `useT`.
- [x] **Step 4:** `[locale]/layout.tsx` — `generateStaticParams` for both locales; unknown locale → `notFound()`; wrap in `I18nProvider` with `dir`.
- [x] **Step 5:** `npm run build` → succeeds. **Commit** `git commit -m "feat(frontend): i18n en/fa with rtl and dictionary provider"`

---

### Task 3: API client + auth store

**Files:**
- Create: `frontend/src/api/client.ts`, `frontend/src/api/auth/types.ts`, `frontend/src/api/auth/api.ts`, `frontend/src/api/auth/store.ts`, `frontend/src/lib/utils.ts`

**Interfaces:**
- Produces:
  - `client.ts`: `const BASE = process.env.NEXT_PUBLIC_API_BASE ?? ""`; `apiFetch<T>(path, opts?)` — attaches `Authorization: Bearer <token>` from `authStore.token()`, JSON by default, throws `ApiError{status,message}` on non-2xx. Also `apiText`/`apiRaw` variants for XML REST.
  - `auth/types.ts`: `interface LoginResponse { token: string; expiresIn: number }`, `interface Session { token: string; user: string; roles: string[] }`.
  - `auth/api.ts`: `login(username, password): Promise<LoginResponse>` → POST `/api/v1/auth/login`.
  - `auth/store.ts`: `"use client"` tiny store (module-level state + `useSyncExternalStore` or a Zustand-free custom hook). `useSession()`, `setSession(token)`, `clearSession()`, `token()` (non-hook accessor for client.ts). Persists token in `localStorage` (`giti.token`), decodes `sub`/`roles` from the JWT payload.

- [x] **Step 1:** `lib/utils.ts` — `cn(...classes)` (clsx-free join), `decodeJwt(token)` (base64url payload → `{sub, roles}`).
- [x] **Step 2:** `client.ts` with `ApiError` + `apiFetch`/`apiText`.
- [x] **Step 3:** `auth/{types,api,store}`.
- [x] **Step 4:** `npm run build`. **Commit** `git commit -m "feat(frontend): api client and auth store"`

---

### Task 4: Login page + AuthGuard + app shell (Sidebar/Header)

**Files:**
- Create: `frontend/src/components/auth/LoginForm.tsx`, `frontend/src/app/[locale]/login/page.tsx`, `frontend/src/components/auth/AuthGuard.tsx`, `frontend/src/components/layout/{Sidebar.tsx,Header.tsx,Shell.tsx}`, `frontend/src/components/icons/index.tsx`, `frontend/src/app/[locale]/(app)/layout.tsx`, `frontend/src/components/ui/{Button.tsx,Card.tsx,Badge.tsx}`

**Interfaces:**
- Consumes: `useT`, `authStore` (login/session), `apiFetch`.
- Produces:
  - `LoginForm`: username/password (defaults hint `admin`/`geoserver`), calls `authApi.login`, `setSession`, routes to `/{locale}/dashboard`. Graticule background, meridian accent, Space Grotesk title.
  - `AuthGuard`: client component; if no session → redirect to `/{locale}/login`.
  - `Sidebar`: nav list (icons + labels), the **meridian marker** = a teal vertical bar animated (framer-motion `layoutId`) to the active route. `Header`: workspace title, theme toggle, locale switch (en/fa), sign-out.
  - `Shell`: composes Sidebar + Header + `<main class="graticule">`.
  - `(app)/layout.tsx`: wraps children in `AuthGuard` + `Shell`.
  - `icons/index.tsx`: semantic map (`overview→LayoutDashboard`, `workspaces→FolderTree`, `layers→Layers`, `map→Map`, `security→Shield`, ...).

- [x] **Step 1:** ui primitives (Button, Card, Badge) using tokens.
- [x] **Step 2:** icons map.
- [x] **Step 3:** LoginForm + login page.
- [x] **Step 4:** AuthGuard, Sidebar (with meridian marker), Header, Shell, `(app)/layout.tsx`.
- [x] **Step 5:** `npm run build`. **Commit** `git commit -m "feat(frontend): login, auth guard, dashboard shell with meridian nav"`

---

### Task 5: Overview + Workspaces + Layers pages (wired to backend)

**Files:**
- Create: `frontend/src/api/dashboard/overview/{types.ts,api.ts}`, `frontend/src/api/dashboard/workspaces/{types.ts,api.ts}`, `frontend/src/api/dashboard/layers/{types.ts,api.ts}`
- Create: `frontend/src/components/ui/{StatCard.tsx,DataTable.tsx}`, `frontend/src/components/dashboard/pages/{Overview.tsx,Workspaces.tsx,Layers.tsx}`
- Create: `frontend/src/app/[locale]/(app)/dashboard/{page.tsx,workspaces/page.tsx,layers/page.tsx}`

**Interfaces:**
- `overview/api.ts`: `getHealth()` — pings `/healthz` per service via the gateway (or a single `/healthz`); returns service statuses; `getStats()` reads gateway `/metrics` (parse a couple Prometheus lines) — v1: derive counts from layers/workspaces + health.
- `workspaces/api.ts`: `listWorkspaces(): Promise<Workspace[]>` → GET `/api/v1/workspaces`; `createWorkspace(name)` → POST `/giti/rest/workspaces` (XML).
- `layers/api.ts`: `listLayers(): Promise<Layer[]>` → GET `/api/v1/layers`.
- Pages are server components that render the client `Overview`/`Workspaces`/`Layers` components (which fetch on mount via the api layer).

- [x] **Step 1:** api types + funcs for the three features.
- [x] **Step 2:** `StatCard` (big number Space Grotesk + label + trend), `DataTable` (hairline rows, mono IDs).
- [x] **Step 3:** `Overview` (service-health strip + stat grid), `Workspaces` (table + create), `Layers` (table with type badge, links to map).
- [x] **Step 4:** route pages mounting the components; sidebar links resolve.
- [x] **Step 5:** `npm run build`. **Commit** `git commit -m "feat(frontend): overview, workspaces, layers pages wired to backend"`

---

### Task 6: MapLibre map workspace + remaining dashboard stubs

**Files:**
- Create: `frontend/src/lib/basemaps.ts`, `frontend/src/components/map/MapWorkspace.tsx`, `frontend/src/app/[locale]/(app)/map/page.tsx`
- Create: `frontend/src/app/[locale]/(app)/dashboard/{stores,styles,tile-cache,security,wps,conversions,settings}/page.tsx` + `frontend/src/components/dashboard/pages/Placeholder.tsx`

**Interfaces:**
- `basemaps.ts`: a couple of MapLibre style URLs/specs (a dark and light raster-free demo style using OSM tiles, plus a "Giti layers" source that points at `/tiles/{layer}/{z}/{x}/{y}.pbf`).
- `MapWorkspace`: `"use client"`, dynamic-imports maplibre-gl (no SSR), full-bleed map + a floating layer panel that lists Giti vector layers (from `layersApi`) and toggles them as MVT sources on the map. Theme-aware basemap.
- Remaining dashboard pages render `<Placeholder title=... />` (same component-per-route pattern the spec requires), each linking to its future feature. Honest: they say "Coming from the <service> API".

- [x] **Step 1:** basemaps + MapWorkspace (guard maplibre to client; add a Giti MVT source + layer).
- [x] **Step 2:** map route page.
- [x] **Step 3:** Placeholder component + the 7 stub route pages.
- [x] **Step 4:** `npm run build`. **Commit** `git commit -m "feat(frontend): maplibre workspace and dashboard section routes"`

---

### Task 7: Docker, compose, e2e, docs, close out

**Files:**
- Create: `frontend/Dockerfile`, `frontend/.dockerignore`, `docs/services/frontend.md`
- Modify: `frontend/next.config.ts` (`output: "standalone"`), `deploy/compose/docker-compose.yml`, `.github/workflows/ci.yml`, `docs/architecture.md`, `task.md`

**Interfaces:**
- Dockerfile: multi-stage (node build → standalone runtime), serves on `:8080`. Compose `frontend` service, Traefik route `PathPrefix(/)` **priority 0** (lowest — everything else wins; frontend is the catch-all). Env `NEXT_PUBLIC_API_BASE=""` (same origin).

- [x] **Step 1:** `next.config.ts` standalone; Dockerfile + .dockerignore.
- [x] **Step 2:** compose `frontend` service (build, traefik catch-all priority 0, depends_on gateway). CI: `docker build -f frontend/Dockerfile .` (context repo root or frontend/). Note: frontend build context = `frontend/` (self-contained, no workspace).
- [x] **Step 3:** `docker compose up -d --build frontend` → open `http://localhost/en/login`, sign in `admin`/`geoserver`, land on overview, see workspaces/layers, open the map.

```bash
cd deploy/compose && docker compose up -d --build frontend
curl -s -o /dev/null -w '%{http_code}\n' http://localhost/en/login          # 200
curl -s http://localhost/en/login | grep -o 'Giti' | head -1               # brand present
```

- [x] **Step 4:** `docs/services/frontend.md`; architecture.md frontend row → done; task.md Sprint 9 → [x]; plan boxes → [x].
- [x] **Step 5:** Final commit `git commit -m "feat(frontend): docker, compose, docs; complete sprint 9"`.
