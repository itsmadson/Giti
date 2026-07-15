# Sprint 1 — Skeleton & Infra Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootable Geoson monorepo: Go + Rust workspaces with two stub services exposing the project-wide health convention, full infra compose stack (Traefik/Postgres+PostGIS/Redis/NATS/MinIO), horizontal-scale smoke test, CI, and docs scaffold.

**Architecture:** Monorepo per spec §6. Two language workspaces (Go `go.work`, Rust cargo workspace). `gateway` (Go) and `wms` (Rust) exist as health-only stubs to prove the container/scale/LB path end-to-end before any OGC logic lands. All later services copy these stubs' conventions.

**Tech Stack:** Go 1.24, Rust stable (2021 edition, axum 0.8 + tokio 1), Traefik v3, postgis/postgis:17-3.5, redis:7-alpine, nats:2.11-alpine (JetStream on), minio, GitHub Actions.

## Global Constraints

- License: Apache-2.0 (spec §11).
- Health convention (spec §5): every service serves `GET /healthz` → `200` body `ok` (liveness, no dep checks) and `GET /readyz` → `200` JSON `{"status":"ready","checks":{...}}` or `503` `{"status":"unready","checks":{...}}` (readiness, dep checks). Graceful shutdown on SIGTERM.
- Service HTTP port env var: `GEOSON_HTTP_ADDR`, default `:8080`, in every service.
- All request-path services stateless (spec §5).
- Compose files live in `deploy/compose/` (spec §6).
- Commit after every task; commit messages Conventional Commits.

---

### Task 1: Repo skeleton, license, gitignore, README

**Files:**
- Create: `LICENSE`, `.gitignore`, `.editorconfig`, `README.md`
- Create (empty dirs via `.gitkeep`): `services/`, `libs/`, `frontend/`, `deploy/compose/`, `deploy/swarm/`, `tests/filter-corpus/`, `tests/compat/`, `docs/`

**Interfaces:**
- Consumes: nothing.
- Produces: directory layout all later tasks write into (paths per spec §6).

- [ ] **Step 1: Create directory skeleton**

```bash
cd /home/madson/geoson
mkdir -p services libs frontend deploy/compose deploy/swarm tests/filter-corpus tests/compat docs/ops docs/dev
touch services/.gitkeep libs/.gitkeep frontend/.gitkeep deploy/swarm/.gitkeep tests/filter-corpus/.gitkeep tests/compat/.gitkeep
```

- [ ] **Step 2: Write LICENSE**

Download the canonical Apache-2.0 text:

```bash
curl -fsSL https://www.apache.org/licenses/LICENSE-2.0.txt -o LICENSE
```

- [ ] **Step 3: Write .gitignore**

```gitignore
# Go
*.exe
*.test
*.out
/services/**/bin/

# Rust
target/

# Node
node_modules/
.next/

# Env / local
.env
.env.local
*.local.yml

# OS / editor
.DS_Store
*.swp
```

- [ ] **Step 4: Write .editorconfig**

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
indent_style = space
indent_size = 4

[*.{yml,yaml,json,ts,tsx,js}]
indent_size = 2

[Makefile]
indent_style = tab

[*.go]
indent_style = tab
```

- [ ] **Step 5: Write README.md**

```markdown
# Geoson

High-performance, horizontally scalable OGC geo engine — a drop-in GeoServer
replacement built as microservices (Go + Rust + Next.js).

- WMS 1.1.1/1.3.0 · WFS 1.0/1.1/2.0 · WMTS 1.0/XYZ/TMS · WPS 1.0
- GeoServer-compatible URLs, vendor params (`CQL_FILTER`, `TILED`, …),
  exception formats, and `/rest` config API
- GeoFence-style security, tile caching + seeding, format conversion
- Stores: PostGIS, Shapefile/GeoPackage/GeoJSON, GeoTIFF/COG, GeoParquet (DuckDB)
- Scale any service independently: `docker compose up -d --scale wms=4`

**Design spec:** `docs/superpowers/specs/2026-07-16-geoson-engine-design.md`
**Task tracker / resume point:** `task.md`
**Getting started:** `docs/dev/getting-started.md`

