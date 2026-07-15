# Geoson — Master Task Tracker

> **Resume point for every session.** When a new Claude session starts on this repo:
> 1. Read `docs/superpowers/specs/2026-07-16-geoson-engine-design.md` (approved spec).
> 2. Find the first unchecked sprint below.
> 3. If that sprint has a plan file in `docs/superpowers/plans/`, execute it with the
>    `superpowers:executing-plans` (or subagent-driven) skill, task by task, checking boxes here and in the plan.
> 4. If the sprint has no plan file yet, write one first with `superpowers:writing-plans`, then execute.
> 5. Commit after every task. Update this file's checkboxes as part of those commits.

**Spec:** `docs/superpowers/specs/2026-07-16-geoson-engine-design.md`
**Rule:** a sprint is checked only when its plan's every task is done, tests pass, and work is committed.

---

## Sprints

- [x] **Sprint 1 — Skeleton & infra**
  Plan: `docs/superpowers/plans/2026-07-16-sprint-1-skeleton-infra.md`
  Monorepo layout, Go/Rust workspaces, healthz/readyz convention, Dockerfiles,
  compose (Traefik/Postgres+PostGIS/Redis/NATS/MinIO), scale smoke test, CI, docs scaffold.

- [ ] **Sprint 2 — Catalog + stores**
  Plan: _not written yet_
  Catalog data model (workspaces/stores/layers/styles), Postgres schema + migrations,
  GeoServer `/rest` compat API, connectors: PostGIS, Shapefile/GeoPackage/GeoJSON,
  GeoTIFF/COG, GeoParquet via DuckDB. NATS config-change events.

- [ ] **Sprint 3 — Gateway**
  Plan: _not written yet_
  OWS KVP+XML parsing, service/version negotiation, GeoServer-format exception
  rendering (all versions), virtual services URLs, routing to services, metrics, rate limits.

- [ ] **Sprint 4 — Auth + GeoFence**
  Plan: _not written yet_
  Users/groups/roles, argon2id, JWT + basic-auth compat, GeoFence-style rule engine
  (ALLOW/DENY/LIMIT with CQL/attribute/geometry limits), Redis decision cache,
  `/rest/security` compat.

- [ ] **Sprint 5 — WFS**
  Plan: _not written yet_
  WFS 1.0/1.1/2.0: GetCapabilities, DescribeFeatureType, GetFeature (paging/sort/hits),
  GetPropertyValue, WFS-T, stored queries. OGC Filter XML + CQL → SQL pushdown
  (PostGIS + DuckDB dialects). Streaming GML2/3.1/3.2, GeoJSON, CSV, SHP-zip encoders.
  Go side of filter golden corpus.

- [ ] **Sprint 6 — WMS**
  Plan: _not written yet_
  WMS 1.1.1 + 1.3.0 (axis order), GetMap/GetFeatureInfo/GetLegendGraphic/DescribeLayer,
  render pipeline (tiny-skia + cosmic-text + rstar label collision), SLD 1.0/1.1,
  MapLibre style JSON, GeoCSS→SLD, raster via async-tiff COG + GDAL fallback,
  fast PNG (image-png fast mode)/JPEG/WebP, all vendor params. Rust side of filter corpus.

- [ ] **Sprint 7 — Tiles**
  Plan: _not written yet_
  WMTS 1.0 KVP+REST, XYZ, TMS; GetCapabilities mirroring GeoWebCache. MVT from
  PostGIS `ST_AsMVT` + DuckDB/geozero. Metatiled raster cache via wms. Tile storage
  (volume/MinIO) + Redis index, event invalidation, seed/truncate jobs, gridset registry.

- [ ] **Sprint 8 — WPS + convert**
  Plan: _not written yet_
  WPS 1.0 sync/async, NATS job queue, process set (buffer/clip/intersection/union/
  dissolve/simplify/reproject/centroid/convex hull/stats/heatmap) with PostGIS pushdown +
  i_overlay paths. Convert service: ingest pipeline, format conversions, GeoTIFF→COG,
  vector→GeoParquet, SSE progress.

- [ ] **Sprint 9 — Frontend**
  Plan: _not written yet_
  Next.js 15 admin per spec §3.9: [locale] fa/en (RTL), dark/light themes, auth shell,
  MapLibre workspace, dashboard sections (overview/workspaces/stores/layers/groups/
  styles/tile-cache/security/wps/convert/settings), feature-first `src/api/`.

- [ ] **Sprint 10 — Compat, benchmarks, release**
  Plan: _not written yet_
  GeoServer golden diff harness (docker GeoServer vs Geoson, XML canonical diff,
  SSIM image diff), k6 load benchmarks vs GeoServer + MapServer (published in docs),
  scaling/ops documentation, Swarm stack, README/LICENSE polish, open-source release.
