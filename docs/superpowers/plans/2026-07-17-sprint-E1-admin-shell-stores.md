# Sprint E1 — Admin Shell + Connect-Anywhere Stores — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a modern enterprise admin shell (grouped nav, command palette, drawer forms, toasts) and a GeoServer-grade "New Data Source" flow: type picker → per-type connection wizard → test connection → create store → introspect → batch publish, with full store CRUD.

**Architecture:** Backend adds a store-type registry (each connector advertises a param schema) and three catalog API v1 endpoints (`GET /store-types`, `POST /stores`, `POST /stores/{ws}/{store}/test`, `PUT`/`DELETE`). Frontend gains a reusable shell (nav config, CommandPalette, Drawer, Toaster) and a Stores feature that drives store CRUD + publish through the typed `src/api` client. Every task ships backend → typed client → UI, verified live through Traefik.

**Tech Stack:** Go 1.26 (catalog, net/http ServeMux, pgx/v5), Next.js 15 App Router + React 19 + TS + Tailwind v4, framer-motion, lucide-react, cmdk (command palette).

## Global Constraints

- Go module `github.com/giti/giti`; Go Dockerfiles build `GOWORK=off`, `GOPROXY=https://goproxy.cn` (build-arg).
- Env prefix `GITI_`; headers `X-Giti-*`; health `/healthz`+`/readyz`.
- Catalog REST mounts API v1 at `/api/v1/` (Traefik routes `/api/v1` + `/giti/rest` → catalog, priority 10).
- Store `host=self` = catalog's own DB via `GITI_DATABASE_URL`.
- Frontend path alias `@/* → ./src/*`; i18n via `useT()` with keys in `src/i18n/dictionaries/{en,fa}.ts` (add every new string to BOTH).
- Colors via CSS vars only (`var(--color-primary)` etc.); no hard-coded hex except map paint.
- Catalog integration tests need compose Postgres: `GITI_TEST_DATABASE_URL=postgres://giti:giti-dev-password@127.0.0.1:5433/giti`.
- Verify live through Traefik at `http://localhost` after each UI task (stack: `cd deploy/compose && docker compose up -d --build catalog frontend`).

---

## File Structure

Backend (catalog):
- `services/catalog/internal/connect/connect.go` — extend registry with `StoreTypeMeta` + `StoreTypes()`.
- `services/catalog/internal/connect/*.go` — each connector adds a `Meta()` (param schema).
- `services/catalog/internal/rest/apiv1_stores.go` — NEW: store-types, create, update, delete, test handlers (split out of apiv1.go).
- `services/catalog/internal/rest/apiv1.go` — register the new routes.

Frontend:
- `frontend/src/config/nav.ts` — NEW: grouped nav model (single source for Sidebar + CommandPalette).
- `frontend/src/components/layout/Sidebar.tsx` — consume grouped nav.
- `frontend/src/components/ui/Drawer.tsx` — NEW: right side-panel form container.
- `frontend/src/components/ui/Toast.tsx` — NEW: toaster provider + `useToast()`.
- `frontend/src/components/layout/CommandPalette.tsx` — NEW: ⌘K nav.
- `frontend/src/app/[locale]/(app)/layout.tsx` — mount Toaster + CommandPalette.
- `frontend/src/api/dashboard/stores/{types,api}.ts` — extend: store-types, create/update/delete/test.
- `frontend/src/components/dashboard/stores/NewStoreWizard.tsx` — NEW: type picker + dynamic form.
- `frontend/src/components/dashboard/pages/Stores.tsx` — list + drawer wizard + edit/delete + publish.

---

## Task 1: Store-type registry with param schema (backend)

**Files:**
- Modify: `services/catalog/internal/connect/connect.go`
- Modify: `services/catalog/internal/connect/postgis.go`, `files.go`, `geoparquet.go`, `cog.go`
- Test: `services/catalog/internal/connect/registry_test.go`

**Interfaces:**
- Produces: `type ParamField struct { Key, Label, Type, Default string; Required bool }`; `type StoreTypeMeta struct { Type, Kind, Category, Label string; Params []ParamField }`; `func StoreTypes() []StoreTypeMeta`; connector optional `Meta() StoreTypeMeta` via `type Described interface { Meta() StoreTypeMeta }`.

- [ ] **Step 1: Write the failing test**

```go
// services/catalog/internal/connect/registry_test.go
package connect

import "testing"

func TestStoreTypesIncludesPostGIS(t *testing.T) {
	types := StoreTypes()
	var pg *StoreTypeMeta
	for i := range types {
		if types[i].Type == "PostGIS" {
			pg = &types[i]
		}
	}
	if pg == nil {
		t.Fatal("PostGIS not in StoreTypes()")
	}
	if pg.Category != "Vector" || pg.Kind != "datastore" {
		t.Fatalf("bad meta: %+v", pg)
	}
	keys := map[string]bool{}
	for _, p := range pg.Params {
		keys[p.Key] = true
	}
	for _, want := range []string{"host", "port", "database", "user", "passwd", "schema"} {
		if !keys[want] {
			t.Errorf("PostGIS meta missing param %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/catalog/internal/connect/ -run TestStoreTypesIncludesPostGIS -v`