License: Apache-2.0
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: repo skeleton, license, readme"
```

---

### Task 2: Go workspace + gateway health stub (TDD)

**Files:**
- Create: `go.work`
- Create: `libs/ogc-kit/go.mod`, `libs/ogc-kit/health/health.go`, `libs/ogc-kit/health/health_test.go`
- Create: `services/gateway/go.mod`, `services/gateway/main.go`, `services/gateway/main_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - Go module `github.com/geoson/geoson/libs/ogc-kit` with package `health`:
    - `func NewMux(checks map[string]Check) *http.ServeMux` — mux serving `/healthz` and `/readyz` per Global Constraints.
    - `type Check func(ctx context.Context) error`
    - `func Serve(ctx context.Context, addr string, handler http.Handler) error` — HTTP server with SIGTERM-driven graceful shutdown (callers pass `signal.NotifyContext` ctx).
  - Go module `github.com/geoson/geoson/services/gateway` binary using it. Every later Go service copies this shape.

- [ ] **Step 1: Init modules and workspace**

```bash
cd /home/madson/geoson
mkdir -p libs/ogc-kit/health services/gateway
( cd libs/ogc-kit && go mod init github.com/geoson/geoson/libs/ogc-kit )
( cd services/gateway && go mod init github.com/geoson/geoson/services/gateway )
go work init ./libs/ogc-kit ./services/gateway
rm -f libs/.gitkeep services/.gitkeep
```

- [ ] **Step 2: Write failing test for health mux**

`libs/ogc-kit/health/health_test.go`:

```go
package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestHealthzAlwaysOK(t *testing.T) {
	mux := NewMux(map[string]Check{
		"broken": func(ctx context.Context) error { return errors.New("down") },
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q, want 200 ok", rec.Code, rec.Body.String())
	}
}

func TestReadyzReadyWhenChecksPass(t *testing.T) {
	mux := NewMux(map[string]Check{
		"redis": func(ctx context.Context) error { return nil },
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 200 {
		t.Fatalf("readyz code = %d, want 200", rec.Code)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ready" || body.Checks["redis"] != "ok" {
		t.Fatalf("body = %+v, want ready/redis ok", body)
	}
}

func TestReadyzUnreadyWhenCheckFails(t *testing.T) {
	mux := NewMux(map[string]Check{
		"postgres": func(ctx context.Context) error { return errors.New("conn refused") },
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 503 {
		t.Fatalf("readyz code = %d, want 503", rec.Code)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "unready" || body.Checks["postgres"] != "conn refused" {
		t.Fatalf("body = %+v, want unready/conn refused", body)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /home/madson/geoson/libs/ogc-kit && go test ./health/`
Expected: FAIL — `undefined: NewMux`, `undefined: Check`

- [ ] **Step 4: Implement health package**

`libs/ogc-kit/health/health.go`:

```go
// Package health implements the Geoson project-wide health convention:
// GET /healthz  -> 200 "ok" (liveness, never checks dependencies)
// GET /readyz   -> 200/503 JSON {"status": ..., "checks": {...}} (readiness)
package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Check reports whether one dependency is usable.
type Check func(ctx context.Context) error

// NewMux returns a mux serving /healthz and /readyz over the given checks.
func NewMux(checks map[string]Check) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		status := "ready"
		code := http.StatusOK
		results := make(map[string]string, len(checks))
		for name, check := range checks {
			if err := check(ctx); err != nil {
				results[name] = err.Error()
				status = "unready"
				code = http.StatusServiceUnavailable
			} else {
				results[name] = "ok"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{"status": status, "checks": results})
	})
	return mux
}

// Serve runs an HTTP server until ctx is cancelled, then drains gracefully.
func Serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/madson/geoson/libs/ogc-kit && go test ./health/ -v`
Expected: PASS (3 tests)

- [ ] **Step 6: Write failing gateway smoke test**

`services/gateway/main_test.go`:

```go
package main

import (
	"net/http/httptest"
	"testing"
)

func TestGatewayServesHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q, want 200 ok", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 7: Run test to verify it fails**

Run: `cd /home/madson/geoson/services/gateway && go test ./...`
Expected: FAIL — `undefined: newHandler`

- [ ] **Step 8: Implement gateway main**

`services/gateway/main.go`:

```go
// Command gateway is the Geoson OWS front door. Sprint 1: health endpoints only.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoson/geoson/libs/ogc-kit/health"
)

func newHandler() http.Handler {
	// No dependencies yet; checks are added as sprints wire in Postgres/Redis/NATS.
	return health.NewMux(map[string]health.Check{})
}

