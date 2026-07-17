# Sprint E5 — Tile Caching (GeoWebCache parity) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** Full GeoWebCache-equivalent admin: Tile Layers, Caching Defaults, Gridsets CRUD, BlobStores CRUD (file/S3), Disk Quota, per-layer tile-cache config (metatiling/formats/gridsets/expire/parameter-filters), seed/truncate/reseed jobs with progress.

**Architecture:** The `tiles` (Rust) service already caches content-addressed blobs with a Redis generation counter + NATS invalidation (S7). E5 adds config stores (gridset registry, blobstore config, per-layer cache config, disk-quota policy) in catalog + tiles, a seed/truncate job runner (NATS queue + progress like WPS), and the admin UI.

**Tech Stack:** Rust (tiles: gridset/blobstore/quota/seed), Go (catalog config API), Next.js UI, Redis/NATS, MinIO (S3 blobstore).

## Global Constraints

Same as prior. Tile cache key stays `SHA-256(layer/grid/z/x/y/fmt/generation)`; gridsets EPSG:3857/4326 built-in + custom. Blobstore backends: `file` (volume) + `s3` (MinIO). Disk-quota LRU eviction on size cap.

## File Structure

- `services/catalog/internal/rest/apiv1_gwc.go` — gridsets/blobstores/quota/per-layer-cache config CRUD.
- `services/catalog/internal/store/gwc.go` — config tables.
- `services/tiles/src/gridset.rs` — registry (built-in + DB-loaded custom).
- `services/tiles/src/blobstore.rs` — file + s3 backends behind a trait.
- `services/tiles/src/quota.rs` — usage tracking + LRU eviction.
- `services/tiles/src/seed.rs` — seed/truncate/reseed job runner (NATS) + progress.
- `frontend/src/api/dashboard/tilecache/{types,api}.ts`.
- `frontend/src/components/dashboard/pages/TileCache.tsx` (replace stub) + subpages/tabs.

## Task 1: GWC config tables + catalog API (backend Go)

- [ ] Migration: `gridsets(name, srs, extent jsonb, resolutions numeric[], tile_size int)`, `blobstores(name, type, config jsonb, is_default bool)`, `layer_cache(layer, workspace, enabled bool, metatile_x int, metatile_y int, gutter int, formats text[], expire_server int, expire_client int, gridsets text[], param_filters jsonb, blobstore text)`, `disk_quota(policy text, max_bytes bigint)`.
- [ ] Store CRUD + `apiv1_gwc.go` routes: `GET/POST/PUT/DELETE /api/v1/gwc/gridsets`, `/gwc/blobstores`, `GET/PUT /api/v1/gwc/quota`, `GET/PUT /api/v1/gwc/layers/{ws}/{name}` (per-layer cache config), `GET /api/v1/gwc/layers` (tile layers list).
- [ ] **Test:** create custom gridset + blobstore; read back; set per-layer config.
- [ ] Build + live verify curl round-trips.
- [ ] Commit `feat(catalog): GWC config tables + api (E5)`.

## Task 2: Gridset registry (Rust tiles)

- [ ] `gridset.rs`: built-in EPSG:3857 (GoogleMapsCompatible) + EPSG:4326; load custom from catalog (`GET /api/v1/gwc/gridsets`) at startup + on NATS `catalog.gridset.*`. Tile addressing resolves against the named gridset.
- [ ] **Test (Rust):** resolution/tile-envelope for 3857 z0..z2 matches known values; a custom gridset resolves.
- [ ] `cargo build -p tiles`; live verify WMTS GetCapabilities lists gridsets.
- [ ] Commit `feat(tiles): gridset registry (built-in + custom) (E5)`.

## Task 3: BlobStores (file + S3) (Rust tiles)

- [ ] `blobstore.rs`: trait `BlobStore { get, put, delete, list_prefix }`; `FileBlobStore` (existing cache dir) + `S3BlobStore` (MinIO via `GITI_S3_*`). Select per-layer from `layer_cache.blobstore`, default from config.
- [ ] **Test:** put/get/delete round-trip on file backend; s3 backend behind a feature flag / integration gate.
- [ ] Build; live verify a cached tile lands in the selected blobstore.
- [ ] Commit `feat(tiles): file + S3 blobstores (E5)`.

## Task 4: Disk quota + eviction (Rust tiles)

- [ ] `quota.rs`: track cache size (Redis counter per blobstore), enforce `max_bytes` with LRU eviction (Redis ZSET by last-access); expose `GET /gwc/quota/usage`.
- [ ] **Test:** exceeding cap evicts oldest; usage reported.
- [ ] Build; live verify usage endpoint.
- [ ] Commit `feat(tiles): disk quota LRU eviction (E5)`.

## Task 5: Seed / truncate / reseed jobs (Rust tiles)

- [ ] `seed.rs`: job = `{layer, gridset, zoom_start, zoom_stop, bounds, op:seed|reseed|truncate, format}`; enqueue on NATS; worker iterates tiles (respect metatiling), writes/removes blobs, publishes progress (`tiles.seed.progress`), status queryable `GET /gwc/seed/{jobId}`.
- [ ] **Test:** seed a small bounds/zoom range → tiles present; truncate → removed; progress increments.
- [ ] Build; live verify a seed job populates tiles.
- [ ] Commit `feat(tiles): seed/truncate/reseed jobs + progress (E5)`.

## Task 6: Tile-cache API client + UI (frontend)

- [ ] Client for gridsets/blobstores/quota/layer-config/tile-layers/seed (types + functions).
- [ ] `TileCache.tsx` with sub-tabs: **Tile Layers** (list + cached status + seed button), **Caching Defaults**, **Gridsets** (CRUD table + editor), **BlobStores** (CRUD + default toggle), **Disk Quota** (policy + usage bar). Per-layer config surfaced as a **Tile Caching** tab inside the E2 Layer editor (add it there): create-cached, metatiling x/y, gutter, image formats (checkbox list incl. `application/vnd.mapbox-vector-tile`), expire server/client, gridset subset, **parameter filters** editor. Seed dialog: gridset + zoom range + bounds + op → progress bar (poll seed status).
- [ ] i18n both dicts; live verify each subpage 200; run a seed job from UI and watch progress.
- [ ] Commit `feat(frontend): tile caching admin (gridsets/blobstores/quota/seed) (E5)`.

## Task 7: E5 acceptance

- [ ] Define custom gridset + S3 blobstore via UI.
- [ ] Enable caching on a layer with metatiling + param filters; hit a cached tile.
- [ ] Run a seed job; watch progress; truncate.
- [ ] Disk-quota eviction observable when cap exceeded.
- [ ] Commit `chore: E5 tile caching complete`.

## Self-Review

Spec E5 coverage: Tile Layers/Defaults/Gridsets/BlobStores/DiskQuota ✅(T1–T4/T6), per-layer config incl. param filters ✅(T1/T6), seed/truncate/reseed + progress ✅(T5/T6), GWC REST parity via api ✅(T1). Cache key + Redis generation unchanged. Config shapes consistent catalog↔tiles (T1↔T2/T3).
