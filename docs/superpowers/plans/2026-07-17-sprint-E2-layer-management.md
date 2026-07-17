# Sprint E2 — Layer Management — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use `- [ ]` checkboxes.

**Goal:** Full GeoServer-grade layer editing: list at scale, tabbed edit (Data / Publishing / CRS / Feature-Type-Details / SQL View), bbox compute + reprojection, metadata/keywords, feature-type introspection.

**Architecture:** Catalog API v1 gains `PATCH /layers/{ws}/{name}` and `PATCH /featuretypes`, bbox compute/reproject via PostGIS, CRS lookup, SQL-view resource. Frontend replaces the flat Layers table with a searchable/paginated list + a tabbed edit Drawer wired to those endpoints. Depends on E1 shell (Drawer, Toaster, apiJson/apiPut).

**Tech Stack:** Go (catalog, pgx), Next.js 15/React 19/TS, MapLibre (reuse LayerPreview).

## Global Constraints

Same as E1 (`github.com/giti/giti`, `GITI_*`, API v1 at `/api/v1`, i18n both dicts, CSS-var colors, live-verify through Traefik). CRS handling uses PostGIS `spatial_ref_sys` + `ST_Transform`.

## File Structure

- `services/catalog/internal/store/layers_detail.go` — bbox compute/reproject, PATCH helpers, SQL-view create.
- `services/catalog/internal/rest/apiv1_layers.go` — PATCH layer/featuretype, `/srs/{code}`, `/layers/{ws}/{name}/bbox` compute.
- `services/catalog/internal/model/*.go` — add `Metadata`, `Keywords`, `Title`, `Abstract`, `Advertised`, declared/native SRS, `SRSHandling`, `Queryable`, `Opaque` fields where missing (+ migration).
- `services/catalog/migrations/NNNN_layer_metadata.sql` — new columns.
- `frontend/src/api/dashboard/layers/{types,api}.ts` — extend detail + patch + srs + bbox.
- `frontend/src/components/dashboard/layers/LayerEditDrawer.tsx` — tabbed editor.
- `frontend/src/components/dashboard/pages/Layers.tsx` — search/paginate + open editor.

## Task 1: Layer metadata columns + migration (backend)

**Files:** Create `services/catalog/migrations/0007_layer_metadata.sql`; Modify `model` structs; Test `store` package.

- [ ] Add SQL migration adding to `layers`/`resources` (as appropriate): `title text, abstract text, keywords text[], metadata jsonb default '{}', advertised bool default true, queryable bool default true, opaque bool default false, declared_srs text, srs_handling text default 'FORCE'`. Follow the existing embedded-migration numbering/pattern in `services/catalog/migrations/`.
- [ ] Extend `model.Layer`/`model.FeatureType` with matching fields + json tags.
- [ ] **Test:** insert a featuretype, set keywords + metadata, read back — assert round-trip (integration test with `GITI_TEST_DATABASE_URL`).
- [ ] Run: `go test ./services/catalog/internal/store/ -run Metadata` → PASS.
- [ ] Commit `feat(catalog): layer metadata/keywords/srs columns (E2)`.

## Task 2: bbox compute + reproject + SRS lookup (backend)

**Files:** Create `services/catalog/internal/store/layers_detail.go` funcs; Modify `apiv1_layers.go`; Test store.

**Interfaces produced:** `func (s *Store) ComputeBbox(ctx, ws, name, mode string) ([]float64, error)` (mode: `data`|`srs`); `func (s *Store) SRSInfo(ctx, code string) (SRSInfo, error)` where `SRSInfo{Code, Name string; Bounds []float64}`; routes `GET /api/v1/srs/{code}`, `POST /api/v1/layers/{ws}/{name}/bbox?mode=data`.

- [ ] **Test (failing):** `ComputeBbox(ctx,"iran","cities","data")` returns 4-float 4326 extent for seeded data.
- [ ] Implement `ComputeBbox`: resolve featuretype table+geom (reuse `GetLayerDetail` internals), `SELECT ST_XMin/…(ST_Extent(ST_Transform(geom,4326)))`. `mode=srs` → from `spatial_ref_sys` bounds via declared SRS.
- [ ] Implement `SRSInfo`: `SELECT srtext FROM spatial_ref_sys WHERE auth_srid=$1` → parse name; bounds via `ST_TileEnvelope`/`ST_Transform` of the CRS domain (fallback world).
- [ ] Wire the two routes in `apiv1_layers.go`; register in `apiV1Routes`.
- [ ] Run test → PASS; `go build ./services/catalog/...`.
- [ ] Live verify: `curl -s -X POST http://localhost/api/v1/layers/iran/cities/bbox?mode=data`, `curl -s http://localhost/api/v1/srs/4326`.
- [ ] Commit `feat(catalog): bbox compute/reproject + SRS lookup (E2)`.

