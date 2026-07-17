# Sprint E3 — Styles + SLD Editor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** GeoServer-grade style management: styles list at 365-scale, a multi-language style editor (SLD/CSS/YSLD/MBStyle) with validate, generate-default, copy, upload, legend, and live layer preview.

**Architecture:** Catalog gains `/styles/validate` (parse via a shared style-parse lib), `/styles/generate` (geometry → default SLD), and full style CRUD over API v1; the wms service already parses SLD (S6) and GeoCSS→SLD — expose its parser through a small internal endpoint the catalog calls, or link the parse crate. Frontend adds a CodeMirror editor with syntax modes + a preview pane reusing `LayerPreview`.

**Tech Stack:** Go (catalog), Rust (wms parse), Next.js + `@uiw/react-codemirror` + `@codemirror/lang-xml`/`lang-css`.

## Global Constraints

Same as E1/E2. SLD validation must reuse the WMS SLD parser (`services/wms/src/...`) so editor-validate == render-time parse (no drift). Catalog→wms validate call over `GITI_WMS_URL`.

## File Structure

- `services/wms/src/style_validate.rs` + route `POST /style/validate` (internal) — parse SLD/CSS/YSLD/MBStyle, return `{ok, errors:[{line,message}]}`.
- `services/catalog/internal/rest/apiv1_styles.go` — validate (proxy to wms), generate, CRUD.
- `services/catalog/internal/style/generate.go` — default SLD templates per geometry.
- `frontend/src/api/dashboard/styles/{types,api}.ts` — client.
- `frontend/src/components/dashboard/styles/StyleEditor.tsx` — editor + tabs (Data/Publishing/Preview/Attributes).
- `frontend/src/components/dashboard/pages/Styles.tsx` — list (search/paginate/bulk-remove) replacing the stub.
- `frontend/src/app/[locale]/(app)/dashboard/styles/[name]/page.tsx` — editor route.

## Task 1: WMS style-validate endpoint (Rust)

- [ ] **Test (failing, Rust):** `parse_style("<StyledLayerDescriptor…>", "sld")` returns Ok; malformed returns Err with a line.
- [ ] Implement `style_validate.rs`: reuse existing SLD parser; add format dispatch (`sld`→existing, `css`→GeoCSS→SLD path from S6, `ysld`/`mbstyle`→convert-then-parse; if a converter is absent, return `{ok:false, errors:[{line:0,message:"format not yet supported"}]}`).
- [ ] Add axum route `POST /style/validate` body `{format, content}` → `{ok, errors}`.
- [ ] `cargo test -p wms style_validate` → PASS; `cargo build -p wms`.
- [ ] Commit `feat(wms): style validate endpoint (E3)`.

## Task 2: Catalog styles API — validate/generate/CRUD (Go)

**Interfaces produced:** `POST /api/v1/styles/validate` (proxy to wms), `POST /api/v1/styles/generate` body `{geomType, color?}` → `{sld}`, `GET/POST/PUT/DELETE /api/v1/styles` (+ `{ws}` scoped), `GET /api/v1/styles/{name}` → `{name, format, content}`.

- [ ] **Test:** `generateDefault("POINT","#2FA7A1")` returns SLD containing `PointSymbolizer`.
- [ ] Implement `style/generate.go` templates (point/line/polygon/raster).
- [ ] `apiv1_styles.go`: validate handler POSTs to `GITI_WMS_URL + /style/validate`; CRUD delegates to existing store style methods (`GetStyle/ListStyles/CreateStyle/UpdateStyle`); return content body.
- [ ] Build + live verify: `curl -s -X POST http://localhost/api/v1/styles/validate -d '{"format":"sld","content":"<bad/>"}'` → `{ok:false,...}`; `.../generate -d '{"geomType":"POINT"}'` → SLD.
- [ ] Commit `feat(catalog): styles validate/generate/crud api (E3)`.

## Task 3: Styles API client (frontend)

- [ ] Types `Style{name,format,workspace?}`, `StyleContent{name,format,content}`, `ValidateResult{ok,errors:[{line,message}]}`. Functions `listStyles`, `getStyle`, `createStyle`, `updateStyle`, `deleteStyle`, `validateStyle`, `generateStyle`.
- [ ] Typecheck; commit `feat(frontend): styles api client (E3)`.

## Task 4: Styles list page (frontend)

- [ ] Replace `Styles.tsx` Placeholder: DataTable with search + pagination (handles 365 rows), workspace column, "Add style" → navigate to `/dashboard/styles/new`, bulk-remove (checkbox column + delete).
- [ ] i18n keys both dicts.
- [ ] Live verify `http://localhost/en/dashboard/styles` (200).
- [ ] Commit `feat(frontend): styles list page (E3)`.

## Task 5: Style editor (frontend)

**Files:** `StyleEditor.tsx`, route `styles/[name]/page.tsx`; add deps `@uiw/react-codemirror @codemirror/lang-xml @codemirror/lang-css`.

- [ ] Editor layout: left = CodeMirror (mode from format select SLD-XML/CSS/YSLD/MBStyle) + toolbar (Validate / Generate-default / Copy-from / Upload / Save); right = tabs **Preview** (LayerPreview with a chosen layer applying this style via `?style=`), **Attributes** (layer attributes), **Legend** (`<img src=GetLegendGraphic>`). Validate shows error markers (line + message list under editor). Generate opens a small geom-type+color prompt calling `generateStyle`.
- [ ] Wire preview: pass `style` name to `gitiMvtTiles`/WMS preview (WMS GetMap with `styles=<name>`); reuse `LayerPreview` with an optional `styleName` prop (add it).
- [ ] i18n keys both dicts.
- [ ] Typecheck; live verify create SLD → validate error surfaced → fix → save → preview updates.
- [ ] Commit `feat(frontend): multi-language style editor + preview (E3)`.

## Task 6: E3 acceptance

- [ ] Create/edit SLD with validation errors inline; fix; save.
- [ ] Generate default point/line/polygon style; assign as layer default (via E2 editor) → preview + legend reflect it.
- [ ] Author a CSS style and validate.
- [ ] Styles list searches/paginates at full scale; bulk remove works.
- [ ] Commit `chore: E3 styles + SLD editor complete`.

## Self-Review

Spec E3 coverage: list ✅(T4), editor+validate ✅(T1/T2/T5), generate/copy/upload ✅(T2/T5), legend+preview+attributes ✅(T5), multi-language SLD/CSS/YSLD/MBStyle ✅(T1/T5). Validate reuses render parser (no drift). Names `validateStyle`/`generateStyle` consistent T3↔T5.