Expected: FAIL — `StoreTypes` / `StoreTypeMeta` undefined.

- [ ] **Step 3: Add types + registry accessor**

Append to `connect.go`:

```go
// ParamField describes one connection parameter for a store type.
type ParamField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // text | number | password | select
	Default  string `json:"default,omitempty"`
	Required bool   `json:"required"`
}

// StoreTypeMeta is a store type advertised to the admin UI.
type StoreTypeMeta struct {
	Type     string       `json:"type"`     // e.g. "PostGIS"
	Kind     string       `json:"kind"`     // datastore | coveragestore
	Category string       `json:"category"` // Vector | Raster | Cascade
	Label    string       `json:"label"`
	Params   []ParamField `json:"params"`
}

// Described is implemented by connectors that expose UI metadata.
type Described interface{ Meta() StoreTypeMeta }

var metas []StoreTypeMeta

func registerMeta(m StoreTypeMeta) { metas = append(metas, m) }

// StoreTypes returns all advertised store types (sorted by category then label).
func StoreTypes() []StoreTypeMeta {
	out := make([]StoreTypeMeta, len(metas))
	copy(out, metas)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Label < out[j].Label
	})
	return out
}
```

Add `"sort"` to the import block.

- [ ] **Step 4: Register PostGIS meta**

In `postgis.go`, find the `register("PostGIS", ...)` init and add alongside it a `registerMeta`. If registration happens in an `init()`, add:

```go
func init() {
	registerMeta(StoreTypeMeta{
		Type: "PostGIS", Kind: "datastore", Category: "Vector", Label: "PostGIS Database",
		Params: []ParamField{
			{Key: "host", Label: "Host", Type: "text", Default: "self", Required: true},
			{Key: "port", Label: "Port", Type: "number", Default: "5432", Required: false},
			{Key: "database", Label: "Database", Type: "text", Required: false},
			{Key: "user", Label: "User", Type: "text", Required: false},
			{Key: "passwd", Label: "Password", Type: "password", Required: false},
			{Key: "schema", Label: "Schema", Type: "text", Default: "public", Required: false},
		},
	})
}
```

Add analogous `registerMeta` init blocks in `files.go` (Shapefile, Directory, GeoJSON — param `url`/`path`), `geoparquet.go` (GeoParquet — param `path`), and `cog.go` (GeoTIFF — kind `coveragestore`, category `Raster`, param `url`). Use whatever store-type string each connector already registers with `register(...)`.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./services/catalog/internal/connect/ -run TestStoreTypesIncludesPostGIS -v`
Expected: PASS.

- [ ] **Step 6: Build + commit**

```bash
go build ./services/catalog/...
git add services/catalog/internal/connect
git commit -m "feat(catalog): store-type registry with UI param schema (E1)"
```

---

## Task 2: API v1 store endpoints — types, create, update, delete, test (backend)

**Files:**
- Create: `services/catalog/internal/rest/apiv1_stores.go`
- Modify: `services/catalog/internal/rest/apiv1.go:12-19` (register routes)
- Test: `services/catalog/internal/rest/apiv1_stores_test.go`

**Interfaces:**
- Consumes: `connect.StoreTypes()`, `connect.ForType`, `store.CreateStore/GetStore/UpdateStore/DeleteStore/ListAllStores`.
- Produces: routes `GET /api/v1/store-types`, `POST /api/v1/stores`, `PUT /api/v1/stores/{ws}/{store}`, `DELETE /api/v1/stores/{ws}/{store}`, `POST /api/v1/stores/{ws}/{store}/test`. Request body `type storeReq struct { Workspace, Name, Type, Kind, Description string; Enabled bool; Connection map[string]string }`.

- [ ] **Step 1: Write the failing test**

```go
// services/catalog/internal/rest/apiv1_stores_test.go
package rest

import (
	"net/http/httptest"
	"testing"
)