## Task 3: PATCH layer + featuretype (backend)

**Files:** Modify `apiv1_layers.go`, `store` update methods.

**Interfaces produced:** `PATCH /api/v1/layers/{ws}/{name}` body `{title?,abstract?,keywords?,metadata?,defaultStyle?,alternateStyles?,queryable?,opaque?,advertised?,enabled?}`; `PATCH /api/v1/featuretypes/{ws}/{store}/{name}` body `{title?,abstract?,keywords?,srs?,declaredSrs?,srsHandling?,bbox?}`. Partial update (only provided keys).

- [ ] **Test (failing):** PATCH layer title → GET detail shows new title.
- [ ] Implement `store.PatchLayer` / `store.PatchFeatureType` using COALESCE-style dynamic update (build SET clause from non-nil pointer fields; keep parameterized).
- [ ] Handlers decode into pointer-field structs, call store, return 204; publish `catalog.layer.updated` (bumps tile generation).
- [ ] Run test → PASS; build.
- [ ] Live verify PATCH round-trip via curl.
- [ ] Commit `feat(catalog): PATCH layer + featuretype (E2)`.

## Task 4: SQL View resource (backend)

**Files:** Modify `store` (create resource with `native_sql`), `connect/postgis.go` introspect a view; migration adds `native_sql text` to resources.

- [ ] Add `native_sql` column; `CreateSQLView(ctx, ft model.FeatureType, sql string)` stores the query; WFS/WMS resolvers already read `native_name` — extend to prefer `native_sql` wrapped as `(<sql>) AS t` when present. **Guard:** parameter placeholders `%param%` validated against an allowlist.
- [ ] **Test:** create a SQL-view featuretype `SELECT id,name,geom FROM cities WHERE pop>%min%`, resolve → returns rows; injection attempt rejected.
- [ ] Route: `POST /api/v1/stores/{ws}/{store}/sql-views` body `{name, sql, geomColumn, srs, params:[{name,default,regexp}]}`.
- [ ] Build + live verify: create SQL view, WFS GetFeature returns filtered rows.
- [ ] Commit `feat(catalog): SQL view resources (E2)`.

## Task 5: Layers API client extend (frontend)

**Files:** Modify `frontend/src/api/dashboard/layers/{types,api}.ts`.

**Interfaces produced:** `patchLayer(ws,name,patch)`, `patchFeatureType(ws,store,name,patch)`, `computeBbox(ws,name,mode)`, `getSRS(code)`, `createSqlView(ws,store,req)`; types `LayerPatch`, `FeatureTypePatch`, `SRSInfo`, `SqlViewReq`.

- [ ] Add types + functions (use `apiJson`/`apiPut`/`apiFetch`; add `apiPatch` to client mirroring `apiPut` with method PATCH).
- [ ] Typecheck; commit `feat(frontend): layers api client — patch/bbox/srs/sqlview (E2)`.

## Task 6: Layer edit Drawer — tabs (frontend)

**Files:** Create `LayerEditDrawer.tsx`; Modify `Layers.tsx`.

- [ ] Build tabbed Drawer (tabs: Data / Publishing / CRS / Feature Type / SQL View). Data: title/abstract/keywords(chip input)/enabled/advertised. Publishing: default-style select (from `listStyles`) + alternate-styles multiselect + queryable/opaque. CRS: native SRS (read-only) + declared SRS input + SRS-handling select + **Compute bbox** buttons (data/srs) showing native + lat/lon boxes. Feature Type: attributes table from `getLayerDetail`. SQL View: shown only for DB stores — SQL textarea + params + create. Save calls `patchLayer` + `patchFeatureType`, toast + reload.
- [ ] Add i18n keys (both dicts) for all tab labels/fields.
- [ ] Wire `Layers.tsx`: add search box + client-side pagination (or `?limit/offset`), row click opens `LayerEditDrawer`, keep detail-page link + map link.
- [ ] Typecheck; live verify `http://localhost/en/dashboard/layers` (200), edit a layer end-to-end.
- [ ] Commit `feat(frontend): tabbed layer editor + searchable list (E2)`.

## Task 7: E2 acceptance

- [ ] Edit layer title/keywords/styles via UI → persisted (GET detail).
- [ ] Change declared SRS + reproject; compute bbox from data; boxes populate.
- [ ] Define SQL view; publish; WFS GetFeature returns filtered rows.
- [ ] Feature-type details render from live introspection.
- [ ] Commit `chore: E2 layer management complete`.

## Self-Review

Spec E2 coverage: list+search ✅(T6), Data/Publishing/CRS/FeatureType tabs ✅(T6), bbox compute+reproject ✅(T2/T6), metadata/keywords ✅(T1/T3/T6), SQL views + FID ✅(T4). Types `LayerPatch`/`FeatureTypePatch` consistent T3↔T5. No placeholders — code-bearing steps specify exact SQL/route shapes.