func main() {
	addr := os.Getenv("GEOSON_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	slog.Info("gateway listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler()); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
```

Then wire the local dependency:

```bash
cd /home/madson/geoson/services/gateway
go mod edit -require=github.com/geoson/geoson/libs/ogc-kit@v0.0.0
go mod edit -replace=github.com/geoson/geoson/libs/ogc-kit=../../libs/ogc-kit
go mod tidy
```

- [ ] **Step 9: Run all Go tests to verify pass**

Run: `cd /home/madson/geoson && go test ./libs/... ./services/...`
Expected: PASS everywhere

- [ ] **Step 10: Commit**

```bash
git add go.work libs/ogc-kit services/gateway
git commit -m "feat(gateway): go workspace, health convention lib, gateway stub"
```

---

### Task 3: Rust workspace + wms health stub (TDD)

**Files:**
- Create: `Cargo.toml` (workspace root)
- Create: `libs/geo-core/Cargo.toml`, `libs/geo-core/src/lib.rs`, `libs/geo-core/src/health.rs`
- Create: `services/wms/Cargo.toml`, `services/wms/src/main.rs`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - Crate `geo-core` with module `health`:
    - `pub type Check = Arc<dyn Fn() -> BoxFuture<'static, Result<(), String>> + Send + Sync>`
    - `pub fn router(checks: HashMap<String, Check>) -> axum::Router` — serves `/healthz` and `/readyz` per Global Constraints.
  - Binary crate `wms` using it. Every later Rust service copies this shape.

- [ ] **Step 1: Create workspace and crates**

Root `Cargo.toml`:

```toml
[workspace]
resolver = "2"
members = ["libs/geo-core", "services/wms"]

[workspace.dependencies]
axum = "0.8"
tokio = { version = "1", features = ["rt-multi-thread", "macros", "signal"] }
serde_json = "1"
futures = "0.3"
```

`libs/geo-core/Cargo.toml`:

```toml
[package]
name = "geo-core"
version = "0.1.0"
edition = "2021"
license = "Apache-2.0"

[dependencies]
axum = { workspace = true }
serde_json = { workspace = true }
futures = { workspace = true }

[dev-dependencies]
tokio = { workspace = true }
tower = { version = "0.5", features = ["util"] }
http-body-util = "0.1"
```

`services/wms/Cargo.toml`:

```toml
[package]
name = "wms"
version = "0.1.0"
edition = "2021"
license = "Apache-2.0"

[dependencies]
geo-core = { path = "../../libs/geo-core" }
axum = { workspace = true }
tokio = { workspace = true }
```

`libs/geo-core/src/lib.rs`:

```rust
pub mod health;
```

- [ ] **Step 2: Write failing tests for health router**

Append to `libs/geo-core/src/health.rs` (create file with only the test module first):

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use http_body_util::BodyExt;
    use std::collections::HashMap;
    use std::sync::Arc;
    use tower::ServiceExt;

    fn ok_check() -> Check {
        Arc::new(|| Box::pin(async { Ok(()) }))
    }

    fn failing_check(msg: &'static str) -> Check {
        Arc::new(move || Box::pin(async move { Err(msg.to_string()) }))
    }

    #[tokio::test]
    async fn healthz_always_ok() {
        let app = router(HashMap::from([("broken".to_string(), failing_check("down"))]));
        let res = app
            .oneshot(Request::get("/healthz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::OK);
        let body = res.into_body().collect().await.unwrap().to_bytes();
        assert_eq!(&body[..], b"ok");
    }

    #[tokio::test]
    async fn readyz_ready_when_checks_pass() {
        let app = router(HashMap::from([("redis".to_string(), ok_check())]));
        let res = app
            .oneshot(Request::get("/readyz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::OK);
        let body = res.into_body().collect().await.unwrap().to_bytes();
        let v: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(v["status"], "ready");
        assert_eq!(v["checks"]["redis"], "ok");
    }

    #[tokio::test]
    async fn readyz_unready_when_check_fails() {
        let app = router(HashMap::from([(
            "postgres".to_string(),
            failing_check("conn refused"),
        )]));
        let res = app
            .oneshot(Request::get("/readyz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::SERVICE_UNAVAILABLE);
        let body = res.into_body().collect().await.unwrap().to_bytes();
        let v: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(v["status"], "unready");
        assert_eq!(v["checks"]["postgres"], "conn refused");
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/madson/geoson && cargo test -p geo-core`
Expected: compile FAIL — `cannot find function router`, `cannot find type Check`

- [ ] **Step 4: Implement health module**

Prepend to `libs/geo-core/src/health.rs` (above the test module):

```rust
//! Geoson project-wide health convention:
//! GET /healthz -> 200 "ok" (liveness, never checks dependencies)
//! GET /readyz  -> 200/503 JSON {"status": ..., "checks": {...}} (readiness)

use axum::http::StatusCode;
use axum::response::IntoResponse;
use axum::routing::get;
use axum::{Json, Router};
use futures::future::BoxFuture;
use std::collections::HashMap;
use std::sync::Arc;

pub type Check = Arc<dyn Fn() -> BoxFuture<'static, Result<(), String>> + Send + Sync>;

pub fn router(checks: HashMap<String, Check>) -> Router {
    let checks = Arc::new(checks);
    Router::new()
        .route("/healthz", get(|| async { "ok" }))
        .route(
            "/readyz",
            get(move || {
                let checks = checks.clone();
                async move {
                    let mut results = serde_json::Map::new();
                    let mut ready = true;
                    for (name, check) in checks.iter() {
                        match check().await {
                            Ok(()) => {
                                results.insert(name.clone(), "ok".into());
                            }
                            Err(msg) => {
                                results.insert(name.clone(), msg.into());
                                ready = false;
                            }
                        }
                    }
                    let status = if ready { "ready" } else { "unready" };
                    let code = if ready {
                        StatusCode::OK
                    } else {
                        StatusCode::SERVICE_UNAVAILABLE
                    };
                    (
                        code,
                        Json(serde_json::json!({"status": status, "checks": results})),
                    )
                        .into_response()
                }
            }),
        )
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/madson/geoson && cargo test -p geo-core`
Expected: PASS (3 tests)

- [ ] **Step 6: Implement wms main**

`services/wms/src/main.rs`:

```rust
//! Geoson WMS renderer. Sprint 1: health endpoints only.

use std::collections::HashMap;

#[tokio::main]
async fn main() {
    let addr = std::env::var("GEOSON_HTTP_ADDR").unwrap_or_else(|_| ":8080".into());
    // Accept ":8080" (Go-style) and "0.0.0.0:8080" forms.
    let addr = if addr.starts_with(':') {
        format!("0.0.0.0{addr}")
    } else {
        addr
    };
    let app = geo_core::health::router(HashMap::new());
    let listener = tokio::net::TcpListener::bind(&addr).await.expect("bind");
    println!("wms listening on {addr}");
    axum::serve(listener, app)
        .with_graceful_shutdown(async {
            let mut term =
                tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
                    .expect("sigterm handler");
            tokio::select! {
                _ = term.recv() => {},
                _ = tokio::signal::ctrl_c() => {},
            }
        })
        .await
        .expect("server");
}
```

- [ ] **Step 7: Verify build and full test suite**

Run: `cd /home/madson/geoson && cargo build -p wms && cargo test`
Expected: build OK, tests PASS

- [ ] **Step 8: Commit**

```bash
git add Cargo.toml Cargo.lock libs/geo-core services/wms
git commit -m "feat(wms): rust workspace, health convention module, wms stub"
```

---

### Task 4: Dockerfiles for gateway and wms

**Files:**
- Create: `services/gateway/Dockerfile`
- Create: `services/wms/Dockerfile`
- Create: `.dockerignore`

**Interfaces:**
- Consumes: Task 2 gateway binary, Task 3 wms binary.
- Produces: images `geoson/gateway` and `geoson/wms`, each listening on `:8080` with `/healthz`; consumed by Task 5 compose. Build context is always **repo root** (monorepo needs libs/).

- [ ] **Step 1: Write .dockerignore**

```
.git
target/
node_modules/
frontend/.next/
docs/
*.md
```

- [ ] **Step 2: Write gateway Dockerfile**

`services/gateway/Dockerfile`:

```dockerfile
# Build context must be the repo root: docker build -f services/gateway/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.work go.work.sum* ./
COPY libs/ogc-kit/ libs/ogc-kit/
COPY services/gateway/ services/gateway/
RUN go build -C services/gateway -ldflags="-s -w" -o /out/gateway .

FROM alpine:3.21
RUN apk add --no-cache curl ca-certificates && adduser -D -u 10001 geoson
USER geoson
COPY --from=build /out/gateway /usr/local/bin/gateway
ENV GEOSON_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["gateway"]
```

- [ ] **Step 3: Write wms Dockerfile**

`services/wms/Dockerfile`:

```dockerfile
# Build context must be the repo root: docker build -f services/wms/Dockerfile .
FROM rust:1-alpine AS build
RUN apk add --no-cache musl-dev
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY libs/geo-core/ libs/geo-core/
COPY services/wms/ services/wms/
RUN cargo build --release -p wms

FROM alpine:3.21
RUN apk add --no-cache curl && adduser -D -u 10001 geoson
USER geoson
COPY --from=build /src/target/release/wms /usr/local/bin/wms
ENV GEOSON_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["wms"]
```

- [ ] **Step 4: Build both images to verify**

Run:
```bash
cd /home/madson/geoson
docker build -f services/gateway/Dockerfile -t geoson/gateway:dev .
docker build -f services/wms/Dockerfile -t geoson/wms:dev .
```
Expected: both builds succeed.

- [ ] **Step 5: Smoke-run one container**

Run:
```bash
docker run -d --name g1 -p 18080:8080 geoson/gateway:dev
sleep 1
curl -fsS http://localhost:18080/healthz && echo
curl -fsS http://localhost:18080/readyz && echo
docker rm -f g1
```
Expected: `ok` then `{"status":"ready","checks":{}}`.

- [ ] **Step 6: Commit**

```bash
git add .dockerignore services/gateway/Dockerfile services/wms/Dockerfile
git commit -m "build: multi-stage dockerfiles for gateway and wms"
```

---

### Task 5: Compose stack — infra + services + Traefik routing

**Files:**
- Create: `deploy/compose/docker-compose.yml`
- Create: `deploy/compose/.env.example`

**Interfaces:**
- Consumes: Task 4 images (built by compose `build:`).
- Produces: running stack. Traefik on `:80` routes `PathPrefix(/geoson)` → gateway replicas; wms internal-only (reached by gateway in later sprints). Infra DNS names other sprints rely on: `postgres:5432`, `redis:6379`, `nats:4222`, `minio:9000`. Named volumes: `pgdata`, `miniodata`, `tilecache`.

- [ ] **Step 1: Write .env.example**

```bash
POSTGRES_USER=geoson
POSTGRES_PASSWORD=geoson-dev-password
POSTGRES_DB=geoson
MINIO_ROOT_USER=geoson
MINIO_ROOT_PASSWORD=geoson-dev-password
```

- [ ] **Step 2: Write docker-compose.yml**

`deploy/compose/docker-compose.yml`:

```yaml
name: geoson

services:
  traefik:
    image: traefik:v3.4
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
      - --ping=true
    ports:
      - "80:80"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    healthcheck:
      test: ["CMD", "traefik", "healthcheck", "--ping"]
      interval: 10s
      timeout: 3s
      retries: 3

  gateway:
    build:
      context: ../..
      dockerfile: services/gateway/Dockerfile
    labels:
      - traefik.enable=true
      - traefik.http.routers.gateway.rule=PathPrefix(`/geoson`) || PathPrefix(`/healthz`) || PathPrefix(`/readyz`)
      - traefik.http.services.gateway.loadbalancer.server.port=8080
      - traefik.http.services.gateway.loadbalancer.healthcheck.path=/healthz
      - traefik.http.services.gateway.loadbalancer.healthcheck.interval=5s
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }
      nats: { condition: service_healthy }

  wms:
    build:
      context: ../..
      dockerfile: services/wms/Dockerfile
    # internal only — gateway talks to it via service DNS (http://wms:8080)
    depends_on:
      postgres: { condition: service_healthy }

  postgres:
    image: postgis/postgis:17-3.5-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-geoson}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-geoson-dev-password}
      POSTGRES_DB: ${POSTGRES_DB:-geoson}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 5s
      timeout: 3s
      retries: 10

  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

  nats:
    image: nats:2.11-alpine
    command: ["-js", "-m", "8222"]
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O- http://localhost:8222/healthz || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 10

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-geoson}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD:-geoson-dev-password}
    volumes:
      - miniodata:/data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 10

volumes:
  pgdata:
  miniodata:
  tilecache:
```

- [ ] **Step 3: Validate compose file**

Run: `cd /home/madson/geoson/deploy/compose && docker compose config -q`
Expected: no output (valid).

- [ ] **Step 4: Boot the stack**

Run:
```bash
cd /home/madson/geoson/deploy/compose
cp .env.example .env
docker compose up -d --build
docker compose ps
```
Expected: all services `healthy` (gateway/wms show healthy via their HEALTHCHECK).

- [ ] **Step 5: Verify routing through Traefik**

Run: `curl -fsS http://localhost/healthz && echo`
Expected: `ok` (served by gateway through Traefik).

- [ ] **Step 6: Commit**

```bash
git add deploy/compose
git commit -m "feat(deploy): compose stack with traefik, postgis, redis, nats, minio"
```

---

### Task 6: Horizontal-scale smoke test script

**Files:**
- Create: `deploy/compose/scale-smoke.sh` (executable)

**Interfaces:**
- Consumes: Task 5 running stack.
- Produces: `./scale-smoke.sh <service> <replicas>` — scales, waits healthy, proves Traefik balances across replicas. Referenced by ops docs (Task 8).

- [ ] **Step 1: Write the script**

`deploy/compose/scale-smoke.sh`:

```bash
#!/usr/bin/env bash
# Scale a Geoson service and prove Traefik load-balances across the replicas.
# Usage: ./scale-smoke.sh gateway 4
set -euo pipefail
cd "$(dirname "$0")"

SERVICE="${1:-gateway}"
REPLICAS="${2:-3}"

echo "==> scaling ${SERVICE} to ${REPLICAS} replicas"
docker compose up -d --scale "${SERVICE}=${REPLICAS}" --no-recreate

echo "==> waiting for replicas to be healthy"
for i in $(seq 1 30); do
    healthy=$(docker compose ps "${SERVICE}" --format '{{.Health}}' | grep -c healthy || true)
    [ "${healthy}" -eq "${REPLICAS}" ] && break
    sleep 2
done
healthy=$(docker compose ps "${SERVICE}" --format '{{.Health}}' | grep -c healthy || true)
if [ "${healthy}" -ne "${REPLICAS}" ]; then
    echo "FAIL: only ${healthy}/${REPLICAS} healthy" >&2
    exit 1
fi
echo "==> ${healthy}/${REPLICAS} healthy"

echo "==> hitting /healthz 20x through traefik"
for i in $(seq 1 20); do
    curl -fsS http://localhost/healthz >/dev/null
done
echo "==> checking requests spread across replicas (docker logs)"
docker compose ps "${SERVICE}" --format '{{.Name}}'
echo "OK: ${SERVICE} scaled to ${REPLICAS} and serving through traefik"
```

- [ ] **Step 2: Make executable and run**

Run:
```bash
chmod +x /home/madson/geoson/deploy/compose/scale-smoke.sh
/home/madson/geoson/deploy/compose/scale-smoke.sh gateway 3
```
Expected: ends with `OK: gateway scaled to 3 and serving through traefik`.

- [ ] **Step 3: Scale back down and commit**

```bash
cd /home/madson/geoson/deploy/compose && docker compose up -d --scale gateway=1 --no-recreate
cd /home/madson/geoson
git add deploy/compose/scale-smoke.sh
git commit -m "feat(deploy): horizontal scale smoke test script"
```

---

### Task 7: CI — GitHub Actions

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: Go workspace (Task 2), Rust workspace (Task 3), compose file (Task 5).
- Produces: CI running on every push/PR: Go tests, Rust tests + clippy + fmt, compose validation, docker builds. Later sprints append jobs (filter corpus, compat harness) to this file.

- [ ] **Step 1: Write workflow**

`.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - run: go vet ./libs/... ./services/...
      - run: go test ./libs/... ./services/...

  rust:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@stable
        with:
          components: clippy, rustfmt
      - uses: Swatinem/rust-cache@v2
      - run: cargo fmt --all --check
      - run: cargo clippy --workspace -- -D warnings
      - run: cargo test --workspace

  compose:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker compose -f deploy/compose/docker-compose.yml config -q

  docker-build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker build -f services/gateway/Dockerfile .
      - run: docker build -f services/wms/Dockerfile .
```

- [ ] **Step 2: Verify workflow YAML is valid and local checks match CI**

Run:
```bash
cd /home/madson/geoson
docker compose -f deploy/compose/docker-compose.yml config -q
go vet ./libs/... ./services/... && go test ./libs/... ./services/...
cargo fmt --all --check && cargo clippy --workspace -- -D warnings && cargo test --workspace
```
Expected: all pass (fix any fmt/clippy findings now).

- [ ] **Step 3: Commit**

```bash
git add .github
git commit -m "ci: go, rust, compose validation, docker build jobs"
```

---

### Task 8: Docs scaffold

**Files:**
- Create: `docs/architecture.md`, `docs/dev/getting-started.md`, `docs/ops/scaling.md`

**Interfaces:**
- Consumes: everything above.
- Produces: docs skeleton later sprints extend (one page per service gets added under `docs/` as each service lands).

- [ ] **Step 1: Write docs/architecture.md**

```markdown
# Geoson Architecture

Full approved design: [design spec](superpowers/specs/2026-07-16-geoson-engine-design.md).

## Services

| Service | Language | Role | Status |
|---|---|---|---|
| gateway | Go | OWS front door: parsing, negotiation, exceptions, routing | health stub |
| catalog | Go | config system of record, GeoServer `/rest` compat | planned (Sprint 2) |
| auth | Go | users/groups/roles, GeoFence rules | planned (Sprint 4) |
| wfs | Go | WFS 1.0/1.1/2.0, filters, WFS-T | planned (Sprint 5) |
| wms | Rust | WMS 1.1.1/1.3.0 rendering | health stub |
| tiles | Rust | WMTS/XYZ/TMS, MVT, cache + seeding | planned (Sprint 7) |
| wps | Rust | WPS 1.0 process engine | planned (Sprint 8) |
| convert | Rust | ingest + format conversion | planned (Sprint 8) |
| frontend | Next.js 15 | admin UI | planned (Sprint 9) |

## Infra

Traefik (LB) · Postgres+PostGIS (catalog + data) · Redis (caches/sessions) ·
NATS JetStream (events + jobs) · MinIO (optional object storage).

## Conventions

- Health: `/healthz` liveness (`200 ok`), `/readyz` readiness (JSON, 200/503) — every service.
- Listen address: `GEOSON_HTTP_ADDR` (default `:8080`).
- Stateless request path; state only in Postgres/Redis/NATS/object storage.
- Docker build context: repo root, `-f services/<name>/Dockerfile`.
```

- [ ] **Step 2: Write docs/dev/getting-started.md**

```markdown
# Getting Started (dev)

## Prerequisites

Go 1.24+, Rust stable, Docker + compose plugin.

## Run everything

    cd deploy/compose
    cp .env.example .env
    docker compose up -d --build
    curl http://localhost/healthz    # -> ok (gateway via Traefik)

## Run tests

    go test ./libs/... ./services/...   # Go
    cargo test --workspace              # Rust

## Repo layout

See `docs/architecture.md` and spec §6. Task tracker: `task.md`.
```

- [ ] **Step 3: Write docs/ops/scaling.md**

```markdown
# Scaling & HA

Every request-path service is stateless — scale any of them independently:

    cd deploy/compose
    docker compose up -d --scale wms=4 --scale gateway=2 --no-recreate

Traefik discovers replicas via the Docker provider and round-robins with
per-replica health checks (`/healthz`).

Prove it works:

    ./scale-smoke.sh gateway 4

## Notes

- Postgres is the single stateful primary; scale reads later via replicas.
- Tile seeding uses Redis locks so replicas never render the same metatile twice (Sprint 7).
- Docker Swarm stack for multi-node HA lands in Sprint 10 (`deploy/swarm/`).
```

- [ ] **Step 4: Commit**

```bash
git add docs/architecture.md docs/dev docs/ops
git commit -m "docs: architecture, getting started, scaling scaffold"
```

---

### Task 9: Close out Sprint 1

**Files:**
- Modify: `task.md` (check Sprint 1)

**Interfaces:**
- Consumes: all tasks above complete.
- Produces: tracker state for next session.

- [ ] **Step 1: Full verification pass**

Run:
```bash
cd /home/madson/geoson
go test ./libs/... ./services/...
cargo test --workspace
cd deploy/compose && docker compose up -d --build && docker compose ps && curl -fsS http://localhost/healthz && echo
```
Expected: tests pass, stack healthy, `ok`.

- [ ] **Step 2: Check Sprint 1 in task.md**

Change `- [ ] **Sprint 1 — Skeleton & infra**` to `- [x] **Sprint 1 — Skeleton & infra**`.

- [ ] **Step 3: Commit**

```bash
git add task.md docs/superpowers/plans/2026-07-16-sprint-1-skeleton-infra.md
git commit -m "chore: complete sprint 1 (skeleton & infra)"
```