func TestStoreTypesEndpoint(t *testing.T) {
	a := newTestAPI(t) // existing test helper; if absent, see Step 3 note
	req := httptest.NewRequest("GET", "/api/v1/store-types", nil)
	rec := httptest.NewRecorder()
	a.mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d", rec.Code)
	}
	if !contains(rec.Body.String(), "PostGIS") {
		t.Fatalf("expected PostGIS in body: %s", rec.Body.String())
	}
}
```

If `newTestAPI`/`a.mux()`/`contains` helpers do not exist, add a minimal `apiv1_stores_test.go` variant that calls the handler function directly (`a.v1StoreTypes`) with a constructed `*api{}` whose `s` is nil (store-types needs no DB). Prefer the direct-handler form to avoid DB in unit test:

```go
func TestStoreTypesEndpoint(t *testing.T) {
	a := &api{}
	rec := httptest.NewRecorder()
	a.v1StoreTypes(rec, httptest.NewRequest("GET", "/api/v1/store-types", nil))
	if rec.Code != 200 || !contains(rec.Body.String(), "PostGIS") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
```

Add imports `net/http/httptest`, `strings`, `testing`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/catalog/internal/rest/ -run TestStoreTypesEndpoint -v`
Expected: FAIL — `v1StoreTypes` undefined.

- [ ] **Step 3: Implement handlers**

```go
// services/catalog/internal/rest/apiv1_stores.go
package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/connect"
	"github.com/giti/giti/services/catalog/internal/model"
)

func (a *api) v1StoreTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, connect.StoreTypes())
}

type storeReq struct {
	Workspace   string            `json:"workspace"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Kind        string            `json:"kind"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Connection  map[string]string `json:"connection"`
}

func (b storeReq) toModel() model.Store {
	kind := b.Kind
	if kind == "" {
		kind = "datastore"
	}
	return model.Store{Workspace: b.Workspace, Name: b.Name, Type: b.Type, Kind: kind,
		Description: b.Description, Enabled: b.Enabled, Connection: b.Connection}
}

func (a *api) v1CreateStore(w http.ResponseWriter, r *http.Request) {
	var b storeReq
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Workspace == "" || b.Name == "" || b.Type == "" {
		http.Error(w, "workspace, name, type required", http.StatusBadRequest)
		return
	}
	st := b.toModel()
	if err := a.s.CreateStore(r.Context(), st); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.store.created", map[string]string{"name": st.Name, "workspace": st.Workspace})
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"name": st.Name})
}

func (a *api) v1UpdateStore(w http.ResponseWriter, r *http.Request) {
	ws, name := r.PathValue("ws"), r.PathValue("store")
	var b storeReq
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	b.Workspace, b.Name = ws, name
	if err := a.s.UpdateStore(r.Context(), ws, name, b.toModel()); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1DeleteStore(w http.ResponseWriter, r *http.Request) {
	ws, name := r.PathValue("ws"), r.PathValue("store")
	recurse := r.URL.Query().Get("recurse") == "true"
	if err := a.s.DeleteStore(r.Context(), ws, name, recurse); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// v1TestStore validates connectivity for a candidate store WITHOUT persisting.
func (a *api) v1TestStore(w http.ResponseWriter, r *http.Request) {
	var b storeReq
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	c, err := connect.ForType(b.Type)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := c.Validate(r.Context(), b.toModel()); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
```

Note: `{ws}/{store}/test` posts a body with `workspace`/`name` optional; the handler reads type+connection only.

- [ ] **Step 4: Register routes**

In `apiv1.go` `apiV1Routes`, add:

```go
	mux.HandleFunc("GET /api/v1/store-types", a.v1StoreTypes)
	mux.HandleFunc("POST /api/v1/stores", a.v1CreateStore)
	mux.HandleFunc("PUT /api/v1/stores/{ws}/{store}", a.v1UpdateStore)
	mux.HandleFunc("DELETE /api/v1/stores/{ws}/{store}", a.v1DeleteStore)
	mux.HandleFunc("POST /api/v1/stores/{ws}/{store}/test", a.v1TestStore)
	mux.HandleFunc("POST /api/v1/stores/test", a.v1TestStore)
```

- [ ] **Step 5: Run test + build**

Run: `go test ./services/catalog/internal/rest/ -run TestStoreTypesEndpoint -v && go build ./services/catalog/...`
Expected: PASS + clean build.

- [ ] **Step 6: Commit**

```bash
git add services/catalog/internal/rest
git commit -m "feat(catalog): api v1 store crud + connection-test + store-types (E1)"
```

---

## Task 3: Live backend verification through Traefik

**Files:** none (verification only).

- [ ] **Step 1: Rebuild + restart catalog**

```bash
cd deploy/compose && set -a && . ./.env && set +a && docker compose up -d --build catalog && sleep 6
```

- [ ] **Step 2: Verify endpoints**

```bash
curl -s http://localhost/api/v1/store-types | python3 -c 'import sys,json;[print(t["category"],t["type"]) for t in json.load(sys.stdin)]'
curl -s -X POST http://localhost/api/v1/stores/test -H 'Content-Type: application/json' -d '{"type":"PostGIS","connection":{"host":"self","schema":"public"}}'
```

Expected: store-types lists Vector/Raster entries incl. PostGIS; test returns `{"ok":true}`.

- [ ] **Step 3: Verify create + delete round-trip**

```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost/api/v1/stores -H 'Content-Type: application/json' -d '{"workspace":"iran","name":"e1probe","type":"PostGIS","connection":{"host":"self","schema":"public"}}'
curl -s http://localhost/api/v1/stores | grep -q e1probe && echo listed
curl -s -o /dev/null -w '%{http_code}\n' -X DELETE http://localhost/api/v1/stores/iran/e1probe
```

Expected: 201, `listed`, 204.

- [ ] **Step 4: Commit (no-op marker)** — none; proceed.

---

## Task 4: Frontend grouped nav config + Sidebar sections

**Files:**
- Create: `frontend/src/config/nav.ts`
- Modify: `frontend/src/components/layout/Sidebar.tsx`
- Modify: `frontend/src/i18n/dictionaries/en.ts`, `fa.ts` (group labels)

**Interfaces:**
- Produces: `type NavItem = { key: string; href: string; icon: string }`; `type NavGroup = { labelKey: string; items: NavItem[] }`; `export const navGroups: NavGroup[]`.

- [ ] **Step 1: Create nav config**

```ts
// frontend/src/config/nav.ts
export type NavItem = { key: string; href: string; icon: string };
export type NavGroup = { labelKey: string; items: NavItem[] };

export const navGroups: NavGroup[] = [
  {
    labelKey: "navgroup.data",
    items: [
      { key: "nav.overview", href: "/dashboard", icon: "overview" },
      { key: "nav.workspaces", href: "/dashboard/workspaces", icon: "workspaces" },
      { key: "nav.stores", href: "/dashboard/stores", icon: "stores" },
      { key: "nav.layers", href: "/dashboard/layers", icon: "layers" },
      { key: "nav.layerGroups", href: "/dashboard/layer-groups", icon: "layers" },
      { key: "nav.styles", href: "/dashboard/styles", icon: "styles" },
      { key: "nav.map", href: "/map", icon: "map" },
    ],
  },
  {
    labelKey: "navgroup.tiles",
    items: [{ key: "nav.tileCache", href: "/dashboard/tile-cache", icon: "tileCache" }],
  },
  {
    labelKey: "navgroup.services",
    items: [
      { key: "nav.wps", href: "/dashboard/wps", icon: "wps" },
      { key: "nav.conversions", href: "/dashboard/conversions", icon: "conversions" },
    ],
  },
  {
    labelKey: "navgroup.security",
    items: [{ key: "nav.security", href: "/dashboard/security", icon: "security" }],
  },
  {
    labelKey: "navgroup.system",
    items: [{ key: "nav.settings", href: "/dashboard/settings", icon: "settings" }],
  },
];
```

- [ ] **Step 2: Add group label keys to both dictionaries**

en.ts: `"navgroup.data": "Data", "navgroup.tiles": "Tile Caching", "navgroup.services": "Services", "navgroup.security": "Security", "navgroup.system": "System", "nav.layerGroups": "Layer Groups",`
fa.ts: `"navgroup.data": "داده", "navgroup.tiles": "کش کاشی", "navgroup.services": "سرویس‌ها", "navgroup.security": "امنیت", "navgroup.system": "سیستم", "nav.layerGroups": "گروه لایه‌ها",`

- [ ] **Step 3: Render groups in Sidebar**

Replace the flat `items` array + `.map` in `Sidebar.tsx` with an import of `navGroups` and a nested render: for each group render a small uppercase `t(group.labelKey)` header (`text-[10px] uppercase tracking-wide text-[var(--color-muted)] px-3 pt-4 pb-1`) then its items (keep the existing `Meridian` active-marker + `isActive` logic). Remove the now-duplicated hard-coded `map` link (it's in the data group).

- [ ] **Step 4: Typecheck + verify live**

```bash
cd frontend && npx tsc --noEmit
cd ../deploy/compose && set -a && . ./.env && set +a && docker compose up -d --build frontend && sleep 6
curl -s -o /dev/null -w '%{http_code}\n' http://localhost/en/dashboard
```

Expected: tsc clean, 200. Visually: grouped sidebar with section headers.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/config/nav.ts frontend/src/components/layout/Sidebar.tsx frontend/src/i18n
git commit -m "feat(frontend): grouped enterprise sidebar nav (E1)"
```

---

## Task 5: Toaster + Drawer UI primitives

**Files:**
- Create: `frontend/src/components/ui/Toast.tsx`
- Create: `frontend/src/components/ui/Drawer.tsx`
- Modify: `frontend/src/app/[locale]/(app)/layout.tsx` (mount `<Toaster/>`)

**Interfaces:**
- Produces: `Toaster` component + `useToast()` → `{ toast: (t: {title: string; tone?: "ok"|"err"}) => void }`; `Drawer({open,onClose,title,children,footer})`.

- [ ] **Step 1: Toast provider**

```tsx
// frontend/src/components/ui/Toast.tsx
"use client";
import { createContext, useContext, useState, useCallback } from "react";
import { Check, AlertCircle } from "lucide-react";

type Tone = "ok" | "err";
type Item = { id: number; title: string; tone: Tone };
const Ctx = createContext<{ toast: (t: { title: string; tone?: Tone }) => void }>({ toast: () => {} });

export function useToast() { return useContext(Ctx); }

export function Toaster({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<Item[]>([]);
  const toast = useCallback((t: { title: string; tone?: Tone }) => {
    const id = Date.now() + Math.random();
    setItems((x) => [...x, { id, title: t.title, tone: t.tone ?? "ok" }]);
    setTimeout(() => setItems((x) => x.filter((i) => i.id !== id)), 3500);
  }, []);
  return (
    <Ctx.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-4 end-4 z-[60] flex flex-col gap-2">
        {items.map((i) => (
          <div key={i.id} className="flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm shadow-lg">
            {i.tone === "ok" ? <Check size={15} className="text-[var(--color-ok)]" /> : <AlertCircle size={15} className="text-[var(--color-err)]" />}
            {i.title}
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}
```

- [ ] **Step 2: Drawer**

```tsx
// frontend/src/components/ui/Drawer.tsx
"use client";
import { useEffect } from "react";
import { X } from "lucide-react";

export function Drawer({ open, onClose, title, children, footer }: {
  open: boolean; onClose: () => void; title: string;
  children: React.ReactNode; footer?: React.ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40" onClick={onClose}>
      <div className="flex h-full w-full max-w-xl flex-col bg-[var(--color-surface)] shadow-2xl" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-3">
          <h2 className="font-display text-base font-semibold">{title}</h2>
          <button onClick={onClose} className="rounded-md p-1 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)]"><X size={18} /></button>
        </div>
        <div className="flex-1 overflow-auto px-5 py-4">{children}</div>
        {footer && <div className="border-t border-[var(--color-border)] px-5 py-3">{footer}</div>}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Mount Toaster in app layout**

In `(app)/layout.tsx`, wrap the existing shell children with `<Toaster>...</Toaster>` (import from `@/components/ui/Toast`).

- [ ] **Step 4: Typecheck + commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/components/ui/Toast.tsx frontend/src/components/ui/Drawer.tsx "frontend/src/app/[locale]/(app)/layout.tsx"
git commit -m "feat(frontend): Toaster + Drawer primitives (E1)"
```

---

## Task 6: Command palette (⌘K)

**Files:**
- Create: `frontend/src/components/layout/CommandPalette.tsx`
- Modify: `frontend/src/app/[locale]/(app)/layout.tsx` (mount)
- Modify: `frontend/package.json` (add `cmdk`)

**Interfaces:**
- Consumes: `navGroups` from `@/config/nav`.
- Produces: `CommandPalette` (self-contained; listens for ⌘K/Ctrl-K).

- [ ] **Step 1: Add dependency**

```bash
cd frontend && npm install cmdk
```

- [ ] **Step 2: Implement palette**

```tsx
// frontend/src/components/layout/CommandPalette.tsx
"use client";
import { useEffect, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import { Command } from "cmdk";
import { navGroups } from "@/config/nav";
import { useT } from "@/i18n/provider";
import { icons } from "@/components/icons";

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const { t } = useT();
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-[70] flex items-start justify-center bg-black/40 p-4 pt-[15vh]" onClick={() => setOpen(false)}>
      <Command onClick={(e) => e.stopPropagation()} className="w-full max-w-lg overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl">
        <Command.Input autoFocus placeholder={t("cmd.placeholder")} className="w-full border-b border-[var(--color-border)] bg-transparent px-4 py-3 text-sm outline-none" />
        <Command.List className="max-h-80 overflow-auto p-2">
          <Command.Empty className="px-3 py-6 text-center text-sm text-[var(--color-muted)]">{t("cmd.empty")}</Command.Empty>
          {navGroups.map((g) => (
            <Command.Group key={g.labelKey} heading={t(g.labelKey)} className="px-1 text-xs text-[var(--color-muted)]">
              {g.items.map((it) => {
                const Icon = icons[it.icon];
                return (
                  <Command.Item key={it.href} value={t(it.key)} onSelect={() => { setOpen(false); router.push(`/${locale}${it.href}`); }}
                    className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm text-[var(--color-text)] aria-selected:bg-[var(--color-surface-2)]">
                    <Icon size={15} /> {t(it.key)}
                  </Command.Item>
                );
              })}
            </Command.Group>
          ))}
        </Command.List>
      </Command>
    </div>
  );
}
```

- [ ] **Step 3: Add keys + mount**

en.ts: `"cmd.placeholder": "Search pages…", "cmd.empty": "No results.",`
fa.ts: `"cmd.placeholder": "جستجوی صفحه‌ها…", "cmd.empty": "نتیجه‌ای نیست.",`
Mount `<CommandPalette/>` in `(app)/layout.tsx`.

- [ ] **Step 4: Typecheck + verify live + commit**

```bash
cd frontend && npx tsc --noEmit
cd ../deploy/compose && set -a && . ./.env && set +a && docker compose up -d --build frontend && sleep 6
curl -s -o /dev/null -w '%{http_code}\n' http://localhost/en/dashboard
git -C ../.. add frontend/src/components/layout/CommandPalette.tsx frontend/package.json frontend/package-lock.json "frontend/src/app/[locale]/(app)/layout.tsx" frontend/src/i18n
git -C ../.. commit -m "feat(frontend): ⌘K command palette (E1)"
```

Expected: tsc clean, 200; ⌘K opens palette, selecting navigates.

---

## Task 7: Stores API client — types, create/update/delete/test

**Files:**
- Modify: `frontend/src/api/dashboard/stores/types.ts`
- Modify: `frontend/src/api/dashboard/stores/api.ts`
- Modify: `frontend/src/api/client.ts` (add `apiJson` for typed POST returning JSON, and `apiDelete`)

**Interfaces:**
- Produces: `StoreType`, `ParamField` types; `listStoreTypes()`, `createStore(req)`, `updateStore(ws,name,req)`, `deleteStore(ws,name,recurse?)`, `testStore(req)`; `apiJson<T>(path, body)`, `apiDelete(path)` in client.

- [ ] **Step 1: Extend client**

Add to `client.ts`:

```ts
/** apiJson posts a JSON body and parses a JSON response. */
export async function apiJson<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, { method: "POST", headers: authHeaders({ "Content-Type": "application/json" }), body: JSON.stringify(body) });
  if (!res.ok) throw new ApiError(res.status, await safeText(res));
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function apiDelete(path: string): Promise<void> {
  const res = await fetch(BASE + path, { method: "DELETE", headers: authHeaders() });
  if (!res.ok && res.status !== 404) throw new ApiError(res.status, await safeText(res));
}

export async function apiPut(path: string, body: unknown): Promise<void> {
  const res = await fetch(BASE + path, { method: "PUT", headers: authHeaders({ "Content-Type": "application/json" }), body: JSON.stringify(body) });
  if (!res.ok) throw new ApiError(res.status, await safeText(res));
}
```

- [ ] **Step 2: Extend types**

```ts
// append to frontend/src/api/dashboard/stores/types.ts
export interface ParamField { key: string; label: string; type: string; default?: string; required: boolean; }
export interface StoreType { type: string; kind: string; category: string; label: string; params: ParamField[]; }
export interface StoreReq {
  workspace: string; name: string; type: string; kind?: string;
  description?: string; enabled: boolean; connection: Record<string, string>;
}
export interface TestResult { ok: boolean; error?: string; }
```

- [ ] **Step 3: Extend api**

```ts
// append to frontend/src/api/dashboard/stores/api.ts
import { apiJson, apiPut, apiDelete } from "@/api/client";
import type { StoreType, StoreReq, TestResult } from "./types";

export function listStoreTypes(): Promise<StoreType[]> {
  return apiFetch<StoreType[]>("/api/v1/store-types");
}
export function createStore(req: StoreReq): Promise<{ name: string }> {
  return apiJson<{ name: string }>("/api/v1/stores", req);
}
export function updateStore(ws: string, name: string, req: StoreReq): Promise<void> {
  return apiPut(`/api/v1/stores/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`, req);
}
export function deleteStore(ws: string, name: string, recurse = false): Promise<void> {
  return apiDelete(`/api/v1/stores/${encodeURIComponent(ws)}/${encodeURIComponent(name)}?recurse=${recurse}`);
}
export function testStore(req: Partial<StoreReq> & { type: string; connection: Record<string, string> }): Promise<TestResult> {
  return apiJson<TestResult>("/api/v1/stores/test", req);
}
```

(Ensure `apiFetch` import already present; keep the existing `createPgStore`/`publishTable`/`listStoreTables` — the wizard will use the generic `createStore` instead of `createPgStore`.)

- [ ] **Step 4: Typecheck + commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/api
git commit -m "feat(frontend): stores api client — types/crud/test (E1)"
```

---

## Task 8: New Data Source wizard (type picker + dynamic form + test)

**Files:**
- Create: `frontend/src/components/dashboard/stores/NewStoreWizard.tsx`
- Modify: `frontend/src/i18n/dictionaries/{en,fa}.ts`

**Interfaces:**
- Consumes: `listStoreTypes`, `createStore`, `testStore`, `StoreType`; `listWorkspaces`; `useToast`; `Drawer`; `Input/Select`.
- Produces: `NewStoreWizard({ open, onClose, onCreated })`.

- [ ] **Step 1: Implement wizard**

Two-step drawer. Step A: grouped type picker (group `StoreType[]` by `category`, click a card → step B). Step B: workspace select + store name + dynamic fields from `type.params` (render `Input type={param.type}` or `Select`), a **Test connection** button calling `testStore({type, connection})` → toast ok/err, and **Create** calling `createStore({workspace,name,type,kind,enabled:true,connection})` → toast + `onCreated()`. Back button returns to step A. Build `connection` from param defaults initially.

```tsx
// frontend/src/components/dashboard/stores/NewStoreWizard.tsx
"use client";
import { useEffect, useState } from "react";
import { ArrowLeft, Database, Layers, Cloud, Zap } from "lucide-react";
import { useT } from "@/i18n/provider";
import { Drawer } from "@/components/ui/Drawer";
import { Input, Select } from "@/components/ui/Field";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { listStoreTypes, createStore, testStore } from "@/api/dashboard/stores/api";
import type { StoreType } from "@/api/dashboard/stores/types";
import { listWorkspaces } from "@/api/dashboard/workspaces/api";
import type { Workspace } from "@/api/dashboard/workspaces/types";

const catIcon: Record<string, typeof Database> = { Vector: Layers, Raster: Database, Cascade: Cloud };

export function NewStoreWizard({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const { t } = useT();
  const { toast } = useToast();
  const [types, setTypes] = useState<StoreType[]>([]);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [sel, setSel] = useState<StoreType | null>(null);
  const [ws, setWs] = useState("");
  const [name, setName] = useState("");
  const [conn, setConn] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    listStoreTypes().then(setTypes).catch(() => setTypes([]));
    listWorkspaces().then((w) => { setWorkspaces(w); if (w[0]) setWs(w[0].name); }).catch(() => setWorkspaces([]));
    setSel(null); setName("");
  }, [open]);

  function pick(st: StoreType) {
    setSel(st);
    const c: Record<string, string> = {};
    st.params.forEach((p) => (c[p.key] = p.default ?? ""));
    setConn(c);
  }
  async function test() {
    const r = await testStore({ type: sel!.type, connection: conn });
    toast(r.ok ? { title: t("stores.testOk") } : { title: r.error || t("stores.testFail"), tone: "err" });
  }
  async function create() {
    if (!ws || !name.trim()) return;
    setBusy(true);
    try {
      await createStore({ workspace: ws, name: name.trim(), type: sel!.type, kind: sel!.kind, enabled: true, connection: conn });
      toast({ title: t("stores.created") });
      onCreated();
    } catch (e) { toast({ title: (e as Error).message, tone: "err" }); }
    finally { setBusy(false); }
  }

  const cats = ["Vector", "Raster", "Cascade"];
  return (
    <Drawer open={open} onClose={onClose} title={sel ? sel.label : t("stores.newSource")}
      footer={sel && (
        <div className="flex justify-between">
          <Button variant="ghost" onClick={() => setSel(null)}><ArrowLeft size={15} /> {t("action.back")}</Button>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={test}><Zap size={15} /> {t("stores.test")}</Button>
            <Button onClick={create} disabled={busy}>{busy ? t("common.loading") : t("action.create")}</Button>
          </div>
        </div>
      )}>
      {!sel ? (
        <div className="space-y-5">
          {cats.map((cat) => {
            const list = types.filter((x) => x.category === cat);
            if (!list.length) return null;
            const Icon = catIcon[cat] ?? Database;
            return (
              <div key={cat}>
                <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{cat}</div>
                <div className="grid grid-cols-2 gap-2">
                  {list.map((st) => (
                    <button key={st.type} onClick={() => pick(st)}
                      className="flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2.5 text-start text-sm hover:border-[var(--color-primary)]">
                      <Icon size={16} className="text-[var(--color-primary)]" /> {st.label}
                    </button>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <Select label={t("workspaces.title")} value={ws} onChange={(e) => setWs(e.target.value)}>
              {workspaces.map((w) => <option key={w.name} value={w.name}>{w.name}</option>)}
            </Select>
            <Input label={t("stores.name")} value={name} onChange={(e) => setName(e.target.value)} placeholder="my_store" />
          </div>
          <div className="rounded-lg border border-[var(--color-border)] p-3">
            <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{t("stores.connection")}</div>
            <div className="grid grid-cols-2 gap-3">
              {sel.params.map((p) => (
                <Input key={p.key} label={p.label + (p.required ? " *" : "")} type={p.type === "number" ? "number" : p.type === "password" ? "password" : "text"}
                  value={conn[p.key] ?? ""} onChange={(e) => setConn((c) => ({ ...c, [p.key]: e.target.value }))} />
              ))}
            </div>
          </div>
        </div>
      )}
    </Drawer>
  );
}
```

- [ ] **Step 2: Add keys (both dicts)**

en: `"stores.newSource": "New data source", "stores.connection": "Connection", "stores.test": "Test connection", "stores.testOk": "Connection OK", "stores.testFail": "Connection failed", "stores.created": "Store created", "action.back": "Back",`
fa: `"stores.newSource": "منبع داده جدید", "stores.connection": "اتصال", "stores.test": "آزمایش اتصال", "stores.testOk": "اتصال برقرار است", "stores.testFail": "اتصال ناموفق", "stores.created": "منبع ساخته شد", "action.back": "بازگشت",`

- [ ] **Step 3: Typecheck + commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/components/dashboard/stores/NewStoreWizard.tsx frontend/src/i18n
git commit -m "feat(frontend): New Data Source wizard — type picker + test (E1)"
```

---

## Task 9: Stores page — wire wizard, edit, delete

**Files:**
- Modify: `frontend/src/components/dashboard/pages/Stores.tsx`

**Interfaces:**
- Consumes: `NewStoreWizard`, `deleteStore`, `updateStore`, existing `listStores`, `PublishModal`.

- [ ] **Step 1: Replace AddStoreModal usage with NewStoreWizard**

In `Stores.tsx`: replace the `<Button onClick={() => setAdding(true)}>` target modal with `<NewStoreWizard open={adding} onClose={() => setAdding(false)} onCreated={() => { setAdding(false); load(); }} />`. Remove the old `AddStoreModal` component + its `createPgStore` import (dead code) OR keep file but delete the inline component. Add a per-row **Delete** action (icon button) → `deleteStore(s.workspace, s.name, false)` with confirm + toast + reload; on 409 (has layers) toast "detach layers first".

- [ ] **Step 2: Add delete keys**

en: `"stores.delete": "Delete store", "stores.deleteConfirm": "Delete store {name}? This cannot be undone.", "stores.hasLayers": "Store has published layers — remove them first.",`
fa: `"stores.delete": "حذف منبع", "stores.deleteConfirm": "منبع {name} حذف شود؟ بازگشت‌پذیر نیست.", "stores.hasLayers": "منبع لایه‌ی منتشرشده دارد — ابتدا آن‌ها را حذف کنید.",`

- [ ] **Step 3: Typecheck + verify live**

```bash
cd frontend && npx tsc --noEmit
cd ../deploy/compose && set -a && . ./.env && set +a && docker compose up -d --build frontend && sleep 6
curl -s -o /dev/null -w '%{http_code}\n' http://localhost/en/dashboard/stores
```

Expected: tsc clean, 200.

- [ ] **Step 4: Commit**

```bash
git -C ../.. add frontend/src/components/dashboard/pages/Stores.tsx frontend/src/i18n
git -C ../.. commit -m "feat(frontend): stores page — wizard + delete (E1)"
```

---

## Task 10: E1 end-to-end acceptance

**Files:** none.

- [ ] **Step 1: Manual + curl acceptance through Traefik**

```bash
# type picker source
curl -s http://localhost/api/v1/store-types | python3 -c 'import sys,json;print(len(json.load(sys.stdin)),"types")'
# create via UI-equivalent call, introspect, publish, delete
curl -s -o /dev/null -w 'create %{http_code}\n' -X POST http://localhost/api/v1/stores -H 'Content-Type: application/json' -d '{"workspace":"iran","name":"e1accept","type":"PostGIS","enabled":true,"connection":{"host":"self","schema":"public"}}'
curl -s http://localhost/api/v1/stores/iran/e1accept/tables | python3 -c 'import sys,json;print(len(json.load(sys.stdin)),"tables")'
curl -s -o /dev/null -w 'delete %{http_code}\n' -X DELETE http://localhost/api/v1/stores/iran/e1accept
```

Expected: N types; create 201; tables listed; delete 204.

- [ ] **Step 2: UI checklist (browser at http://localhost/en/dashboard)**
  - Grouped sidebar with section headers renders.
  - ⌘K opens palette; selecting a page navigates.
  - Stores → "Add store" opens the drawer wizard; picking PostGIS shows dynamic fields; Test connection toasts OK; Create adds the store; Publish tables works; Delete removes it.

- [ ] **Step 3: Tag sprint done**

```bash
git commit --allow-empty -m "chore: E1 admin shell + connect-anywhere stores complete"
```

---

## Self-Review

- **Spec coverage (E1 scope):** modern shell (Tasks 4–6 nav/toast/drawer/palette) ✅; New Data Source type picker + per-type wizard + test connection (Tasks 1,2,7,8) ✅; Stores CRUD + introspect→publish (Tasks 2,9, existing PublishModal) ✅; expose every registered connector + param schema (Task 1) ✅. Raster/SQL-Server/CSV/KML/cascade forms appear once their connectors register `Meta()` — backend enable is E8; their picker entries show when registered.
- **Placeholders:** none — every step has concrete code/commands.
- **Type consistency:** `StoreReq`/`StoreType`/`ParamField` identical across backend JSON tags (Task 1/2) and TS types (Task 7); `createStore`/`testStore`/`deleteStore` names consistent across Tasks 7–9.
