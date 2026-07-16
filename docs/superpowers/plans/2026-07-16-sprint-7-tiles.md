# Sprint 7 — Tiles (Rust) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tiles service in Rust: gridsets (EPSG:3857 + 4326), MVT vector tiles from PostGIS `ST_AsMVT` (bbox pushdown), a content-addressed blob cache with Redis index + TTL, event-driven invalidation from catalog NATS events, seed/truncate, WMTS 1.0 (KVP + REST) / XYZ / TMS endpoints, GetCapabilities mirroring GeoWebCache, and raster tiles by proxying WMS GetMap for cache misses.

**Architecture:** `services/tiles` (Rust, axum, reuses `geo_core::health`). Gridset math (tile z/x/y → bbox) is pure functions. MVT tiles come straight from PostGIS `ST_AsMVT` (Martin-style, spec §10). Cache: content-addressed files under a shared volume keyed by SHA-256 of `layer/gridset/z/x/y/format`, with a Redis existence index (TTL + a per-layer generation counter). Catalog NATS `catalog.layer.updated`/`deleted` events bump the generation, invalidating cached tiles for that layer. Raster tiles fetch from `wms:8080/wms` GetMap on cache miss.

**Tech Stack:** Rust stable, axum 0.8, tokio, sqlx (postgres), redis 0.27 (async, tokio), async-nats 0.38, sha2 0.10, reqwest 0.12 (WMS proxy), serde_json.

## Global Constraints

- Health convention: `geo_core::health::router`; `GEOSON_HTTP_ADDR` default `:8080`; SIGTERM graceful shutdown.
- Rust CI gates: `cargo test --workspace`, `cargo fmt --all --check`, `cargo clippy --workspace -- -D warnings`. Integration tests read `GEOSON_TEST_DATABASE_URL`, skip when unset (`127.0.0.1:5433`); tests needing Redis read `GEOSON_TEST_REDIS_URL` (compose exposes Redis on `127.0.0.1:6380`), skip when unset.
- Shared catalog Postgres; layer metadata resolved the same way as WMS/WFS (`resources`/`layers`/`stores`, `host=self`).
- Cache dir env `GEOSON_TILE_CACHE_DIR` (default `/var/cache/geoson/tiles`); compose mounts the `tilecache` named volume there.
- Gateway proxies `/geoserver/gwc/service/wmts` (WMTS KVP), `/geoserver/gwc/service/tms` (TMS), and XYZ `/tiles/{layer}/{z}/{x}/{y}.{ext}` → `tiles:8080`. WMTS service dispatch key is `WMTS`.
- Gridsets: `EPSG:3857` (Web Mercator, 256px, 0..22, origin top-left -20037508.34…20037508.34) and `EPSG:4326` (2 tiles at z0, 256px). Custom gridsets registered in a table (Task 3).
- Commit after every task, Conventional Commits.

## File Structure

```
services/tiles/
  Cargo.toml  src/main.rs  src/lib.rs
  src/grid.rs        # gridset defs + tile->bbox math
  src/meta.rs        # catalog layer lookup (sqlx) — copy of wms::meta shape
  src/cache.rs       # content-addressed blob store + redis index
  src/mvt.rs         # ST_AsMVT vector tile query
  src/raster.rs      # WMS GetMap proxy for raster tiles
  src/events.rs      # NATS subscriber -> cache generation bump
  src/wmts.rs        # WMTS KVP + REST, XYZ, TMS handlers + GetCapabilities
  tests/tiles_it.rs  # integration (DB [+ redis])
```

---

### Task 1: Service scaffold

**Files:**
- Create: `services/tiles/Cargo.toml`, `services/tiles/src/main.rs`, `services/tiles/src/lib.rs`, `services/tiles/Dockerfile`
- Modify: `go.work`? no (Rust). `Cargo.toml` workspace members, `deploy/compose/docker-compose.yml` (tiles service + redis port 6380), `.github/workflows/ci.yml` (docker build)

**Interfaces:**
- Produces: crate `tiles` with `pub fn app(state: AppState) -> axum::Router` (health + endpoints added later) and `pub struct AppState { pub pool: Option<sqlx::PgPool>, pub redis: Option<redis::aio::ConnectionManager>, pub cache_dir: String, pub wms_url: String }`. Binary in `main.rs`.

- [ ] **Step 1: Cargo.toml + register in workspace**

`services/tiles/Cargo.toml`:

