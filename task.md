# Giti — Master Task Tracker

> **Resume point for every session.** When a new Claude session starts on this repo:
> 1. Read `docs/superpowers/specs/2026-07-16-giti-engine-design.md` (approved spec).
> 2. Find the first unchecked sprint below.
> 3. If that sprint has a plan file in `docs/superpowers/plans/`, execute it with the
>    `superpowers:executing-plans` (or subagent-driven) skill, task by task, checking boxes here and in the plan.
> 4. If the sprint has no plan file yet, write one first with `superpowers:writing-plans`, then execute.
> 5. Commit after every task. Update this file's checkboxes as part of those commits.

**Spec:** `docs/superpowers/specs/2026-07-16-giti-engine-design.md`
**Rule:** a sprint is checked only when its plan's every task is done, tests pass, and work is committed.

---

## Sprints

- [x] **Sprint 1 — Skeleton & infra**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-1-skeleton-infra.md`
  Monorepo layout, Go/Rust workspaces, healthz/readyz convention, Dockerfiles,
  compose (Traefik/Postgres+PostGIS/Redis/NATS/MinIO), scale smoke test, CI, docs scaffold.

- [x] **Sprint 2 — Catalog + stores**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-2-catalog-stores.md`
  Catalog data model (workspaces/stores/layers/styles), Postgres schema + migrations,
  GeoServer `/rest` compat API, connectors: PostGIS, Shapefile/GeoPackage/GeoJSON,
  GeoTIFF/COG, GeoParquet via DuckDB. NATS config-change events.

- [x] **Sprint 3 — Gateway**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-3-gateway.md`
  OWS KVP+XML parsing, service/version negotiation, GeoServer-format exception
  rendering (all versions), virtual services URLs, routing to services, metrics, rate limits.

- [x] **Sprint 4 — Auth + GeoFence**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-4-auth-geofence.md`
  Users/groups/roles, argon2id, JWT + basic-auth compat, GeoFence-style rule engine
  (ALLOW/DENY/LIMIT with CQL/attribute/geometry limits), Redis decision cache,
  `/rest/security` compat.

- [x] **Sprint 5 — WFS**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-5-wfs.md`
  WFS 1.0/1.1/2.0: GetCapabilities, DescribeFeatureType, GetFeature (paging/sort/hits),
  GetPropertyValue, WFS-T, stored queries. OGC Filter XML + CQL → SQL pushdown
  (PostGIS + DuckDB dialects). Streaming GML2/3.1/3.2, GeoJSON, CSV, SHP-zip encoders.
  Go side of filter golden corpus.

- [x] **Sprint 6 — WMS** (vector; raster deferred to S7/S12)
  Plan: `docs/superpowers/plans/2026-07-16-sprint-6-wms.md`
  WMS 1.1.1 + 1.3.0 (axis order), GetMap/GetFeatureInfo/GetLegendGraphic/DescribeLayer,
  render pipeline (tiny-skia + cosmic-text + rstar label collision), SLD 1.0/1.1,
  MapLibre style JSON, GeoCSS→SLD, raster via async-tiff COG + GDAL fallback,
  fast PNG (image-png fast mode)/JPEG/WebP, all vendor params. Rust side of filter corpus.

- [x] **Sprint 7 — Tiles**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-7-tiles.md`
  WMTS 1.0 KVP+REST, XYZ, TMS; GetCapabilities mirroring GeoWebCache. MVT from
  PostGIS `ST_AsMVT` + DuckDB/geozero. Metatiled raster cache via wms. Tile storage
  (volume/MinIO) + Redis index, event invalidation, seed/truncate jobs, gridset registry.

- [x] **Sprint 8 — WPS + convert**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-8-wps-convert.md`
  WPS 1.0 sync/async, NATS job queue, process set (buffer/clip/intersection/union/
  dissolve/simplify/reproject/centroid/convex hull/stats/heatmap) with PostGIS pushdown +
  i_overlay paths. Convert service: ingest pipeline, format conversions, GeoTIFF→COG,
  vector→GeoParquet, SSE progress.

- [x] **Sprint 9 — Frontend**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-9-frontend.md`
  Next.js 15 admin per spec §3.9: [locale] fa/en (RTL), dark/light themes, auth shell,
  MapLibre workspace, dashboard sections (overview/workspaces/stores/layers/groups/
  styles/tile-cache/security/wps/convert/settings), feature-first `src/api/`.

- [ ] **Sprint 10 — Compat, benchmarks, release**
  Plan: _not written yet_
  GeoServer golden diff harness (docker GeoServer vs Giti, XML canonical diff,
  SSIM image diff), k6 load benchmarks vs GeoServer + MapServer (published in docs),
  scaling/ops documentation, Swarm stack, README/LICENSE polish, open-source release.

## Backlog — full GeoServer parity (after core S1–S10)

See `docs/feature-parity.md` for the complete GeoServer feature matrix.

## Enterprise parity program (E1–E10)

Approved design: `docs/superpowers/specs/2026-07-17-giti-enterprise-parity-design.md`.
Detailed plans: `docs/superpowers/plans/2026-07-17-sprint-E<n>-*.md`. UI-first,
modernized enterprise UX, connect-anywhere. Old S11–S14 + S4.1 fold in
(S11/S12→E8, S13→E8, S14→E7, S4.1→E6). Execute E1→E10 in order.

- [x] **E1 — Admin shell + connect-anywhere stores** — `plans/2026-07-17-sprint-E1-admin-shell-stores.md`
- [x] **E2 — Layer management** — `plans/2026-07-17-sprint-E2-layer-management.md`
- [ ] **E3 — Styles + SLD editor** — `plans/2026-07-17-sprint-E3-styles-sld-editor.md`
- [ ] **E4 — Layer groups + preview + output formats** — `plans/2026-07-17-sprint-E4-layer-groups-preview.md`
- [ ] **E5 — Tile caching (GWC parity)** — `plans/2026-07-17-sprint-E5-tile-caching.md`
- [ ] **E6 — Security (users/roles/rules)** — `plans/2026-07-17-sprint-E6-security.md`
- [ ] **E7 — Importer + monitor + settings** — `plans/2026-07-17-sprint-E7-importer-monitor-settings.md`
- [ ] **E8 — New connectors (MS SQL/CSV/KML/cascade/raster)** — `plans/2026-07-17-sprint-E8-connectors.md`
- [ ] **E9 — CSW + WCS + OGC-API + REST parity** — `plans/2026-07-17-sprint-E9-csw-wcs-rest.md`
- [ ] **E10 — Compat + benchmarks + release** — `plans/2026-07-17-sprint-E10-compat-bench-release.md`
