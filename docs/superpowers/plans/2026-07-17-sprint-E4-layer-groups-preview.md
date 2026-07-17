# Sprint E4 — Layer Groups + Layer Preview + Output Formats — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** Layer-group CRUD (ordered layers+styles, modes, bounds compute, tile-cache tab), an enterprise Layer Preview page (embedded map + all output-format links), and vector output formats (KML/KMZ/CSV/shape-zip) + basic map-to-PDF print.

**Architecture:** Catalog layergroup endpoints (exist) get bounds-compute + ordered members over API v1. WFS gains output-format encoders (KML/KMZ/CSV/shape-zip). A small print endpoint composes a WMS GetMap into a PDF. Frontend adds Layer-Groups pages + a Preview page listing formats per layer.

**Tech Stack:** Go (catalog/wfs), Rust or Go print (use Go `gofpdf` + wms GetMap image), Next.js + MapLibre/OpenLayers embed.

## Global Constraints

Same as prior E-sprints. Output formats must match WFS `outputFormat` vendor values GeoServer uses (`application/json`, `KML`, `csv`, `SHAPE-ZIP`, `application/gml+xml`).

## File Structure

- `services/catalog/internal/rest/apiv1_layergroups.go` — CRUD + bounds compute over API v1.
- `services/catalog/internal/store/layergroups.go` — ordered members `[{layer,style}]`, mode, bounds, CRS.
- `services/wfs/internal/output/{kml,csv,shapezip}.go` — encoders + registry.
- `services/catalog/internal/print/pdf.go` + route `POST /api/v1/print` — map compose → PDF.
- `frontend/src/api/dashboard/groups/{types,api}.ts`, `.../preview/api.ts`.
- `frontend/src/components/dashboard/pages/LayerGroups.tsx`, `.../groups/GroupEditor.tsx`.
- `frontend/src/components/dashboard/pages/LayerPreviewList.tsx` + route `dashboard/preview`.
- `frontend/src/app/[locale]/(app)/dashboard/layer-groups/page.tsx`.

## Task 1: Layer-group model + ordered members (backend)

- [ ] Migration: `layergroups(name, workspace, title, abstract, mode, bounds jsonb, srs)` + `layergroup_members(group, workspace, position, layer, style)`.
- [ ] Store methods: `CreateLayerGroup`, `GetLayerGroup` (with ordered members), `UpdateLayerGroup`, `DeleteLayerGroup`, `ListLayerGroups`, `ComputeGroupBounds` (union of member bboxes via `ST_Extent`).
- [ ] **Test:** create group with 2 ordered members + styles → read back in order; compute bounds = union.
- [ ] Commit `feat(catalog): layer group model + ordered members (E4)`.

## Task 2: Layer-group API v1 (backend)

**Interfaces produced:** `GET/POST/PUT/DELETE /api/v1/layergroups[/{ws}]/{name}`, `POST /api/v1/layergroups/{ws}/{name}/bounds`. Body `{name,title,abstract,mode,srs,members:[{layer,style}]}`.

- [ ] Handlers delegate to store; bounds route calls `ComputeGroupBounds`.
- [ ] Ensure WMS can render a group (GetMap layer=group → renders members in order) — wire group resolution in wms layer lookup.
- [ ] Build + live verify: create group via curl; WMS GetMap of the group returns a PNG.
- [ ] Commit `feat(catalog): layer group api v1 + wms group render (E4)`.

## Task 3: WFS output formats (backend)

**Interfaces produced:** `output.Encoder` registry keyed by outputFormat; encoders `geojson`(exists), `kml`, `kmz`, `csv`, `shapezip`.

- [ ] **Test:** encode a small FeatureCollection → valid KML (has `<Placemark>`), CSV (header + rows), shape-zip (zip with .shp/.shx/.dbf/.prj).
- [ ] Implement encoders; register; WFS GetFeature dispatches on `outputFormat`/`OUTPUTFORMAT`.
- [ ] Build + live verify: `GetFeature&outputFormat=KML` returns KML; `SHAPE-ZIP` returns a zip.
- [ ] Commit `feat(wfs): KML/KMZ/CSV/shape-zip output formats (E4)`.

## Task 4: Print to PDF (backend)

- [ ] `POST /api/v1/print` body `{layers:[…], bbox, width, height, dpi, title}` → composes wms GetMap image, lays into PDF (title + map + scale bar) via `gofpdf`, returns `application/pdf`.
- [ ] **Test:** print request returns non-empty PDF bytes with `%PDF` header.
- [ ] Live verify curl returns a PDF.
- [ ] Commit `feat(catalog): map-to-PDF print (E4)`.

## Task 5: Layer-Groups UI (frontend)

- [ ] `LayerGroups.tsx` list (name/workspace/mode/layers count) + "Add group" → `GroupEditor` drawer.
- [ ] `GroupEditor`: Data tab (name/title/abstract/workspace/mode select single|opaque|named|EO/CRS + **Compute bounds** + min/max XY), member list with **drag reorder** (layer select + style select per row, add/remove), Publishing tab, Tile-Caching tab (defer detailed config to E5 — show enable toggle).
- [ ] i18n both dicts; route page.
- [ ] Typecheck; live verify `http://localhost/en/dashboard/layer-groups` (200); build a 2-layer ordered group and preview.
- [ ] Commit `feat(frontend): layer groups pages + editor (E4)`.

## Task 6: Layer Preview page (frontend)

- [ ] `LayerPreviewList.tsx`: table of all layers + group entries with **OpenLayers/MapLibre** preview link (opens `/map?layer=` or an embedded modal), plus format links (GeoJSON/GML/KML/KMZ/CSV/shape-zip) built from WFS URLs, and an "All formats" dropdown. Route `dashboard/preview`; add to nav (data group).
- [ ] i18n both dicts.
- [ ] Live verify page 200; format links download.
- [ ] Commit `feat(frontend): enterprise layer preview page (E4)`.

## Task 7: E4 acceptance

- [ ] Build multi-layer group with ordered styles + computed bounds; WMS renders it; preview shows it.
- [ ] Download a layer as GeoJSON/KML/shape-zip from Preview.
- [ ] Print a map to PDF.
- [ ] Commit `chore: E4 layer groups + preview complete`.

## Self-Review

Spec E4 coverage: group CRUD+modes+bounds+ordering ✅(T1/T2/T5), preview page + format links ✅(T6), output formats KML/KMZ/CSV/shape-zip ✅(T3), print/PDF ✅(T4). `output.Encoder` registry consistent T3. WMS group render wired T2 so preview works.