```toml
[package]
name = "tiles"
version = "0.1.0"
edition = "2021"
license = "Apache-2.0"

[dependencies]
geo-core = { path = "../../libs/geo-core" }
axum = { workspace = true }
tokio = { workspace = true }
sqlx = { version = "0.8", default-features = false, features = ["runtime-tokio", "postgres"] }
redis = { version = "0.27", features = ["tokio-comp", "connection-manager"] }
async-nats = "0.38"
sha2 = "0.10"
reqwest = { version = "0.12", default-features = false, features = ["rustls-tls"] }
serde_json = { workspace = true }
futures = { workspace = true }

[dev-dependencies]
tower = { version = "0.5", features = ["util"] }
http-body-util = "0.1"
```

Add `"services/tiles"` to root `Cargo.toml` `members`.

- [ ] **Step 2: Failing smoke test** — in `src/lib.rs` `#[cfg(test)]`:

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use tower::ServiceExt;

    #[tokio::test]
    async fn healthz() {
        let app = app(AppState::default());
        let res = app
            .oneshot(Request::get("/healthz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::OK);
    }
}
```

- [ ] **Step 3: Run** `cargo test -p tiles` → FAIL
- [ ] **Step 4: Implement lib.rs**

```rust
//! Geoson tiles service (WMTS/XYZ/TMS, MVT + raster cache).

pub mod grid;

use axum::Router;
use std::collections::HashMap;

#[derive(Default, Clone)]
pub struct AppState {
    pub pool: Option<sqlx::PgPool>,
    pub redis: Option<redis::aio::ConnectionManager>,
    pub cache_dir: String,
    pub wms_url: String,
}

pub fn app(_state: AppState) -> Router {
    // Endpoints mounted in later tasks; health always present.
    Router::new().merge(geo_core::health::router(HashMap::new()))
}
```

`main.rs`: read `GEOSON_DATABASE_URL`, `GEOSON_REDIS_URL`, `GEOSON_TILE_CACHE_DIR` (default `/var/cache/geoson/tiles`), `GEOSON_WMS_URL` (default `http://wms:8080`), `GEOSON_NATS_URL`; build AppState; `axum::serve` with SIGTERM shutdown (copy wms main.rs shape). Create `grid.rs` empty stub (`// Task 2`) so lib compiles, or omit `pub mod grid;` until Task 2. Keep `pub mod grid;` and stub file.

- [ ] **Step 5: Dockerfile** (pure Rust alpine — no GDAL needed here):

```dockerfile
# Build context = repo root: docker build -f services/tiles/Dockerfile .
FROM rust:1-alpine AS build
RUN apk add --no-cache musl-dev
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY libs/ libs/
COPY services/tiles/ services/tiles/
RUN cargo build --release -p tiles

FROM alpine:3.21
RUN apk add --no-cache curl && adduser -D -u 10001 geoson
USER geoson
COPY --from=build /src/target/release/tiles /usr/local/bin/tiles
ENV GEOSON_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["tiles"]
```

Note: the wms Dockerfile copies only its own crate + libs; but tiles depends on geo-core only (in libs/). Copying `libs/` and `services/tiles/` plus root Cargo.toml/lock works because the workspace root Cargo.toml lists all members — trim members via a build that only needs tiles: use `cargo build -p tiles` which resolves only tiles+geo-core. The other members aren't copied, so their paths break workspace resolution. Fix: copy a minimal root manifest. Simplest: set `ENV CARGO_NET_GIT_FETCH_WITH_CLI=false` and build with `--manifest-path services/tiles/Cargo.toml` after copying only geo-core. Do that:

```dockerfile
RUN cargo build --release --manifest-path services/tiles/Cargo.toml
```

But that still reads the workspace root. Cleanest: copy the whole `services/` and `libs/` and root manifest, accept the larger context (matches wms which copies its crate). Since only tiles+geo-core compile with `-p tiles` and cargo needs all member manifests present, copy every service's `Cargo.toml` (not sources). Pragmatic: copy `services/` fully (all crates) — build is `-p tiles`. Use:

```dockerfile
COPY libs/ libs/
COPY services/ services/
RUN cargo build --release -p tiles
```

- [ ] **Step 6: Compose + CI + Redis test port**

Compose: add `tiles` service (env DATABASE_URL, REDIS_URL redis:6379, NATS, WMS_URL http://wms:8080, TILE_CACHE_DIR /var/cache/geoson/tiles) with `volumes: - tilecache:/var/cache/geoson/tiles`, depends_on postgres+redis+nats healthy. Add `redis` port `127.0.0.1:6380:6379`. Gateway env: `GEOSON_TILES_URL: http://tiles:8080` (already set from Sprint 3). CI docker-build: `docker build -f services/tiles/Dockerfile .`.

- [ ] **Step 7: Run** `cargo test -p tiles` → PASS; `docker compose config -q`. **Commit** `git commit -m "feat(tiles): rust service scaffold"`

---

### Task 2: Gridsets + tile math

**Files:**
- Create: `services/tiles/src/grid.rs` (replace stub)

**Interfaces:**
- Produces:

```rust
pub struct Gridset { pub name: String, pub tile_size: u32, pub max_zoom: u8,
                     pub top_left: (f64, f64), pub world: f64, pub srid: i32 }
pub fn web_mercator() -> Gridset;   // EPSG:3857
pub fn plate_carree() -> Gridset;   // EPSG:4326 (2 cols at z0)
pub fn by_name(name: &str) -> Option<Gridset>;
/// tile_bbox returns [minx,miny,maxx,maxy] in the gridset SRS for z/x/y.
pub fn tile_bbox(g: &Gridset, z: u8, x: u32, y: u32) -> [f64; 4];
/// tiles_per_axis returns the number of tiles along one axis at zoom z.
pub fn tiles_per_axis(g: &Gridset, z: u8) -> u32;
```

Web Mercator: world extent ±20037508.342789244; z tiles = 2^z per axis; tile span = world*2 / 2^z; origin top-left (−W, +W); `minx = -W + x*span; maxx = minx + span; maxy = W - y*span; miny = maxy - span`. Plate carrée (EPSG:4326): 2 columns × 1 row at z0, extent lon −180..180, lat −90..90; at z, cols = 2^(z+1), rows = 2^z; span_x = 360/2^(z+1)... i.e. tile is square in degrees = 180/2^z? Use standard GWC EPSG:4326: at z0 two 256px tiles covering −180..0 and 0..180 for lon, −90..90 lat is one tile height 180°. Simplify: tile degree span = 180 / 2^z; cols = 2^(z+1); rows = 2^z; `minx = -180 + x*span; maxy = 90 - y*span`.

- [ ] **Step 1: Failing tests**

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn web_mercator_z0_is_world() {
        let g = web_mercator();
        let b = tile_bbox(&g, 0, 0, 0);
        assert!((b[0] + 20037508.342789244).abs() < 1e-3);
        assert!((b[3] - 20037508.342789244).abs() < 1e-3);
        assert_eq!(tiles_per_axis(&g, 0), 1);
        assert_eq!(tiles_per_axis(&g, 3), 8);
    }

    #[test]
    fn web_mercator_z1_quadrants() {
        let g = web_mercator();
        let tl = tile_bbox(&g, 1, 0, 0); // top-left quadrant
        assert!(tl[0] < 0.0 && tl[3] > 0.0);
        let br = tile_bbox(&g, 1, 1, 1); // bottom-right
        assert!(br[0] >= 0.0 && br[1] < 0.0);
    }

    #[test]
    fn plate_carree_z0() {
        let g = plate_carree();
        let b = tile_bbox(&g, 0, 0, 0);
        assert!((b[0] + 180.0).abs() < 1e-6);
        assert!((b[3] - 90.0).abs() < 1e-6);
        // two columns at z0
        let b1 = tile_bbox(&g, 0, 1, 0);
        assert!((b1[0] - 0.0).abs() < 1e-6);
    }

    #[test]
    fn lookup() {
        assert!(by_name("EPSG:3857").is_some());
        assert!(by_name("EPSG:4326").is_some());
        assert!(by_name("nope").is_none());
    }
}
```

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement grid.rs** per formulas above. **Step 4: Run** → PASS. **Commit** `git commit -m "feat(tiles): gridsets and tile bbox math"`

---

### Task 3: Layer metadata + MVT tile query

**Files:**
- Create: `services/tiles/src/meta.rs`, `services/tiles/src/mvt.rs`
- Modify: `services/tiles/src/lib.rs` (`pub mod meta; pub mod mvt;`)

**Interfaces:**
- Produces (`meta`): `pub struct LayerMeta { pub workspace, name, table, geom_col, srs: String }`; `pub async fn resolve(pool, ws, name) -> Result<LayerMeta, String>` (same query as wms::meta minus columns/style).
- Produces (`mvt`):

```rust
/// render_mvt returns the Mapbox Vector Tile bytes for a layer at z/x/y.
/// Empty tile (no features) returns an empty Vec (HTTP 204 upstream).
pub async fn render_mvt(pool: &sqlx::PgPool, layer: &crate::meta::LayerMeta,
    grid: &crate::grid::Gridset, z: u8, x: u32, y: u32) -> Result<Vec<u8>, String>;
```

SQL (parameterized bbox, validated idents):
```sql
WITH bounds AS (SELECT ST_TileEnvelope($z,$x,$y) AS geom)  -- 3857 only; for 4326 use ST_MakeEnvelope
SELECT ST_AsMVT(t, 'LAYER') FROM (
  SELECT <cols>, ST_AsMVTGeom(ST_Transform("geom", GRID_SRID),
    (SELECT geom FROM bounds), 4096, 64, true) AS geom
  FROM "table"
  WHERE "geom" && ST_Transform((SELECT geom FROM bounds), TABLE_SRID)
) t WHERE t.geom IS NOT NULL
```
For v1 use `ST_MakeEnvelope(minx,miny,maxx,maxy, grid_srid)` from `grid::tile_bbox` (works for any gridset). Layer name in MVT = layer.name.

- [ ] **Step 1: Failing integration test** — `tests/tiles_it.rs`:

```rust
use sqlx::postgres::PgPoolOptions;

async fn pool() -> Option<sqlx::PgPool> {
    let dsn = std::env::var("GEOSON_TEST_DATABASE_URL").ok()?;
    PgPoolOptions::new().connect(&dsn).await.ok()
}

static SEED: tokio::sync::Mutex<bool> = tokio::sync::Mutex::const_new(false);
async fn seed(pool: &sqlx::PgPool) {
    let mut d = SEED.lock().await;
    if *d { return; }
    for sql in [
        "CREATE EXTENSION IF NOT EXISTS postgis",
        "DELETE FROM layers WHERE workspace='tiletest'",
        "DELETE FROM resources WHERE workspace='tiletest'",
        "DELETE FROM stores WHERE workspace='tiletest'",
        "DELETE FROM workspaces WHERE name='tiletest'",
        "INSERT INTO workspaces(name) VALUES('tiletest')",
        "INSERT INTO stores(workspace,name,kind,type,enabled,connection) VALUES('tiletest','local','datastore','PostGIS',true,'{\"host\":\"self\"}'::jsonb)",
        "DROP TABLE IF EXISTS tile_pts",
        "CREATE TABLE tile_pts (id serial primary key, name text, geom geometry(Point,4326))",
        // a point near 0,0 lon/lat -> mercator origin area, visible at z0
        "INSERT INTO tile_pts(name,geom) VALUES ('c', ST_SetSRID(ST_MakePoint(0.1,0.1),4326))",
        "INSERT INTO resources(workspace,store,name,kind,native_name,srs,enabled) VALUES('tiletest','local','tile_pts','featuretype','tile_pts','EPSG:4326',true)",
        "INSERT INTO layers(workspace,name,type,resource_name,default_style,enabled) VALUES('tiletest','tile_pts','VECTOR','tile_pts','point',true)",
    ] { sqlx::query(sql).execute(pool).await.unwrap(); }
    *d = true;
}

#[tokio::test]
async fn mvt_z0_has_bytes() {
    let Some(pool) = pool().await else { return };
    seed(&pool).await;
    let m = tiles::meta::resolve(&pool, "tiletest", "tile_pts").await.unwrap();
    let g = tiles::grid::web_mercator();
    let bytes = tiles::mvt::render_mvt(&pool, &m, &g, 0, 0, 0).await.unwrap();
    assert!(!bytes.is_empty(), "z0 world tile should contain the point");
}

#[tokio::test]
async fn mvt_empty_tile() {
    let Some(pool) = pool().await else { return };
    seed(&pool).await;
    let m = tiles::meta::resolve(&pool, "tiletest", "tile_pts").await.unwrap();
    let g = tiles::grid::web_mercator();
    // a far tile (z8, top-left corner) has no point
    let bytes = tiles::mvt::render_mvt(&pool, &m, &g, 8, 0, 0).await.unwrap();
    assert!(bytes.is_empty(), "far tile should be empty");
}
```

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement meta.rs + mvt.rs.** ST_AsMVT returns bytea; read as `Vec<u8>`; if null/empty → return empty Vec. Grid SRID from gridset (3857 or 4326); transform table geom to grid srid inside ST_AsMVTGeom. **Step 4: Run** → PASS. **Commit** `git commit -m "feat(tiles): layer metadata and st_asmvt vector tiles"`

---

### Task 4: Content-addressed cache + Redis index

**Files:**
- Create: `services/tiles/src/cache.rs`
- Modify: `services/tiles/src/lib.rs` (`pub mod cache;`)

**Interfaces:**
- Produces:

```rust
pub struct Cache { dir: String, redis: Option<redis::aio::ConnectionManager>, ttl_secs: u64 }
impl Cache {
    pub fn new(dir: String, redis: Option<redis::aio::ConnectionManager>) -> Cache;
    /// key builds the content-address for a tile including the layer generation.
    pub async fn key(&mut self, layer: &str, gridset: &str, z: u8, x: u32, y: u32, fmt: &str) -> String;
    pub async fn get(&mut self, key: &str) -> Option<Vec<u8>>;
    pub async fn put(&mut self, key: &str, bytes: &[u8]) -> Result<(), String>;
    /// bump increments the generation counter for a layer (invalidates its tiles).
    pub async fn bump_generation(&mut self, layer: &str);
    /// generation returns the current layer generation (0 if none / no redis).
    pub async fn generation(&mut self, layer: &str) -> u64;
}
```

Blob path: `{dir}/{key[0..2]}/{key[2..4]}/{key}.{fmt}` where key = hex SHA-256 of `layer/gridset/z/x/y/fmt/gen`. Redis: existence marker `tile:{key}` with TTL (default 86400) set on put; generation `tilegen:{layer}` INCR'd on bump. When redis is None, cache is filesystem-only (no TTL, no invalidation index — still works, tests can run without redis). `key()` embeds generation so a bump changes the address → old blobs become unreachable (garbage-collected later; out of scope v1).

- [ ] **Step 1: Failing test** (filesystem-only path works without redis) — in `cache.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn roundtrip_fs_only() {
        let dir = std::env::temp_dir().join(format!("geoson-tiles-{}", std::process::id()));
        let mut c = Cache::new(dir.to_string_lossy().into(), None);
        let k = c.key("ws:layer", "EPSG:3857", 3, 1, 2, "pbf").await;
        assert!(c.get(&k).await.is_none());
        c.put(&k, b"tiledata").await.unwrap();
        assert_eq!(c.get(&k).await.unwrap(), b"tiledata");
        let _ = std::fs::remove_dir_all(dir);
    }

    #[tokio::test]
    async fn generation_changes_key() {
        // without redis generation() is 0; key stable
        let dir = std::env::temp_dir().join(format!("geoson-tiles-g-{}", std::process::id()));
        let mut c = Cache::new(dir.to_string_lossy().into(), None);
        let k1 = c.key("l", "g", 0, 0, 0, "pbf").await;
        let k2 = c.key("l", "g", 0, 0, 0, "pbf").await;
        assert_eq!(k1, k2);
        let _ = std::fs::remove_dir_all(dir);
    }

    #[tokio::test]
    async fn redis_generation_bumps() {
        let Ok(url) = std::env::var("GEOSON_TEST_REDIS_URL") else { return };
        let client = redis::Client::open(url).unwrap();
        let cm = client.get_connection_manager().await.unwrap();
        let dir = std::env::temp_dir().join(format!("geoson-tiles-r-{}", std::process::id()));
        let mut c = Cache::new(dir.to_string_lossy().into(), Some(cm));
        let k1 = c.key("bumplayer", "g", 0, 0, 0, "pbf").await;
        c.bump_generation("bumplayer").await;
        let k2 = c.key("bumplayer", "g", 0, 0, 0, "pbf").await;
        assert_ne!(k1, k2, "generation bump must change the tile key");
        let _ = std::fs::remove_dir_all(dir);
    }
}
```

- [ ] **Step 2: Run** `cargo test -p tiles cache::` → FAIL. **Step 3: Implement cache.rs** (sha2 hex; tokio::fs for blobs; redis GET/SETEX/INCR). **Step 4: Run** (with and without `GEOSON_TEST_REDIS_URL`) → PASS. **Commit** `git commit -m "feat(tiles): content-addressed blob cache with redis generation index"`

---

### Task 5: Raster tiles via WMS proxy

**Files:**
- Create: `services/tiles/src/raster.rs`
- Modify: `services/tiles/src/lib.rs` (`pub mod raster;`)

**Interfaces:**
- Produces:

```rust
/// fetch_raster_tile requests a PNG tile from the WMS GetMap endpoint for the
/// tile's bbox. Returns (bytes, content_type).
pub async fn fetch_raster_tile(wms_url: &str, layer: &str, grid: &crate::grid::Gridset,
    z: u8, x: u32, y: u32, fmt: &str) -> Result<(Vec<u8>, String), String>;
```

Build `{wms_url}/wms?service=WMS&version=1.3.0&request=GetMap&layers={layer}&crs=EPSG:{srid}&bbox={axis-ordered}&width={tile}&height={tile}&format=image/{png|jpeg}&transparent=true`. For 1.3.0 + EPSG:4326 the WMS expects lat/lon bbox order — emit miny,minx,maxy,maxx; for 3857 emit minx,miny,maxx,maxy. Use reqwest; on non-2xx return Err.

- [ ] **Step 1: Failing test** (unit with a mock WMS via a oneshot server) — in `raster.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn proxies_getmap() {
        // spin a tiny hyper/axum server that returns a fake PNG for /wms
        use axum::routing::get;
        let app = axum::Router::new().route("/wms", get(|| async {
            ([(axum::http::header::CONTENT_TYPE, "image/png")], vec![0x89u8, b'P', b'N', b'G'])
        }));
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move { axum::serve(listener, app).await.unwrap(); });
        let g = crate::grid::web_mercator();
        let (bytes, ct) = fetch_raster_tile(&format!("http://{addr}"), "ws:layer", &g, 0, 0, 0, "png")
            .await.unwrap();
        assert!(ct.contains("png"));
        assert_eq!(&bytes[..4], &[0x89, b'P', b'N', b'G']);
    }
}
```

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement raster.rs.** **Step 4: Run** → PASS. **Commit** `git commit -m "feat(tiles): raster tiles via wms getmap proxy"`

---

### Task 6: WMTS / XYZ / TMS handlers + GetCapabilities + cache wiring

**Files:**
- Create: `services/tiles/src/wmts.rs`
- Modify: `services/tiles/src/lib.rs` (mount routes in `app`)

**Interfaces:**
- Routes (all on the tiles app; gateway rewrites `/geoserver/gwc/...` to these):
  - `GET /wmts` — WMTS KVP: dispatch REQUEST=GetCapabilities | GetTile (LAYER, TILEMATRIXSET→gridset, TILEMATRIX=z, TILEROW=y, TILECOL=x, FORMAT). Vector `application/x-protobuf`/`application/vnd.mapbox-vector-tile` → MVT; image/* → raster.
  - `GET /wmts/{layer}/{tms}/{z}/{y}/{x}.{ext}` — WMTS RESTful.
  - `GET /tiles/{layer}/{z}/{x}/{y}.{ext}` — XYZ (ext pbf/mvt → vector; png/jpg → raster). y is XYZ (top-left origin).
  - `GET /gwc/service/tms/1.0.0/{layer}/{z}/{x}/{y}.{ext}` — TMS (y flipped vs XYZ).
- Flow per tile: resolve gridset + layer; cache.key; cache.get → hit returns bytes; miss → MVT (from Postgres) or raster (WMS proxy) → cache.put → return. Vector content-type `application/vnd.mapbox-vector-tile`; empty MVT → HTTP 204.
- GetCapabilities: WMTS 1.0 XML mirroring GeoWebCache — Contents with a Layer per `meta::list_layers`, TileMatrixSet EPSG:3857 + EPSG:4326, formats.

- [ ] **Step 1: Failing integration tests** — `tests/tiles_it.rs`:

```rust
use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use tower::ServiceExt;

async fn test_app() -> Option<axum::Router> {
    let pool = pool().await?;
    seed(&pool).await;
    let dir = std::env::temp_dir().join(format!("geoson-tiles-app-{}", std::process::id()));
    Some(tiles::app(tiles::AppState {
        pool: Some(pool), redis: None,
        cache_dir: dir.to_string_lossy().into(),
        wms_url: "http://127.0.0.1:0".into(),
    }))
}

#[tokio::test]
async fn xyz_mvt_tile() {
    let Some(app) = test_app().await else { return };
    let res = app.oneshot(Request::get("/tiles/tiletest:tile_pts/0/0/0.pbf").body(Body::empty()).unwrap()).await.unwrap();
    assert_eq!(res.status(), StatusCode::OK);
    assert_eq!(res.headers()["content-type"], "application/vnd.mapbox-vector-tile");
    let body = res.into_body().collect().await.unwrap().to_bytes();
    assert!(!body.is_empty());
}

#[tokio::test]
async fn xyz_empty_tile_204() {
    let Some(app) = test_app().await else { return };
    let res = app.oneshot(Request::get("/tiles/tiletest:tile_pts/8/0/0.pbf").body(Body::empty()).unwrap()).await.unwrap();
    assert_eq!(res.status(), StatusCode::NO_CONTENT);
}

#[tokio::test]
async fn wmts_getcapabilities() {
    let Some(app) = test_app().await else { return };
    let res = app.oneshot(Request::get("/wmts?service=WMTS&request=GetCapabilities").body(Body::empty()).unwrap()).await.unwrap();
    let body = res.into_body().collect().await.unwrap().to_bytes();
    let s = String::from_utf8_lossy(&body);
    assert!(s.contains("Capabilities"));
    assert!(s.contains("tiletest:tile_pts"));
    assert!(s.contains("EPSG:3857"));
}

#[tokio::test]
async fn wmts_kvp_gettile_mvt() {
    let Some(app) = test_app().await else { return };
    let uri = "/wmts?service=WMTS&request=GetTile&layer=tiletest:tile_pts&tilematrixset=EPSG:3857&tilematrix=0&tilerow=0&tilecol=0&format=application/vnd.mapbox-vector-tile";
    let res = app.oneshot(Request::get(uri).body(Body::empty()).unwrap()).await.unwrap();
    assert_eq!(res.status(), StatusCode::OK);
}

#[tokio::test]
async fn tms_flips_y() {
    let Some(app) = test_app().await else { return };
    // TMS y origin is bottom; z0 y0 == XYZ y0 for a single tile
    let res = app.oneshot(Request::get("/gwc/service/tms/1.0.0/tiletest:tile_pts/0/0/0.pbf").body(Body::empty()).unwrap()).await.unwrap();
    assert_eq!(res.status(), StatusCode::OK);
}
```

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement wmts.rs + mount in app()** (cache is per-request built from AppState clone; MVT vs raster by format; TMS y-flip = `(2^z - 1) - y`). **Step 4: Run** → PASS. **Commit** `git commit -m "feat(tiles): wmts/xyz/tms handlers, getcapabilities, cache-through"`

---

### Task 7: NATS cache invalidation

**Files:**
- Create: `services/tiles/src/events.rs`
- Modify: `services/tiles/src/main.rs` (spawn subscriber)

**Interfaces:**
- Produces:

```rust
/// subscribe_invalidations connects to NATS and bumps the tile cache generation
/// on catalog.layer.* events. Runs until the process exits.
pub async fn subscribe_invalidations(nats_url: &str,
    redis: redis::aio::ConnectionManager) -> Result<(), String>;
```

Subscribe subject `catalog.layer.*` and `catalog.featuretype.*`; payload JSON `{"name","workspace"}`; bump generation for `{workspace}:{name}` (INCR `tilegen:{ws}:{name}`).

- [ ] **Step 1: Failing test** (unit — bump via a fake published message using an in-process channel is heavy; instead test the handler function directly):

```rust
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn layer_key_from_event() {
        let payload = br#"{"name":"roads","workspace":"topp"}"#;
        assert_eq!(layer_key(payload), Some("topp:roads".to_string()));
        assert_eq!(layer_key(b"not json"), None);
    }
}
```

(`layer_key(payload: &[u8]) -> Option<String>` is the extractable pure part; `subscribe_invalidations` calls it per message then `Cache::bump_generation`.)

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement events.rs** (`layer_key` + async subscribe loop using async-nats; on each message bump via a `Cache`). Wire in main.rs: `tokio::spawn(subscribe_invalidations(...))` when NATS+Redis present. **Step 4: Run** `cargo test -p tiles events::` → PASS. **Commit** `git commit -m "feat(tiles): nats-driven cache invalidation"`

---

### Task 8: Seed / truncate endpoints

**Files:**
- Modify: `services/tiles/src/wmts.rs` (add admin routes), `services/tiles/src/lib.rs`

**Interfaces:**
- Routes:
  - `POST /api/v1/tiles/seed` JSON `{"layer":"ws:name","gridset":"EPSG:3857","zoomStart":0,"zoomStop":2,"format":"pbf"}` → renders + caches all tiles in range, returns `{"seeded":N}`.
  - `POST /api/v1/tiles/truncate` JSON `{"layer":"ws:name"}` → bumps generation (invalidates all tiles for the layer), returns 200.
- Seeding is synchronous+bounded in v1 (zoomStop ≤ 5 guard to avoid explosion); returns count.

- [ ] **Step 1: Failing integration test** — `tests/tiles_it.rs`:

```rust
#[tokio::test]
async fn seed_then_truncate() {
    let Some(app) = test_app().await else { return };
    let seed_req = Request::post("/api/v1/tiles/seed")
        .header("content-type", "application/json")
        .body(Body::from(r#"{"layer":"tiletest:tile_pts","gridset":"EPSG:3857","zoomStart":0,"zoomStop":1,"format":"pbf"}"#))
        .unwrap();
    let res = app.clone().oneshot(seed_req).await.unwrap();
    assert_eq!(res.status(), StatusCode::OK);
    let body = res.into_body().collect().await.unwrap().to_bytes();
    assert!(String::from_utf8_lossy(&body).contains("seeded"));

    let trunc = Request::post("/api/v1/tiles/truncate")
        .header("content-type", "application/json")
        .body(Body::from(r#"{"layer":"tiletest:tile_pts"}"#)).unwrap();
    let res = app.oneshot(trunc).await.unwrap();
    assert_eq!(res.status(), StatusCode::OK);
}
```

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement seed/truncate handlers** (loop z in [start,stop], x/y in 0..tiles_per_axis, render_mvt, cache.put; truncate = cache.bump_generation). **Step 4: Run** → PASS. **Commit** `git commit -m "feat(tiles): seed and truncate endpoints"`

---

### Task 9: E2E, docs, close out

**Files:**
- Create: `docs/services/tiles.md`
- Modify: `deploy/compose/docker-compose.yml` (gateway route for XYZ/gwc if needed), `docs/architecture.md`, `task.md`

- [ ] **Step 1: Gateway routing** — ensure the gateway forwards `/geoserver/gwc/service/wmts` and `/geoserver/gwc/service/tms` to tiles. The gateway dispatcher maps `gwc` endpoint → WMTS service (Sprint 3 `endpointService["gwc"]="WMTS"`) and `GEOSON_TILES_URL` → tiles. Add a Traefik route on the tiles service for XYZ `/tiles` too (bypasses OWS gateway): `traefik.http.routers.tiles.rule=PathPrefix(`/tiles`)`, priority 5. Verify the gwc WMTS path reaches tiles via gateway.

- [ ] **Step 2: Compose e2e**

```bash
cd deploy/compose && docker compose up -d --build tiles gateway wms
# reuse demo2:demo_poly (or seed a point layer)
docker compose exec -T postgres psql -U geoson -d geoson -c "CREATE TABLE IF NOT EXISTS tile_demo (id serial primary key, geom geometry(Point,4326)); INSERT INTO tile_demo(geom) SELECT ST_SetSRID(ST_MakePoint(0.1*i,0.1*i),4326) FROM generate_series(1,20) i ON CONFLICT DO NOTHING;"
curl -s -X POST -H 'Content-Type: application/xml' -d '<featureType><name>tile_demo</name><enabled>true</enabled></featureType>' http://localhost/geoserver/rest/workspaces/demo2/datastores/self/featuretypes
# XYZ vector tile direct to tiles service (via traefik /tiles route)
curl -s "http://localhost/tiles/demo2:tile_demo/0/0/0.pbf" -o /tmp/geoson.pbf; ls -l /tmp/geoson.pbf   # non-empty
# WMTS GetCapabilities through gateway gwc path
curl -s "http://localhost/geoserver/gwc/service/wmts?service=WMTS&request=GetCapabilities" | grep -o demo2:tile_demo | head -1
# WMTS GetTile
curl -s "http://localhost/geoserver/gwc/service/wmts?service=WMTS&request=GetTile&layer=demo2:tile_demo&tilematrixset=EPSG:3857&tilematrix=0&tilerow=0&tilecol=0&format=application/vnd.mapbox-vector-tile" -o /tmp/geoson-wmts.pbf; ls -l /tmp/geoson-wmts.pbf
```

Expected: XYZ + WMTS tiles are non-empty MVT; capabilities lists the layer.

- [ ] **Step 3: docs/services/tiles.md** — endpoints (WMTS KVP/REST, XYZ, TMS), gridsets, MVT (ST_AsMVT), cache (content-addressed + redis generation), invalidation (NATS), seed/truncate, raster-via-WMS.
- [ ] **Step 4: architecture.md tiles row → done; task.md Sprint 7 → [x]; plan boxes → [x]**
- [ ] **Step 5: Final verify + commit**

```bash
cargo fmt --all --check && cargo clippy --workspace -- -D warnings && cargo test --workspace
git add -A && git commit -m "feat(tiles): e2e, docs; complete sprint 7"
```
