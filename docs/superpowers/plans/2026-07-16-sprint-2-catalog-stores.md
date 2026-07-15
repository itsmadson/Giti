# Sprint 2 — Catalog + Stores Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Catalog service (Go + Postgres): GeoServer-shaped config model with migrations, GeoServer `/rest`-compatible API (XML + JSON), store connectors (PostGIS, files, COG, GeoParquet/DuckDB), and NATS config-change events.

**Architecture:** `services/catalog` is the system of record. Entities mirror GeoServer's model (workspace → store → featuretype/coverage → layer; styles; layergroups) so `/rest` compat is natural. Repository over pgx against the compose Postgres; connectors are a registry keyed by store type, each implementing `Validate` + `Introspect`. Every mutation publishes a NATS event.

**Tech Stack:** Go 1.26, pgx/v5, embedded SQL migrations (no external tool), nats.go, DuckDB via `github.com/marcboeker/go-duckdb/v2` (cgo — catalog image is debian-based, not alpine).

## Global Constraints

- Health convention from Sprint 1: `/healthz` → `200 ok`, `/readyz` → JSON 200/503; `GEOSON_HTTP_ADDR` default `:8080` (use `libs/ogc-kit/health`).
- Go tests run as `go test github.com/geoson/geoson/...` (dir patterns don't cross module boundaries).
- `/rest` responses must match GeoServer shapes: XML default, JSON via `.json` suffix or `Accept` header; both directions (request bodies too).
- Integration tests need Postgres: they read `GEOSON_TEST_DATABASE_URL` and `t.Skip` when unset. Compose exposes Postgres on `127.0.0.1:5433` for this.
- Every catalog mutation publishes NATS subject `catalog.<entity>.<created|updated|deleted>` with JSON `{"name": ..., "workspace": ...}`.
- Commit after every task, Conventional Commits.
- Deferred from spec §3.2 to later sprints: `/rest/namespaces` (namespace URI already stored on workspaces), global settings endpoints, SQL views + filter pushdown (WFS sprint), image mosaics.

## File Structure

```
services/catalog/
  go.mod  main.go  main_test.go  Dockerfile
  internal/model/model.go            # entity structs (GeoServer-shaped)
  internal/store/store.go            # pgx repository (CRUD per entity)
  internal/store/migrate.go          # embedded migrations runner
  internal/store/migrations/0001_init.sql
  internal/store/store_test.go       # integration tests (skip w/o DB)
  internal/rest/encode.go            # dual XML/JSON content negotiation
  internal/rest/workspaces.go        # /rest/workspaces handlers
  internal/rest/stores.go            # /rest/.../datastores + coveragestores
  internal/rest/layers.go            # featuretypes, layers, styles, layergroups
  internal/rest/rest.go              # router assembly
  internal/rest/*_test.go
  internal/connect/connect.go        # Connector interface + registry
  internal/connect/postgis.go        # live introspection via pgx
  internal/connect/files.go          # shapefile/gpkg/geojson validation
  internal/connect/cog.go            # GeoTIFF magic/BigTIFF check
  internal/connect/geoparquet.go     # DuckDB parquet introspection
  internal/connect/*_test.go
  internal/events/events.go          # NATS publisher (no-op without NATS)
```

---

### Task 1: Catalog service scaffold + compose wiring

**Files:**
- Create: `services/catalog/go.mod`, `services/catalog/main.go`, `services/catalog/main_test.go`, `services/catalog/Dockerfile`
- Modify: `go.work` (add module), `deploy/compose/docker-compose.yml` (catalog service + postgres port expose), `.github/workflows/ci.yml` (docker build + Go test service container)

**Interfaces:**
- Consumes: `libs/ogc-kit/health` (`health.NewMux`, `health.Check`, `health.Serve`) from Sprint 1.
- Produces: `newHandler(deps) http.Handler` in `main.go` where `type deps struct { db *pgxpool.Pool; nc *nats.Conn }` — later tasks mount `/rest` and `/api/v1` into it. Service DNS `catalog:8080`. Postgres reachable for tests at `127.0.0.1:5433`.

- [ ] **Step 1: Init module, add to workspace**

```bash
cd /home/madson/geoson
mkdir -p services/catalog
( cd services/catalog && go mod init github.com/geoson/geoson/services/catalog )
go work use ./services/catalog
cd services/catalog
go mod edit -require=github.com/geoson/geoson/libs/ogc-kit@v0.0.0
go mod edit -replace=github.com/geoson/geoson/libs/ogc-kit=../../libs/ogc-kit
go get github.com/jackc/pgx/v5/pgxpool@latest github.com/nats-io/nats.go@latest
```

- [ ] **Step 2: Failing smoke test**

`services/catalog/main_test.go`:

```go
package main

import (
	"net/http/httptest"
	"testing"
)

func TestCatalogServesHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler(deps{}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q, want 200 ok", rec.Code, rec.Body.String())
	}
}
```

Run: `go test ./...` → FAIL `undefined: newHandler`

- [ ] **Step 3: Implement main.go**

```go
// Command catalog is the Geoson configuration system of record.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoson/geoson/libs/ogc-kit/health"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type deps struct {
	db *pgxpool.Pool
	nc *nats.Conn
}

func newHandler(d deps) http.Handler {
	checks := map[string]health.Check{}
	if d.db != nil {
		checks["postgres"] = func(ctx context.Context) error { return d.db.Ping(ctx) }
	}
	if d.nc != nil {
		checks["nats"] = func(ctx context.Context) error {
			if !d.nc.IsConnected() {
				return context.DeadlineExceeded
			}
			return nil
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(checks))
	mux.Handle("/readyz", health.NewMux(checks))
	// Task 3+ mount /rest and /api/v1 here via rest.Mount(mux, d.db, publisher).
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	addr := os.Getenv("GEOSON_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	var d deps
	if dsn := os.Getenv("GEOSON_DATABASE_URL"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			slog.Error("postgres connect", "err", err)
			os.Exit(1)
		}
		d.db = pool
	}
	if url := os.Getenv("GEOSON_NATS_URL"); url != "" {
		nc, err := nats.Connect(url)
		if err != nil {
			slog.Error("nats connect", "err", err)
			os.Exit(1)
		}
		d.nc = nc
	}
	slog.Info("catalog listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(d)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
```

Run: `go mod tidy && go test ./...` → PASS

- [ ] **Step 4: Dockerfile (debian-based — DuckDB cgo lands in Task 6)**

`services/catalog/Dockerfile`:

```dockerfile
# Build context must be the repo root: docker build -f services/catalog/Dockerfile .
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.work go.work.sum* ./
COPY libs/ogc-kit/ libs/ogc-kit/
COPY services/gateway/ services/gateway/
COPY services/catalog/ services/catalog/
RUN go build -C services/catalog -ldflags="-s -w" -o /out/catalog .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/* && useradd -u 10001 geoson
USER geoson
COPY --from=build /out/catalog /usr/local/bin/catalog
ENV GEOSON_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["catalog"]
```

(go.work lists all modules, so gateway must be copied too for `go build` to resolve the workspace.)

- [ ] **Step 5: Compose wiring**

In `deploy/compose/docker-compose.yml`:
- add under `postgres:` service:

```yaml
    ports:
      - "127.0.0.1:5433:5432"
```

- add service:

```yaml
  catalog:
    build:
      context: ../..
      dockerfile: services/catalog/Dockerfile
    environment:
      GEOSON_DATABASE_URL: postgres://${POSTGRES_USER:-geoson}:${POSTGRES_PASSWORD:-geoson-dev-password}@postgres:5432/${POSTGRES_DB:-geoson}
      GEOSON_NATS_URL: nats://nats:4222
    labels:
      - traefik.enable=true
      - traefik.http.routers.catalog.rule=PathPrefix(`/geoserver/rest`) || PathPrefix(`/api/v1`)
      - traefik.http.services.catalog.loadbalancer.server.port=8080
    depends_on:
      postgres: { condition: service_healthy }
      nats: { condition: service_healthy }
```

- [ ] **Step 6: CI — add postgres service to go job, add catalog docker build**

In `.github/workflows/ci.yml` `go:` job add:

```yaml
    services:
      postgres:
        image: postgis/postgis:17-3.5-alpine
        env:
          POSTGRES_USER: geoson
          POSTGRES_PASSWORD: geoson
          POSTGRES_DB: geoson_test
        ports: ["5433:5432"]
        options: >-
          --health-cmd "pg_isready -U geoson" --health-interval 5s
          --health-timeout 3s --health-retries 10
    env:
      GEOSON_TEST_DATABASE_URL: postgres://geoson:geoson@localhost:5433/geoson_test
```

And in `docker-build:` job add:

```yaml
      - run: docker build -f services/catalog/Dockerfile .
```

- [ ] **Step 7: Verify + commit**

```bash
cd /home/madson/geoson
go test github.com/geoson/geoson/...
docker compose -f deploy/compose/docker-compose.yml config -q
docker build -f services/catalog/Dockerfile . 
cd deploy/compose && docker compose up -d --build catalog postgres && docker compose ps catalog
git add -A && git commit -m "feat(catalog): service scaffold, compose + ci wiring"
```

Expected: tests pass, catalog container healthy.

---

### Task 2: Migrations + workspace repository (TDD, real Postgres)

**Files:**
- Create: `services/catalog/internal/store/migrate.go`, `services/catalog/internal/store/migrations/0001_init.sql`, `services/catalog/internal/store/store.go`, `services/catalog/internal/store/store_test.go`
- Modify: `services/catalog/main.go` (run migrations at boot)

**Interfaces:**
- Consumes: compose Postgres at `127.0.0.1:5433` (env `GEOSON_TEST_DATABASE_URL`).
- Produces (package `store`):
  - `func Migrate(ctx context.Context, db *pgxpool.Pool) error`
  - `func New(db *pgxpool.Pool) *Store`
  - `type Store struct{ ... }` with methods used by all later tasks:
    - `CreateWorkspace(ctx, model.Workspace) error`
    - `GetWorkspace(ctx, name string) (model.Workspace, error)` (returns `ErrNotFound`)
    - `ListWorkspaces(ctx) ([]model.Workspace, error)`
    - `UpdateWorkspace(ctx, name string, w model.Workspace) error`
    - `DeleteWorkspace(ctx, name string, recurse bool) error`
  - `var ErrNotFound = errors.New("not found")`, `var ErrConflict = errors.New("already exists")`
- Also produces `internal/model/model.go` with:

```go
package model

type Workspace struct {
	Name         string `json:"name" xml:"name"`
	Isolated     bool   `json:"isolated,omitempty" xml:"isolated,omitempty"`
	NamespaceURI string `json:"-" xml:"-"` // stored alongside; exposed via /rest/namespaces later
}

type Store struct { // datastore or coveragestore
	Name        string            `json:"name" xml:"name"`
	Workspace   string            `json:"-" xml:"-"`
	Kind        string            `json:"-" xml:"-"` // "datastore" | "coveragestore"
	Type        string            `json:"type,omitempty" xml:"type,omitempty"` // PostGIS, Shapefile, GeoPackage, GeoJSON, GeoTIFF, GeoParquet
	Enabled     bool              `json:"enabled" xml:"enabled"`
	Description string            `json:"description,omitempty" xml:"description,omitempty"`
	Connection  map[string]string `json:"-" xml:"-"` // connectionParameters
}

type FeatureType struct {
	Name       string `json:"name" xml:"name"`
	NativeName string `json:"nativeName" xml:"nativeName"`
	Workspace  string `json:"-" xml:"-"`
	Store      string `json:"-" xml:"-"`
	Title      string `json:"title,omitempty" xml:"title,omitempty"`
	SRS        string `json:"srs,omitempty" xml:"srs,omitempty"`
	Enabled    bool   `json:"enabled" xml:"enabled"`
}

type Coverage struct {
	Name       string `json:"name" xml:"name"`
	NativeName string `json:"nativeName" xml:"nativeName"`
	Workspace  string `json:"-" xml:"-"`
	Store      string `json:"-" xml:"-"`
	Title      string `json:"title,omitempty" xml:"title,omitempty"`
	SRS        string `json:"srs,omitempty" xml:"srs,omitempty"`
	Enabled    bool   `json:"enabled" xml:"enabled"`
}

type Layer struct {
	Name         string `json:"name" xml:"name"`
	Workspace    string `json:"-" xml:"-"`
	Type         string `json:"type" xml:"type"` // VECTOR | RASTER
	ResourceName string `json:"-" xml:"-"`       // featuretype/coverage name
	DefaultStyle string `json:"-" xml:"-"`
	Enabled      bool   `json:"enabled,omitempty" xml:"enabled,omitempty"`
}

type Style struct {
	Name      string `json:"name" xml:"name"`
	Workspace string `json:"-" xml:"-"` // empty = global
	Format    string `json:"format,omitempty" xml:"format,omitempty"` // sld | mbstyle | geocss
	Filename  string `json:"filename,omitempty" xml:"filename,omitempty"`
	Body      string `json:"-" xml:"-"`
}

type LayerGroup struct {
	Name      string   `json:"name" xml:"name"`
	Workspace string   `json:"-" xml:"-"`
	Mode      string   `json:"mode" xml:"mode"` // SINGLE
	Layers    []string `json:"-" xml:"-"`
}
```

- [ ] **Step 1: Write 0001_init.sql**

`services/catalog/internal/store/migrations/0001_init.sql`:

```sql
CREATE TABLE IF NOT EXISTS geoson_migrations (
    version int PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspaces (
    name text PRIMARY KEY,
    isolated boolean NOT NULL DEFAULT false,
    namespace_uri text NOT NULL DEFAULT ''
);

CREATE TABLE stores (
    workspace text NOT NULL REFERENCES workspaces(name) ON DELETE CASCADE,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('datastore','coveragestore')),
    type text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    description text NOT NULL DEFAULT '',
    connection jsonb NOT NULL DEFAULT '{}',
    PRIMARY KEY (workspace, name)
);

CREATE TABLE resources (
    workspace text NOT NULL,
    store text NOT NULL,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('featuretype','coverage')),
    native_name text NOT NULL,
    title text NOT NULL DEFAULT '',
    srs text NOT NULL DEFAULT 'EPSG:4326',
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (workspace, store, name),
    FOREIGN KEY (workspace, store) REFERENCES stores(workspace, name) ON DELETE CASCADE
);

CREATE TABLE layers (
    workspace text NOT NULL REFERENCES workspaces(name) ON DELETE CASCADE,
    name text NOT NULL,
    type text NOT NULL CHECK (type IN ('VECTOR','RASTER')),
    resource_name text NOT NULL,
    default_style text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (workspace, name)
);

CREATE TABLE styles (
    workspace text NOT NULL DEFAULT '',
    name text NOT NULL,
    format text NOT NULL DEFAULT 'sld',
    filename text NOT NULL DEFAULT '',
    body text NOT NULL DEFAULT '',
    PRIMARY KEY (workspace, name)
);

CREATE TABLE layer_groups (
    workspace text NOT NULL DEFAULT '',
    name text NOT NULL,
    mode text NOT NULL DEFAULT 'SINGLE',
    layers jsonb NOT NULL DEFAULT '[]',
    PRIMARY KEY (workspace, name)
);
```

- [ ] **Step 2: Failing migration + workspace CRUD test**

`services/catalog/internal/store/store_test.go`:

```go
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/geoson/geoson/services/catalog/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GEOSON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GEOSON_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// isolate: fresh schema per test run
	schema := fmt.Sprintf("t%d", time.Now().UnixNano())
	if _, err := pool.Exec(context.Background(),
		fmt.Sprintf("CREATE SCHEMA %s; SET search_path TO %s", schema, schema)); err != nil {
		t.Fatal(err)
	}
	cfg := pool.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool2, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
		pool2.Close()
	})
	return pool2
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestWorkspaceCRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)

	if err := s.CreateWorkspace(ctx, model.Workspace{Name: "topp"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateWorkspace(ctx, model.Workspace{Name: "topp"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup create = %v, want ErrConflict", err)
	}
	got, err := s.GetWorkspace(ctx, "topp")
	if err != nil || got.Name != "topp" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if _, err := s.GetWorkspace(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing get = %v, want ErrNotFound", err)
	}
	list, err := s.ListWorkspaces(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}
	if err := s.UpdateWorkspace(ctx, "topp", model.Workspace{Name: "topp2", Isolated: true}); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetWorkspace(ctx, "topp2")
	if err != nil || !got.Isolated {
		t.Fatalf("after update = %+v, %v", got, err)
	}
	if err := s.DeleteWorkspace(ctx, "topp2", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetWorkspace(ctx, "topp2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete = %v, want ErrNotFound", err)
	}
}
```

Run: `cd services/catalog && GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./internal/store/`
Expected: FAIL — `undefined: Migrate`, `undefined: New` (create `internal/model/model.go` first with the structs from Interfaces above, or the import fails).

- [ ] **Step 3: Implement migrate.go**

```go
package store

import (
	"context"
	"embed"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies embedded migrations in filename order, tracking versions
// in geoson_migrations. Safe to run on every boot.
func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS geoson_migrations (
		version int PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for i, name := range names {
		version := i + 1
		var exists bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM geoson_migrations WHERE version=$1)`, version,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO geoson_migrations(version) VALUES($1)`, version); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Implement store.go (workspaces)**

```go
// Package store is the catalog repository over Postgres.
package store

import (
	"context"
	"errors"

	"github.com/geoson/geoson/services/catalog/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) CreateWorkspace(ctx context.Context, w model.Workspace) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO workspaces(name, isolated, namespace_uri) VALUES($1,$2,$3)`,
		w.Name, w.Isolated, w.NamespaceURI)
	return mapErr(err)
}

func (s *Store) GetWorkspace(ctx context.Context, name string) (model.Workspace, error) {
	var w model.Workspace
	err := s.db.QueryRow(ctx,
		`SELECT name, isolated, namespace_uri FROM workspaces WHERE name=$1`, name,
	).Scan(&w.Name, &w.Isolated, &w.NamespaceURI)
	return w, mapErr(err)
}

func (s *Store) ListWorkspaces(ctx context.Context) ([]model.Workspace, error) {
	rows, err := s.db.Query(ctx,
		`SELECT name, isolated, namespace_uri FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Workspace
	for rows.Next() {
		var w model.Workspace
		if err := rows.Scan(&w.Name, &w.Isolated, &w.NamespaceURI); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) UpdateWorkspace(ctx context.Context, name string, w model.Workspace) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspaces SET name=$2, isolated=$3 WHERE name=$1`, name, w.Name, w.Isolated)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteWorkspace(ctx context.Context, name string, recurse bool) error {
	if !recurse {
		var n int
		if err := s.db.QueryRow(ctx,
			`SELECT count(*) FROM stores WHERE workspace=$1`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM workspaces WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

Create `internal/model/model.go` with the structs from the Interfaces block.

- [ ] **Step 5: Run tests → PASS; wire Migrate into main**

In `main.go` after pool creation add:

```go
		if err := store.Migrate(ctx, pool); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
```

(import `github.com/geoson/geoson/services/catalog/internal/store`)

Run: `GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/catalog
git commit -m "feat(catalog): migrations, model, workspace repository"
```

---

### Task 3: Content negotiation + /rest/workspaces (GeoServer shapes)

**Files:**
- Create: `services/catalog/internal/rest/encode.go`, `services/catalog/internal/rest/rest.go`, `services/catalog/internal/rest/workspaces.go`, `services/catalog/internal/rest/workspaces_test.go`
- Modify: `services/catalog/main.go` (mount router)

**Interfaces:**
- Consumes: `store.Store` methods + errors from Task 2.
- Produces (package `rest`):
  - `func Mount(mux *http.ServeMux, s *store.Store, pub Publisher)` — mounts everything under `/geoserver/rest/` and `/rest/` (both prefixes, GeoServer serves both when behind proxy).
  - `type Publisher interface { Publish(subject string, payload any) }` — Task 8 provides NATS impl; tests use a recording fake.
  - `func writePayload(w http.ResponseWriter, r *http.Request, xmlBody, jsonBody any)` — `.json` suffix or `Accept: application/json` → JSON, else XML.
  - `func readPayload(r *http.Request, xmlDst, jsonDst any) error` — Content-Type driven.
- GeoServer shapes (exact):
  - `GET /rest/workspaces` XML: `<workspaces><workspace><name>topp</name></workspace></workspaces>`; JSON: `{"workspaces":{"workspace":[{"name":"topp","href":"..."}]}}`
  - `GET /rest/workspaces/{ws}` XML: `<workspace><name>topp</name><isolated>false</isolated></workspace>`; JSON: `{"workspace":{"name":"topp","isolated":false}}`
  - `POST /rest/workspaces` body `<workspace><name>x</name></workspace>` → `201`, `Location` header
  - `PUT /rest/workspaces/{ws}` → `200`; `DELETE /rest/workspaces/{ws}?recurse=true|false` → `200`; missing → `404`; dup POST → `409`; non-empty delete w/o recurse → `403` (GeoServer uses 403 here)

- [ ] **Step 1: Failing handler tests**

`services/catalog/internal/rest/workspaces_test.go`:

```go
package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geoson/geoson/services/catalog/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakePub struct{ subjects []string }

func (f *fakePub) Publish(subject string, payload any) { f.subjects = append(f.subjects, subject) }

func testMux(t *testing.T) (*http.ServeMux, *fakePub) {
	t.Helper()
	dsn := os.Getenv("GEOSON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GEOSON_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	pool.Exec(context.Background(), `TRUNCATE workspaces CASCADE; TRUNCATE styles; TRUNCATE layer_groups`)
	pub := &fakePub{}
	mux := http.NewServeMux()
	Mount(mux, store.New(pool), pub)
	return mux, pub
}

func do(t *testing.T, mux *http.ServeMux, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestWorkspaceRESTLifecycle(t *testing.T) {
	mux, pub := testMux(t)

	rec := do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>topp</name></workspace>`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>topp</name></workspace>`)
	if rec.Code != 409 {
		t.Fatalf("dup POST = %d", rec.Code)
	}

	rec = do(t, mux, "GET", "/rest/workspaces/topp", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<name>topp</name>") {
		t.Fatalf("GET xml = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp.json", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"name":"topp"`) {
		t.Fatalf("GET json = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces.json", "", "")
	if !strings.Contains(rec.Body.String(), `"workspaces"`) {
		t.Fatalf("list json = %s", rec.Body.String())
	}

	rec = do(t, mux, "POST", "/rest/workspaces.json", "application/json",
		`{"workspace":{"name":"sf"}}`)
	if rec.Code != 201 {
		t.Fatalf("POST json = %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, mux, "PUT", "/rest/workspaces/sf", "application/xml",
		`<workspace><name>sf</name><isolated>true</isolated></workspace>`)
	if rec.Code != 200 {
		t.Fatalf("PUT = %d", rec.Code)
	}
	rec = do(t, mux, "DELETE", "/rest/workspaces/sf", "", "")
	if rec.Code != 200 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
	rec = do(t, mux, "GET", "/rest/workspaces/sf", "", "")
	if rec.Code != 404 {
		t.Fatalf("GET deleted = %d", rec.Code)
	}
	want := []string{"catalog.workspace.created", "catalog.workspace.created",
		"catalog.workspace.updated", "catalog.workspace.deleted"}
	if len(pub.subjects) != len(want) {
		t.Fatalf("events = %v, want %v", pub.subjects, want)
	}
}
```

Run: `GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./internal/rest/`
Expected: FAIL — `undefined: Mount`

- [ ] **Step 2: Implement encode.go**

```go
package rest

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
)

// wantsJSON: .json suffix (GeoServer style) or Accept: application/json.
func wantsJSON(r *http.Request) bool {
	if strings.HasSuffix(r.URL.Path, ".json") {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// trimFormat strips a trailing .json/.xml from the last path segment.
func trimFormat(name string) string {
	name = strings.TrimSuffix(name, ".json")
	return strings.TrimSuffix(name, ".xml")
}

func writePayload(w http.ResponseWriter, r *http.Request, xmlBody, jsonBody any) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonBody)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(xmlBody)
}

func readPayload(r *http.Request, xmlDst, jsonDst any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		return err
	}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "json") {
		return json.Unmarshal(body, jsonDst)
	}
	return xml.Unmarshal(body, xmlDst)
}
```

- [ ] **Step 3: Implement rest.go**

```go
// Package rest implements the GeoServer-compatible /rest configuration API.
package rest

import (
	"errors"
	"net/http"

	"github.com/geoson/geoson/services/catalog/internal/store"
)

type Publisher interface {
	Publish(subject string, payload any)
}

type api struct {
	s   *store.Store
	pub Publisher
}

// Mount registers all /rest handlers under both /rest/ and /geoserver/rest/.
func Mount(mux *http.ServeMux, s *store.Store, pub Publisher) {
	a := &api{s: s, pub: pub}
	inner := http.NewServeMux()
	a.workspaceRoutes(inner)
	mux.Handle("/rest/", inner)
	mux.Handle("/geoserver/rest/", http.StripPrefix("/geoserver", inner))
}

// httpErr maps repository errors to GeoServer-compatible status codes.
func httpErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, store.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
```

- [ ] **Step 4: Implement workspaces.go**

```go
package rest

import (
	"errors"
	"net/http"

	"github.com/geoson/geoson/services/catalog/internal/model"
	"github.com/geoson/geoson/services/catalog/internal/store"
)

// XML/JSON wire shapes for GeoServer compat.
type wsXML struct {
	XMLName  struct{} `xml:"workspace"`
	Name     string   `xml:"name"`
	Isolated bool     `xml:"isolated,omitempty"`
}
type wsListXML struct {
	XMLName struct{} `xml:"workspaces"`
	Items   []wsXML  `xml:"workspace"`
}
type wsJSON struct {
	Workspace struct {
		Name     string `json:"name"`
		Isolated bool   `json:"isolated,omitempty"`
	} `json:"workspace"`
}

func (a *api) workspaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /rest/workspaces", a.listWorkspaces)
	mux.HandleFunc("GET /rest/workspaces.json", a.listWorkspaces)
	mux.HandleFunc("POST /rest/workspaces", a.createWorkspace)
	mux.HandleFunc("POST /rest/workspaces.json", a.createWorkspace)
	mux.HandleFunc("GET /rest/workspaces/{ws}", a.getWorkspace)
	mux.HandleFunc("PUT /rest/workspaces/{ws}", a.updateWorkspace)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}", a.deleteWorkspace)
}

func (a *api) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListWorkspaces(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	xmlOut := wsListXML{}
	type wsRef struct {
		Name string `json:"name"`
	}
	jsonItems := []wsRef{}
	for _, ws := range list {
		xmlOut.Items = append(xmlOut.Items, wsXML{Name: ws.Name, Isolated: ws.Isolated})
		jsonItems = append(jsonItems, wsRef{Name: ws.Name})
	}
	writePayload(w, r, xmlOut,
		map[string]any{"workspaces": map[string]any{"workspace": jsonItems}})
}

func (a *api) readWorkspace(r *http.Request) (model.Workspace, error) {
	var x wsXML
	var j wsJSON
	if err := readPayload(r, &x, &j); err != nil {
		return model.Workspace{}, err
	}
	if j.Workspace.Name != "" {
		return model.Workspace{Name: j.Workspace.Name, Isolated: j.Workspace.Isolated}, nil
	}
	if x.Name == "" {
		return model.Workspace{}, errors.New("workspace name required")
	}
	return model.Workspace{Name: x.Name, Isolated: x.Isolated}, nil
}

func (a *api) createWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := a.readWorkspace(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.s.CreateWorkspace(r.Context(), ws); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.workspace.created", map[string]string{"name": ws.Name})
	w.Header().Set("Location", "/rest/workspaces/"+ws.Name)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(ws.Name))
}

func (a *api) getWorkspace(w http.ResponseWriter, r *http.Request) {
	name := trimFormat(r.PathValue("ws"))
	ws, err := a.s.GetWorkspace(r.Context(), name)
	if err != nil {
		httpErr(w, err)
		return
	}
	var j wsJSON
	j.Workspace.Name = ws.Name
	j.Workspace.Isolated = ws.Isolated
	writePayload(w, r, wsXML{Name: ws.Name, Isolated: ws.Isolated}, j)
}

func (a *api) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	name := trimFormat(r.PathValue("ws"))
	ws, err := a.readWorkspace(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.s.UpdateWorkspace(r.Context(), name, ws); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.workspace.updated", map[string]string{"name": ws.Name})
	w.WriteHeader(http.StatusOK)
}

func (a *api) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	name := trimFormat(r.PathValue("ws"))
	recurse := r.URL.Query().Get("recurse") == "true"
	if err := a.s.DeleteWorkspace(r.Context(), name, recurse); err != nil {
		if errors.Is(err, store.ErrConflict) {
			http.Error(w, "workspace not empty", http.StatusForbidden)
			return
		}
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.workspace.deleted", map[string]string{"name": name})
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 5: Run tests → PASS. Mount in main.go**

Replace the comment line in `newHandler` with:

```go
	rest.Mount(mux, store.New(d.db), d.pub)
```

Guard: only call when `d.db != nil`. Add `pub Publisher` field to `deps` (use `noopPub` struct until Task 8):

```go
type noopPub struct{}

func (noopPub) Publish(subject string, payload any) {}
```

Run: `GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./...` → PASS

- [ ] **Step 6: Commit**

```bash
git add services/catalog
git commit -m "feat(catalog): /rest/workspaces with GeoServer XML/JSON compat"
```

---

### Task 4: Store (datastore/coveragestore) repository + /rest endpoints

**Files:**
- Create: `services/catalog/internal/rest/stores.go`, `services/catalog/internal/rest/stores_test.go`
- Modify: `services/catalog/internal/store/store.go` (add store CRUD), `services/catalog/internal/store/store_test.go` (add test), `services/catalog/internal/rest/rest.go` (register routes)

**Interfaces:**
- Consumes: Task 2 `Store`, Task 3 `writePayload/readPayload/httpErr/trimFormat`.
- Produces (`store.Store` methods):
  - `CreateStore(ctx, model.Store) error` / `GetStore(ctx, ws, name, kind string) (model.Store, error)` / `ListStores(ctx, ws, kind string) ([]model.Store, error)` / `UpdateStore(ctx, ws, name string, st model.Store) error` / `DeleteStore(ctx, ws, name string, recurse bool) error`
- REST shapes (GeoServer):
  - `GET/POST /rest/workspaces/{ws}/datastores`, `GET/PUT/DELETE /rest/workspaces/{ws}/datastores/{ds}`
  - same for `coveragestores`
  - datastore XML: `<dataStore><name>x</name><type>PostGIS</type><enabled>true</enabled><connectionParameters><entry key="host">postgres</entry><entry key="port">5432</entry></connectionParameters></dataStore>`
  - JSON: `{"dataStore":{"name":"x","type":"PostGIS","enabled":true,"connectionParameters":{"entry":[{"@key":"host","$":"postgres"}]}}}`

- [ ] **Step 1: Failing repo test (append to store_test.go)**

```go
func TestStoreCRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if err := s.CreateWorkspace(ctx, model.Workspace{Name: "topp"}); err != nil {
		t.Fatal(err)
	}
	ds := model.Store{
		Workspace: "topp", Name: "pg", Kind: "datastore", Type: "PostGIS",
		Enabled: true, Connection: map[string]string{"host": "postgres", "port": "5432"},
	}
	if err := s.CreateStore(ctx, ds); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetStore(ctx, "topp", "pg", "datastore")
	if err != nil || got.Connection["host"] != "postgres" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if _, err := s.GetStore(ctx, "topp", "pg", "coveragestore"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong-kind get = %v, want ErrNotFound", err)
	}
	list, err := s.ListStores(ctx, "topp", "datastore")
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}
	ds.Description = "updated"
	if err := s.UpdateStore(ctx, "topp", "pg", ds); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteStore(ctx, "topp", "pg", false); err != nil {
		t.Fatal(err)
	}
}
```

Run → FAIL `undefined: s.CreateStore`

- [ ] **Step 2: Implement store CRUD (append to store.go)**

```go
func (s *Store) CreateStore(ctx context.Context, st model.Store) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO stores(workspace, name, kind, type, enabled, description, connection)
		 VALUES($1,$2,$3,$4,$5,$6,$7)`,
		st.Workspace, st.Name, st.Kind, st.Type, st.Enabled, st.Description, st.Connection)
	return mapErr(err)
}

func (s *Store) GetStore(ctx context.Context, ws, name, kind string) (model.Store, error) {
	var st model.Store
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, kind, type, enabled, description, connection
		 FROM stores WHERE workspace=$1 AND name=$2 AND kind=$3`, ws, name, kind,
	).Scan(&st.Workspace, &st.Name, &st.Kind, &st.Type, &st.Enabled, &st.Description, &st.Connection)
	return st, mapErr(err)
}

func (s *Store) ListStores(ctx context.Context, ws, kind string) ([]model.Store, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, kind, type, enabled, description, connection
		 FROM stores WHERE workspace=$1 AND kind=$2 ORDER BY name`, ws, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Store
	for rows.Next() {
		var st model.Store
		if err := rows.Scan(&st.Workspace, &st.Name, &st.Kind, &st.Type,
			&st.Enabled, &st.Description, &st.Connection); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStore(ctx context.Context, ws, name string, st model.Store) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE stores SET type=$4, enabled=$5, description=$6, connection=$7
		 WHERE workspace=$1 AND name=$2 AND kind=$3`,
		ws, name, st.Kind, st.Type, st.Enabled, st.Description, st.Connection)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteStore(ctx context.Context, ws, name string, recurse bool) error {
	if !recurse {
		var n int
		if err := s.db.QueryRow(ctx,
			`SELECT count(*) FROM resources WHERE workspace=$1 AND store=$2`,
			ws, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM stores WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

Run → PASS

- [ ] **Step 3: Failing REST test**

`services/catalog/internal/rest/stores_test.go`:

```go
package rest

import (
	"strings"
	"testing"
)

func TestDatastoreRESTLifecycle(t *testing.T) {
	mux, _ := testMux(t)
	do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>topp</name></workspace>`)

	rec := do(t, mux, "POST", "/rest/workspaces/topp/datastores", "application/xml",
		`<dataStore><name>pg</name><type>PostGIS</type><enabled>true</enabled>
		 <connectionParameters><entry key="host">postgres</entry>
		 <entry key="port">5432</entry></connectionParameters></dataStore>`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores/pg", "", "")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `<entry key="host">postgres</entry>`) {
		t.Fatalf("GET xml = %d %s", rec.Code, body)
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores/pg.json", "", "")
	body = rec.Body.String()
	if !strings.Contains(body, `"@key":"host"`) || !strings.Contains(body, `"$":"postgres"`) {
		t.Fatalf("GET json = %s", body)
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores.json", "", "")
	if !strings.Contains(rec.Body.String(), `"dataStores"`) {
		t.Fatalf("list json = %s", rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/coveragestores/pg", "", "")
	if rec.Code != 404 {
		t.Fatalf("cross-kind GET = %d", rec.Code)
	}
	rec = do(t, mux, "DELETE", "/rest/workspaces/topp/datastores/pg", "", "")
	if rec.Code != 200 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
}
```

Run → FAIL (404, routes missing)

- [ ] **Step 4: Implement stores.go**

```go
package rest

import (
	"net/http"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

// GeoServer connectionParameters wire formats.
type cpEntryXML struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}
type storeXML struct {
	XMLName     struct{}     `xml:"dataStore"`
	Name        string       `xml:"name"`
	Type        string       `xml:"type,omitempty"`
	Enabled     bool         `xml:"enabled"`
	Description string       `xml:"description,omitempty"`
	Params      []cpEntryXML `xml:"connectionParameters>entry"`
}
type covStoreXML struct {
	XMLName     struct{} `xml:"coverageStore"`
	Name        string   `xml:"name"`
	Type        string   `xml:"type,omitempty"`
	Enabled     bool     `xml:"enabled"`
	Description string   `xml:"description,omitempty"`
	URL         string   `xml:"url,omitempty"`
}
type cpEntryJSON struct {
	Key   string `json:"@key"`
	Value string `json:"$"`
}
type storeBodyJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Params      *struct {
		Entry []cpEntryJSON `json:"entry"`
	} `json:"connectionParameters,omitempty"`
}
type dsJSON struct {
	DataStore storeBodyJSON `json:"dataStore"`
}
type csJSON struct {
	CoverageStore storeBodyJSON `json:"coverageStore"`
}

func connToEntries(conn map[string]string) []cpEntryXML {
	out := make([]cpEntryXML, 0, len(conn))
	for k, v := range conn {
		out = append(out, cpEntryXML{Key: k, Value: v})
	}
	return out
}

func (a *api) storeRoutes(mux *http.ServeMux) {
	for _, kind := range []string{"datastores", "coveragestores"} {
		k := map[string]string{"datastores": "datastore", "coveragestores": "coveragestore"}[kind]
		mux.HandleFunc("GET /rest/workspaces/{ws}/"+kind, a.listStoresH(k))
		mux.HandleFunc("GET /rest/workspaces/{ws}/"+kind+".json", a.listStoresH(k))
		mux.HandleFunc("POST /rest/workspaces/{ws}/"+kind, a.createStoreH(k))
		mux.HandleFunc("POST /rest/workspaces/{ws}/"+kind+".json", a.createStoreH(k))
		mux.HandleFunc("GET /rest/workspaces/{ws}/"+kind+"/{name}", a.getStoreH(k))
		mux.HandleFunc("PUT /rest/workspaces/{ws}/"+kind+"/{name}", a.updateStoreH(k))
		mux.HandleFunc("DELETE /rest/workspaces/{ws}/"+kind+"/{name}", a.deleteStoreH(k))
	}
}

func (a *api) readStore(r *http.Request, kind string) (model.Store, error) {
	st := model.Store{Kind: kind, Enabled: true}
	if kind == "datastore" {
		var x storeXML
		var j dsJSON
		if err := readPayload(r, &x, &j); err != nil {
			return st, err
		}
		b := j.DataStore
		if b.Name == "" { // XML path
			st.Name, st.Type, st.Enabled, st.Description = x.Name, x.Type, x.Enabled, x.Description
			st.Connection = map[string]string{}
			for _, e := range x.Params {
				st.Connection[e.Key] = e.Value
			}
			return st, nil
		}
		st.Name, st.Type, st.Enabled, st.Description = b.Name, b.Type, b.Enabled, b.Description
		st.Connection = map[string]string{}
		if b.Params != nil {
			for _, e := range b.Params.Entry {
				st.Connection[e.Key] = e.Value
			}
		}
		return st, nil
	}
	var x covStoreXML
	var j csJSON
	if err := readPayload(r, &x, &j); err != nil {
		return st, err
	}
	b := j.CoverageStore
	if b.Name == "" {
		st.Name, st.Type, st.Enabled, st.Description = x.Name, x.Type, x.Enabled, x.Description
		st.Connection = map[string]string{"url": x.URL}
		return st, nil
	}
	st.Name, st.Type, st.Enabled, st.Description = b.Name, b.Type, b.Enabled, b.Description
	st.Connection = map[string]string{"url": b.URL}
	return st, nil
}

func (a *api) createStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws := r.PathValue("ws")
		st, err := a.readStore(r, kind)
		if err != nil || st.Name == "" {
			http.Error(w, "invalid store body", http.StatusBadRequest)
			return
		}
		st.Workspace = ws
		if err := a.s.CreateStore(r.Context(), st); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.store.created",
			map[string]string{"name": st.Name, "workspace": ws})
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(st.Name))
	}
}

func (a *api) getStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, name := r.PathValue("ws"), trimFormat(r.PathValue("name"))
		st, err := a.s.GetStore(r.Context(), ws, name, kind)
		if err != nil {
			httpErr(w, err)
			return
		}
		writeStore(w, r, st)
	}
}

func writeStore(w http.ResponseWriter, r *http.Request, st model.Store) {
	body := storeBodyJSON{Name: st.Name, Type: st.Type, Enabled: st.Enabled, Description: st.Description}
	entries := []cpEntryJSON{}
	for k, v := range st.Connection {
		entries = append(entries, cpEntryJSON{Key: k, Value: v})
	}
	body.Params = &struct {
		Entry []cpEntryJSON `json:"entry"`
	}{Entry: entries}
	if st.Kind == "coveragestore" {
		body.URL = st.Connection["url"]
		writePayload(w, r, covStoreXML{Name: st.Name, Type: st.Type, Enabled: st.Enabled,
			Description: st.Description, URL: st.Connection["url"]}, csJSON{CoverageStore: body})
		return
	}
	writePayload(w, r, storeXML{Name: st.Name, Type: st.Type, Enabled: st.Enabled,
		Description: st.Description, Params: connToEntries(st.Connection)}, dsJSON{DataStore: body})
}

func (a *api) listStoresH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws := trimFormat(r.PathValue("ws"))
		list, err := a.s.ListStores(r.Context(), ws, kind)
		if err != nil {
			httpErr(w, err)
			return
		}
		type ref struct {
			Name string `json:"name" xml:"name"`
		}
		refs := []ref{}
		for _, st := range list {
			refs = append(refs, ref{Name: st.Name})
		}
		if kind == "coveragestore" {
			type listXML struct {
				XMLName struct{} `xml:"coverageStores"`
				Items   []ref    `xml:"coverageStore"`
			}
			writePayload(w, r, listXML{Items: refs},
				map[string]any{"coverageStores": map[string]any{"coverageStore": refs}})
			return
		}
		type listXML struct {
			XMLName struct{} `xml:"dataStores"`
			Items   []ref    `xml:"dataStore"`
		}
		writePayload(w, r, listXML{Items: refs},
			map[string]any{"dataStores": map[string]any{"dataStore": refs}})
	}
}

func (a *api) updateStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, name := r.PathValue("ws"), trimFormat(r.PathValue("name"))
		st, err := a.readStore(r, kind)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.s.UpdateStore(r.Context(), ws, name, st); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.store.updated",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}

func (a *api) deleteStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, name := r.PathValue("ws"), trimFormat(r.PathValue("name"))
		recurse := r.URL.Query().Get("recurse") == "true"
		if err := a.s.DeleteStore(r.Context(), ws, name, recurse); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.store.deleted",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}
```

Register in `rest.go` `Mount`: add `a.storeRoutes(inner)` after `a.workspaceRoutes(inner)`.

- [ ] **Step 5: Run tests → PASS; commit**

```bash
GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./...
git add services/catalog
git commit -m "feat(catalog): datastore/coveragestore repository and /rest endpoints"
```

---

### Task 5: Connector registry + PostGIS connector

**Files:**
- Create: `services/catalog/internal/connect/connect.go`, `services/catalog/internal/connect/postgis.go`, `services/catalog/internal/connect/postgis_test.go`

**Interfaces:**
- Consumes: `model.Store` (Task 2), test Postgres (has PostGIS extension — compose image is postgis).
- Produces (package `connect`):

```go
type ResourceInfo struct {
	Name         string // table / layer name
	GeometryType string // e.g. Point, MultiPolygon, Raster, ""
	SRS          string // e.g. EPSG:4326
}

type Connector interface {
	// Validate checks the connection params are usable (connect/open).
	Validate(ctx context.Context, st model.Store) error
	// Introspect lists publishable resources in the store.
	Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error)
}

// ForType returns the connector for a store type ("PostGIS", "Shapefile",
// "GeoPackage", "GeoJSON", "GeoTIFF", "GeoParquet"). Unknown -> error.
func ForType(storeType string) (Connector, error)
```

- [ ] **Step 1: Failing PostGIS connector test**

`services/catalog/internal/connect/postgis_test.go`:

```go
package connect

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/geoson/geoson/services/catalog/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

// storeFromDSN converts GEOSON_TEST_DATABASE_URL into PostGIS connection params.
func storeFromDSN(t *testing.T) (model.Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("GEOSON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GEOSON_TEST_DATABASE_URL not set")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pw, _ := u.User.Password()
	st := model.Store{Type: "PostGIS", Connection: map[string]string{
		"host":     u.Hostname(),
		"port":     u.Port(),
		"database": strings.TrimPrefix(u.Path, "/"),
		"user":     u.User.Username(),
		"passwd":   pw,
	}}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return st, pool
}

func TestPostGISValidateAndIntrospect(t *testing.T) {
	st, pool := storeFromDSN(t)
	ctx := context.Background()
	pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis`)
	pool.Exec(ctx, `DROP TABLE IF EXISTS conn_test_roads`)
	if _, err := pool.Exec(ctx,
		`CREATE TABLE conn_test_roads (id serial PRIMARY KEY, geom geometry(LineString, 3857))`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DROP TABLE conn_test_roads`) })

	c, err := ForType("PostGIS")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(ctx, st); err != nil {
		t.Fatalf("validate: %v", err)
	}
	bad := st
	bad.Connection = map[string]string{"host": "127.0.0.1", "port": "1", "database": "x", "user": "x", "passwd": "x"}
	if err := c.Validate(ctx, bad); err == nil {
		t.Fatal("validate bad params: want error")
	}
	infos, err := c.Introspect(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	var found *ResourceInfo
	for i := range infos {
		if infos[i].Name == "conn_test_roads" {
			found = &infos[i]
		}
	}
	if found == nil || found.GeometryType != "LineString" || found.SRS != "EPSG:3857" {
		t.Fatalf("introspect = %+v", infos)
	}
}

func TestForTypeUnknown(t *testing.T) {
	if _, err := ForType("Oracle"); err == nil {
		t.Fatal("want error for unknown type")
	}
}
```

Run → FAIL `undefined: ForType`

- [ ] **Step 2: Implement connect.go**

```go
// Package connect holds store connectors: connection validation and
// resource introspection per store type.
package connect

import (
	"context"
	"fmt"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

type ResourceInfo struct {
	Name         string
	GeometryType string
	SRS          string
}

type Connector interface {
	Validate(ctx context.Context, st model.Store) error
	Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error)
}

var registry = map[string]Connector{}

func register(storeType string, c Connector) { registry[storeType] = c }

func ForType(storeType string) (Connector, error) {
	c, ok := registry[storeType]
	if !ok {
		return nil, fmt.Errorf("unsupported store type %q", storeType)
	}
	return c, nil
}
```

- [ ] **Step 3: Implement postgis.go**

```go
package connect

import (
	"context"
	"fmt"
	"time"

	"github.com/geoson/geoson/services/catalog/internal/model"
	"github.com/jackc/pgx/v5"
)

func init() { register("PostGIS", postgis{}) }

type postgis struct{}

// dsn builds a pgx DSN from GeoServer-style connection params
// (host, port, database, user, passwd, schema).
func (postgis) dsn(st model.Store) string {
	c := st.Connection
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		c["user"], c["passwd"], c["host"], c["port"], c["database"])
}

func (p postgis) Validate(ctx context.Context, st model.Store) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, p.dsn(st))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	return conn.Ping(ctx)
}

func (p postgis) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	conn, err := pgx.Connect(ctx, p.dsn(st))
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	schema := st.Connection["schema"]
	if schema == "" {
		schema = "public"
	}
	rows, err := conn.Query(ctx, `
		SELECT f_table_name, type, 'EPSG:' || srid
		FROM geometry_columns
		WHERE f_table_schema = $1
		ORDER BY f_table_name`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceInfo
	for rows.Next() {
		var ri ResourceInfo
		if err := rows.Scan(&ri.Name, &ri.GeometryType, &ri.SRS); err != nil {
			return nil, err
		}
		out = append(out, ri)
	}
	return out, rows.Err()
}
```

Note: `geometry_columns` reports type in upper case (`LINESTRING`). GeoServer uses CamelCase. Normalize in Introspect before returning:

```go
		ri.GeometryType = normalizeGeomType(ri.GeometryType)
```

with:

```go
var geomTypes = map[string]string{
	"POINT": "Point", "MULTIPOINT": "MultiPoint",
	"LINESTRING": "LineString", "MULTILINESTRING": "MultiLineString",
	"POLYGON": "Polygon", "MULTIPOLYGON": "MultiPolygon",
	"GEOMETRY": "Geometry", "GEOMETRYCOLLECTION": "GeometryCollection",
}

func normalizeGeomType(t string) string {
	if v, ok := geomTypes[t]; ok {
		return v
	}
	return t
}
```

- [ ] **Step 4: Run tests → PASS; commit**

```bash
GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./internal/connect/
git add services/catalog/internal/connect
git commit -m "feat(catalog): connector registry and PostGIS introspection"
```

---

### Task 6: File connectors (Shapefile/GeoPackage/GeoJSON), COG, GeoParquet(DuckDB)

**Files:**
- Create: `services/catalog/internal/connect/files.go`, `services/catalog/internal/connect/cog.go`, `services/catalog/internal/connect/geoparquet.go`, `services/catalog/internal/connect/files_test.go`
- Create: `tests/testdata/` fixtures (generated in Step 1)
- Modify: `services/catalog/go.mod` (add go-duckdb), `services/catalog/Dockerfile` (no change needed — already debian)

**Interfaces:**
- Consumes: registry from Task 5. Store connection params: file stores use `Connection["url"]` = `file:///abs/path` or plain path (GeoServer uses `url` param for shapefile stores).
- Produces: registered connectors `"Shapefile"`, `"GeoPackage"`, `"GeoJSON"`, `"GeoTIFF"`, `"GeoParquet"`. Validation level for Sprint 2: file exists + format magic-byte check; Introspect returns one ResourceInfo derived from filename (deep schema readers land in WFS/WMS sprints). GeoParquet Introspect is real: DuckDB reads parquet schema + geo metadata.

- [ ] **Step 1: Create test fixtures**

```bash
mkdir -p /home/madson/geoson/tests/testdata
cd /home/madson/geoson/tests/testdata
# GeoJSON
cat > points.geojson << 'EOF'
{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[51.4,35.7]},"properties":{"name":"tehran"}}]}
EOF
# Minimal valid single-record shapefile + gpkg + geoparquet via python/gdal if available,
# else via duckdb CLI for parquet and raw bytes for shp/gpkg headers:
python3 - << 'EOF'
import struct
# minimal .shp: 100-byte header, file code 9994 big-endian, shape type 1 (point), one null record skipped
h = struct.pack('>i', 9994) + b'\x00'*20 + struct.pack('>i', 50)
h += struct.pack('<i', 1000) + struct.pack('<i', 1) + struct.pack('<8d', 0,0,0,0,0,0,0,0)
open('roads.shp','wb').write(h)
# minimal gpkg: sqlite header string
open('data.gpkg','wb').write(b'SQLite format 3\x00' + b'\x00'*84)
# minimal tiff: little-endian magic + IFD offset
open('dem.tif','wb').write(b'II*\x00\x08\x00\x00\x00' + b'\x00'*8)
EOF
# GeoParquet via duckdb (spatial extension writes GeoParquet metadata)
docker run --rm -v "$PWD":/data datacatering/duckdb:v1.3.2 \
  -c "INSTALL spatial; LOAD spatial; COPY (SELECT 1 AS id, ST_Point(51.4, 35.7) AS geometry) TO '/data/places.parquet' (FORMAT PARQUET);"
ls -la
```

(If the duckdb docker image is unavailable, install duckdb CLI locally: `curl -fsSL https://install.duckdb.org | sh`.)

- [ ] **Step 2: Failing connector tests**

`services/catalog/internal/connect/files_test.go`:

```go
package connect

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "tests", "testdata", name)
}

func fileStore(typ, path string) model.Store {
	return model.Store{Type: typ, Connection: map[string]string{"url": path}}
}

func TestFileConnectorsValidate(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		typ, file string
		wantName  string
	}{
		{"Shapefile", "roads.shp", "roads"},
		{"GeoPackage", "data.gpkg", "data"},
		{"GeoJSON", "points.geojson", "points"},
		{"GeoTIFF", "dem.tif", "dem"},
	}
	for _, tc := range cases {
		c, err := ForType(tc.typ)
		if err != nil {
			t.Fatalf("%s: %v", tc.typ, err)
		}
		st := fileStore(tc.typ, testdata(t, tc.file))
		if err := c.Validate(ctx, st); err != nil {
			t.Fatalf("%s validate: %v", tc.typ, err)
		}
		if err := c.Validate(ctx, fileStore(tc.typ, testdata(t, "missing.bin"))); err == nil {
			t.Fatalf("%s: want error for missing file", tc.typ)
		}
		// wrong magic: geojson file fed to binary formats must fail
		if tc.typ != "GeoJSON" {
			if err := c.Validate(ctx, fileStore(tc.typ, testdata(t, "points.geojson"))); err == nil {
				t.Fatalf("%s: want magic-byte error", tc.typ)
			}
		}
		infos, err := c.Introspect(ctx, st)
		if err != nil || len(infos) != 1 || infos[0].Name != tc.wantName {
			t.Fatalf("%s introspect = %+v, %v", tc.typ, infos, err)
		}
	}
}

func TestGeoParquetIntrospect(t *testing.T) {
	ctx := context.Background()
	c, err := ForType("GeoParquet")
	if err != nil {
		t.Fatal(err)
	}
	st := fileStore("GeoParquet", testdata(t, "places.parquet"))
	if err := c.Validate(ctx, st); err != nil {
		t.Fatalf("validate: %v", err)
	}
	infos, err := c.Introspect(ctx, st)
	if err != nil || len(infos) != 1 || infos[0].Name != "places" {
		t.Fatalf("introspect = %+v, %v", infos, err)
	}
}
```

Run → FAIL (connectors not registered)

- [ ] **Step 3: Implement files.go + cog.go**

`files.go`:

```go
package connect

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

func init() {
	register("Shapefile", fileConn{ext: ".shp", magic: []byte{0x00, 0x00, 0x27, 0x0a}, magicAt: 0})
	register("GeoPackage", fileConn{ext: ".gpkg", magic: []byte("SQLite format 3\x00"), magicAt: 0})
	register("GeoJSON", fileConn{ext: ".geojson", jsonCheck: true})
}

// fileConn validates file-backed stores by existence + magic bytes.
// Deep schema introspection is done by wfs/wms at read time.
type fileConn struct {
	ext       string
	magic     []byte
	magicAt   int
	jsonCheck bool
}

func storePath(st model.Store) string {
	return strings.TrimPrefix(st.Connection["url"], "file://")
}

func (f fileConn) Validate(ctx context.Context, st model.Store) error {
	path := storePath(st)
	head := make([]byte, f.magicAt+64)
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	n, _ := fd.Read(head)
	head = head[:n]
	if f.jsonCheck {
		trimmed := bytes.TrimLeft(head, " \t\r\n")
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return fmt.Errorf("%s: not a JSON document", path)
		}
		return nil
	}
	if len(head) < f.magicAt+len(f.magic) ||
		!bytes.Equal(head[f.magicAt:f.magicAt+len(f.magic)], f.magic) {
		return fmt.Errorf("%s: bad magic for %s", path, f.ext)
	}
	return nil
}

func (f fileConn) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := f.Validate(ctx, st); err != nil {
		return nil, err
	}
	path := storePath(st)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return []ResourceInfo{{Name: name, SRS: "EPSG:4326"}}, nil
}
```

Note shapefile magic: big-endian int32 9994 = bytes `00 00 27 0a`.

`cog.go`:

```go
package connect

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

func init() { register("GeoTIFF", cogConn{}) }

type cogConn struct{}

// Validate accepts classic TIFF and BigTIFF, either byte order.
func (cogConn) Validate(ctx context.Context, st model.Store) error {
	path := storePath(st)
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	head := make([]byte, 4)
	if _, err := fd.Read(head); err != nil {
		return err
	}
	ok := bytes.Equal(head, []byte{'I', 'I', 42, 0}) || // little-endian
		bytes.Equal(head, []byte{'M', 'M', 0, 42}) || // big-endian
		bytes.Equal(head, []byte{'I', 'I', 43, 0}) || // BigTIFF LE
		bytes.Equal(head, []byte{'M', 'M', 0, 43}) // BigTIFF BE
	if !ok {
		return fmt.Errorf("%s: not a TIFF", path)
	}
	return nil
}

func (c cogConn) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := c.Validate(ctx, st); err != nil {
		return nil, err
	}
	path := storePath(st)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return []ResourceInfo{{Name: name, GeometryType: "Raster", SRS: "EPSG:4326"}}, nil
}
```

- [ ] **Step 4: Implement geoparquet.go (DuckDB)**

```bash
cd /home/madson/geoson/services/catalog && go get github.com/marcboeker/go-duckdb/v2@latest
```

```go
package connect

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoson/geoson/services/catalog/internal/model"
	_ "github.com/marcboeker/go-duckdb/v2"
)

func init() { register("GeoParquet", geoparquet{}) }

type geoparquet struct{}

func (geoparquet) open() (*sql.DB, error) { return sql.Open("duckdb", "") }

func (g geoparquet) Validate(ctx context.Context, st model.Store) error {
	db, err := g.open()
	if err != nil {
		return err
	}
	defer db.Close()
	path := storePath(st)
	// parquet_schema errors on non-parquet input
	var count int
	err = db.QueryRowContext(ctx,
		`SELECT count(*) FROM parquet_schema($1)`, path).Scan(&count)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if count == 0 {
		return fmt.Errorf("%s: empty parquet schema", path)
	}
	return nil
}

func (g geoparquet) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := g.Validate(ctx, st); err != nil {
		return nil, err
	}
	path := storePath(st)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	db, err := g.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	// Detect geometry column: GeoParquet stores geo metadata in file KV;
	// fall back to a column literally named "geometry"/"geom".
	ri := ResourceInfo{Name: name, SRS: "EPSG:4326"}
	rows, err := db.QueryContext(ctx, `SELECT name FROM parquet_schema($1)`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		if col == "geometry" || col == "geom" {
			ri.GeometryType = "Geometry"
		}
	}
	return []ResourceInfo{ri}, rows.Err()
}
```

- [ ] **Step 5: Run tests → PASS (go-duckdb downloads prebuilt lib; needs cgo)**

```bash
cd /home/madson/geoson/services/catalog && go mod tidy
GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./internal/connect/ -v
```
Expected: PASS. If go-duckdb fails to link in CI, add `CGO_ENABLED=1` and gcc to the go job.

- [ ] **Step 6: Wire store validation into REST create (stores.go)**

In `createStoreH`, before `a.s.CreateStore`, add non-fatal validation:

```go
		if c, cerr := connect.ForType(st.Type); cerr == nil {
			if verr := c.Validate(r.Context(), st); verr != nil {
				http.Error(w, "store validation failed: "+verr.Error(), http.StatusBadRequest)
				return
			}
		}
```

(import `github.com/geoson/geoson/services/catalog/internal/connect`; unknown types pass through — GeoServer also allows saving stores it can't reach only when `enabled=false`, we keep it strict for known types.)

Add REST test in `stores_test.go`:

```go
func TestDatastoreCreateValidatesConnection(t *testing.T) {
	mux, _ := testMux(t)
	do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>vt</name></workspace>`)
	rec := do(t, mux, "POST", "/rest/workspaces/vt/datastores", "application/xml",
		`<dataStore><name>bad</name><type>PostGIS</type><enabled>true</enabled>
		 <connectionParameters><entry key="host">127.0.0.1</entry><entry key="port">1</entry>
		 <entry key="database">x</entry><entry key="user">x</entry><entry key="passwd">x</entry>
		 </connectionParameters></dataStore>`)
	if rec.Code != 400 {
		t.Fatalf("bad store POST = %d %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 7: Commit**

```bash
git add services/catalog tests/testdata
git commit -m "feat(catalog): file, COG and GeoParquet(DuckDB) connectors with store validation"
```

---

### Task 7: FeatureTypes, coverages, layers, styles, layergroups (repo + /rest)

**Files:**
- Create: `services/catalog/internal/rest/layers.go`, `services/catalog/internal/rest/layers_test.go`
- Modify: `services/catalog/internal/store/store.go` (resource/layer/style/layergroup CRUD), `services/catalog/internal/store/store_test.go`, `services/catalog/internal/rest/rest.go`

**Interfaces:**
- Consumes: everything above.
- Produces (`store.Store` methods; all follow the exact Create/Get/List/Update/Delete signature pattern of workspaces/stores):
  - `CreateResource(ctx, model.FeatureType or model.Coverage → stored via kind) error` — implement as `CreateFeatureType(ctx, model.FeatureType) error`, `CreateCoverage(ctx, model.Coverage) error`, `GetFeatureType(ctx, ws, store, name string)`, `ListFeatureTypes(ctx, ws, store string)`, `DeleteFeatureType(ctx, ws, store, name string)` (same trio for Coverage)
  - `CreateLayer/GetLayer/ListLayers/UpdateLayer/DeleteLayer` over `model.Layer` (keyed ws+name; ListLayers(ctx) lists all for global /rest/layers)
  - `CreateStyle/GetStyle/ListStyles/UpdateStyle/DeleteStyle` over `model.Style` (ws may be empty = global)
  - `CreateLayerGroup/GetLayerGroup/ListLayerGroups/DeleteLayerGroup` over `model.LayerGroup`
- REST endpoints (GeoServer paths):
  - `GET/POST /rest/workspaces/{ws}/datastores/{ds}/featuretypes`, `GET/DELETE .../featuretypes/{ft}`
  - `GET/POST /rest/workspaces/{ws}/coveragestores/{cs}/coverages`, `GET/DELETE .../coverages/{c}`
  - `GET /rest/layers`, `GET/PUT/DELETE /rest/layers/{layer}` (also `/rest/workspaces/{ws}/layers/{l}`)
  - `GET/POST /rest/styles` + `/rest/workspaces/{ws}/styles`; `GET/PUT/DELETE /rest/styles/{s}` — POST body may be SLD XML (`Content-Type: application/vnd.ogc.sld+xml`, `?name=` query) or metadata XML/JSON; GET returns metadata, `GET /rest/styles/{s}.sld` returns body
  - `GET/POST /rest/layergroups` + workspace-scoped; `GET/DELETE /rest/layergroups/{lg}`
- Behavior: creating a featuretype auto-creates a VECTOR layer of same name with default style `generic` (GeoServer behavior); creating a coverage auto-creates RASTER layer with style `raster`.
- Seed styles: migration `0002_seed_styles.sql` inserts global styles `generic`, `point`, `line`, `polygon`, `raster` with minimal SLD bodies.

- [ ] **Step 1: Write migration 0002_seed_styles.sql**

`services/catalog/internal/store/migrations/0002_seed_styles.sql`:

```sql
INSERT INTO styles(workspace, name, format, filename, body) VALUES
('', 'generic', 'sld', 'generic.sld', '<?xml version="1.0" encoding="UTF-8"?><StyledLayerDescriptor version="1.0.0" xmlns="http://www.opengis.net/sld"><NamedLayer><Name>generic</Name><UserStyle><FeatureTypeStyle><Rule><PointSymbolizer><Graphic><Mark><WellKnownName>square</WellKnownName><Fill><CssParameter name="fill">#808080</CssParameter></Fill></Mark><Size>6</Size></Graphic></PointSymbolizer><LineSymbolizer><Stroke><CssParameter name="stroke">#303030</CssParameter></Stroke></LineSymbolizer><PolygonSymbolizer><Fill><CssParameter name="fill">#AAAAAA</CssParameter></Fill><Stroke><CssParameter name="stroke">#000000</CssParameter></Stroke></PolygonSymbolizer></Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>'),
('', 'point', 'sld', 'point.sld', '<?xml version="1.0" encoding="UTF-8"?><StyledLayerDescriptor version="1.0.0" xmlns="http://www.opengis.net/sld"><NamedLayer><Name>point</Name><UserStyle><FeatureTypeStyle><Rule><PointSymbolizer><Graphic><Mark><WellKnownName>circle</WellKnownName><Fill><CssParameter name="fill">#FF0000</CssParameter></Fill></Mark><Size>6</Size></Graphic></PointSymbolizer></Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>'),
('', 'line', 'sld', 'line.sld', '<?xml version="1.0" encoding="UTF-8"?><StyledLayerDescriptor version="1.0.0" xmlns="http://www.opengis.net/sld"><NamedLayer><Name>line</Name><UserStyle><FeatureTypeStyle><Rule><LineSymbolizer><Stroke><CssParameter name="stroke">#0000FF</CssParameter></Stroke></LineSymbolizer></Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>'),
('', 'polygon', 'sld', 'polygon.sld', '<?xml version="1.0" encoding="UTF-8"?><StyledLayerDescriptor version="1.0.0" xmlns="http://www.opengis.net/sld"><NamedLayer><Name>polygon</Name><UserStyle><FeatureTypeStyle><Rule><PolygonSymbolizer><Fill><CssParameter name="fill">#AAAAAA</CssParameter></Fill><Stroke><CssParameter name="stroke">#000000</CssParameter></Stroke></PolygonSymbolizer></Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>'),
('', 'raster', 'sld', 'raster.sld', '<?xml version="1.0" encoding="UTF-8"?><StyledLayerDescriptor version="1.0.0" xmlns="http://www.opengis.net/sld"><NamedLayer><Name>raster</Name><UserStyle><FeatureTypeStyle><Rule><RasterSymbolizer><Opacity>1.0</Opacity></RasterSymbolizer></Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>')
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Failing repo tests (append to store_test.go)**

```go
func TestFeatureTypeLayerStyleLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	s.CreateWorkspace(ctx, model.Workspace{Name: "topp"})
	s.CreateStore(ctx, model.Store{Workspace: "topp", Name: "pg", Kind: "datastore", Type: "PostGIS", Enabled: true})

	ft := model.FeatureType{Workspace: "topp", Store: "pg", Name: "roads",
		NativeName: "roads", SRS: "EPSG:3857", Enabled: true}
	if err := s.CreateFeatureType(ctx, ft); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFeatureType(ctx, "topp", "pg", "roads")
	if err != nil || got.SRS != "EPSG:3857" {
		t.Fatalf("get ft = %+v, %v", got, err)
	}
	fts, err := s.ListFeatureTypes(ctx, "topp", "pg")
	if err != nil || len(fts) != 1 {
		t.Fatalf("list fts = %v, %v", fts, err)
	}

	// seeded styles present
	st, err := s.GetStyle(ctx, "", "generic")
	if err != nil || st.Body == "" {
		t.Fatalf("seed style = %+v, %v", st, err)
	}

	if err := s.CreateLayer(ctx, model.Layer{Workspace: "topp", Name: "roads",
		Type: "VECTOR", ResourceName: "roads", DefaultStyle: "generic", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	l, err := s.GetLayer(ctx, "topp", "roads")
	if err != nil || l.DefaultStyle != "generic" {
		t.Fatalf("layer = %+v, %v", l, err)
	}

	if err := s.CreateLayerGroup(ctx, model.LayerGroup{Workspace: "topp",
		Name: "basemap", Mode: "SINGLE", Layers: []string{"roads"}}); err != nil {
		t.Fatal(err)
	}
	lg, err := s.GetLayerGroup(ctx, "topp", "basemap")
	if err != nil || len(lg.Layers) != 1 {
		t.Fatalf("layergroup = %+v, %v", lg, err)
	}

	if err := s.DeleteFeatureType(ctx, "topp", "pg", "roads"); err != nil {
		t.Fatal(err)
	}
}
```

Run → FAIL `undefined: s.CreateFeatureType`

- [ ] **Step 3: Implement repo methods (append to store.go)**

```go
func (s *Store) CreateFeatureType(ctx context.Context, ft model.FeatureType) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO resources(workspace, store, name, kind, native_name, title, srs, enabled)
		 VALUES($1,$2,$3,'featuretype',$4,$5,$6,$7)`,
		ft.Workspace, ft.Store, ft.Name, ft.NativeName, ft.Title, ft.SRS, ft.Enabled)
	return mapErr(err)
}

func (s *Store) GetFeatureType(ctx context.Context, ws, st, name string) (model.FeatureType, error) {
	var ft model.FeatureType
	err := s.db.QueryRow(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='featuretype'`,
		ws, st, name,
	).Scan(&ft.Workspace, &ft.Store, &ft.Name, &ft.NativeName, &ft.Title, &ft.SRS, &ft.Enabled)
	return ft, mapErr(err)
}

func (s *Store) ListFeatureTypes(ctx context.Context, ws, st string) ([]model.FeatureType, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND kind='featuretype' ORDER BY name`, ws, st)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FeatureType
	for rows.Next() {
		var ft model.FeatureType
		if err := rows.Scan(&ft.Workspace, &ft.Store, &ft.Name, &ft.NativeName,
			&ft.Title, &ft.SRS, &ft.Enabled); err != nil {
			return nil, err
		}
		out = append(out, ft)
	}
	return out, rows.Err()
}

func (s *Store) DeleteFeatureType(ctx context.Context, ws, st, name string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='featuretype'`,
		ws, st, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Coverages: identical shape over kind='coverage'.
func (s *Store) CreateCoverage(ctx context.Context, c model.Coverage) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO resources(workspace, store, name, kind, native_name, title, srs, enabled)
		 VALUES($1,$2,$3,'coverage',$4,$5,$6,$7)`,
		c.Workspace, c.Store, c.Name, c.NativeName, c.Title, c.SRS, c.Enabled)
	return mapErr(err)
}

func (s *Store) GetCoverage(ctx context.Context, ws, st, name string) (model.Coverage, error) {
	var c model.Coverage
	err := s.db.QueryRow(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='coverage'`,
		ws, st, name,
	).Scan(&c.Workspace, &c.Store, &c.Name, &c.NativeName, &c.Title, &c.SRS, &c.Enabled)
	return c, mapErr(err)
}

func (s *Store) ListCoverages(ctx context.Context, ws, st string) ([]model.Coverage, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND kind='coverage' ORDER BY name`, ws, st)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Coverage
	for rows.Next() {
		var c model.Coverage
		if err := rows.Scan(&c.Workspace, &c.Store, &c.Name, &c.NativeName,
			&c.Title, &c.SRS, &c.Enabled); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteCoverage(ctx context.Context, ws, st, name string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='coverage'`,
		ws, st, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateLayer(ctx context.Context, l model.Layer) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO layers(workspace, name, type, resource_name, default_style, enabled)
		 VALUES($1,$2,$3,$4,$5,$6)`,
		l.Workspace, l.Name, l.Type, l.ResourceName, l.DefaultStyle, l.Enabled)
	return mapErr(err)
}

func (s *Store) GetLayer(ctx context.Context, ws, name string) (model.Layer, error) {
	var l model.Layer
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, type, resource_name, default_style, enabled
		 FROM layers WHERE workspace=$1 AND name=$2`, ws, name,
	).Scan(&l.Workspace, &l.Name, &l.Type, &l.ResourceName, &l.DefaultStyle, &l.Enabled)
	return l, mapErr(err)
}

func (s *Store) ListLayers(ctx context.Context) ([]model.Layer, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, type, resource_name, default_style, enabled
		 FROM layers ORDER BY workspace, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Layer
	for rows.Next() {
		var l model.Layer
		if err := rows.Scan(&l.Workspace, &l.Name, &l.Type, &l.ResourceName,
			&l.DefaultStyle, &l.Enabled); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) UpdateLayer(ctx context.Context, ws, name string, l model.Layer) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE layers SET default_style=$3, enabled=$4 WHERE workspace=$1 AND name=$2`,
		ws, name, l.DefaultStyle, l.Enabled)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteLayer(ctx context.Context, ws, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM layers WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateStyle(ctx context.Context, st model.Style) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO styles(workspace, name, format, filename, body) VALUES($1,$2,$3,$4,$5)`,
		st.Workspace, st.Name, st.Format, st.Filename, st.Body)
	return mapErr(err)
}

func (s *Store) GetStyle(ctx context.Context, ws, name string) (model.Style, error) {
	var st model.Style
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, format, filename, body FROM styles WHERE workspace=$1 AND name=$2`,
		ws, name,
	).Scan(&st.Workspace, &st.Name, &st.Format, &st.Filename, &st.Body)
	return st, mapErr(err)
}

func (s *Store) ListStyles(ctx context.Context, ws string) ([]model.Style, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, format, filename, body FROM styles WHERE workspace=$1 ORDER BY name`, ws)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Style
	for rows.Next() {
		var st model.Style
		if err := rows.Scan(&st.Workspace, &st.Name, &st.Format, &st.Filename, &st.Body); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStyle(ctx context.Context, ws, name string, st model.Style) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE styles SET format=$3, filename=$4, body=$5 WHERE workspace=$1 AND name=$2`,
		ws, name, st.Format, st.Filename, st.Body)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteStyle(ctx context.Context, ws, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM styles WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateLayerGroup(ctx context.Context, lg model.LayerGroup) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO layer_groups(workspace, name, mode, layers) VALUES($1,$2,$3,$4)`,
		lg.Workspace, lg.Name, lg.Mode, lg.Layers)
	return mapErr(err)
}

func (s *Store) GetLayerGroup(ctx context.Context, ws, name string) (model.LayerGroup, error) {
	var lg model.LayerGroup
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, mode, layers FROM layer_groups WHERE workspace=$1 AND name=$2`,
		ws, name,
	).Scan(&lg.Workspace, &lg.Name, &lg.Mode, &lg.Layers)
	return lg, mapErr(err)
}

func (s *Store) ListLayerGroups(ctx context.Context, ws string) ([]model.LayerGroup, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, mode, layers FROM layer_groups WHERE workspace=$1 ORDER BY name`, ws)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.LayerGroup
	for rows.Next() {
		var lg model.LayerGroup
		if err := rows.Scan(&lg.Workspace, &lg.Name, &lg.Mode, &lg.Layers); err != nil {
			return nil, err
		}
		out = append(out, lg)
	}
	return out, rows.Err()
}

func (s *Store) DeleteLayerGroup(ctx context.Context, ws, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM layer_groups WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

Run repo tests → PASS

- [ ] **Step 4: Failing REST tests**

`services/catalog/internal/rest/layers_test.go`:

```go
package rest

import (
	"strings"
	"testing"
)

func setupWsAndStore(t *testing.T, mux *http.ServeMux) {
	do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>topp</name></workspace>`)
	do(t, mux, "POST", "/rest/workspaces/topp/datastores", "application/xml",
		`<dataStore><name>files</name><type>Directory</type><enabled>true</enabled>
		 <connectionParameters/></dataStore>`)
}

func TestFeatureTypeAutoCreatesLayer(t *testing.T) {
	mux, _ := testMux(t)
	setupWsAndStore(t, mux)

	rec := do(t, mux, "POST", "/rest/workspaces/topp/datastores/files/featuretypes",
		"application/xml",
		`<featureType><name>roads</name><nativeName>roads</nativeName><srs>EPSG:3857</srs><enabled>true</enabled></featureType>`)
	if rec.Code != 201 {
		t.Fatalf("POST ft = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores/files/featuretypes/roads", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<srs>EPSG:3857</srs>") {
		t.Fatalf("GET ft = %d %s", rec.Code, rec.Body.String())
	}
	// auto-created layer with default style
	rec = do(t, mux, "GET", "/rest/layers/topp:roads", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "generic") {
		t.Fatalf("GET layer = %d %s", rec.Code, rec.Body.String())
	}
}

func TestStyleUploadSLD(t *testing.T) {
	mux, _ := testMux(t)
	sld := `<?xml version="1.0"?><StyledLayerDescriptor version="1.0.0"><NamedLayer><Name>x</Name></NamedLayer></StyledLayerDescriptor>`
	rec := do(t, mux, "POST", "/rest/styles?name=mystyle", "application/vnd.ogc.sld+xml", sld)
	if rec.Code != 201 {
		t.Fatalf("POST sld = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/styles/mystyle.sld", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "StyledLayerDescriptor") {
		t.Fatalf("GET sld = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/styles/mystyle.json", "", "")
	if !strings.Contains(rec.Body.String(), `"name":"mystyle"`) {
		t.Fatalf("GET style json = %s", rec.Body.String())
	}
}

func TestLayerGroupREST(t *testing.T) {
	mux, _ := testMux(t)
	setupWsAndStore(t, mux)
	do(t, mux, "POST", "/rest/workspaces/topp/datastores/files/featuretypes",
		"application/xml", `<featureType><name>roads</name><nativeName>roads</nativeName><enabled>true</enabled></featureType>`)
	rec := do(t, mux, "POST", "/rest/workspaces/topp/layergroups", "application/xml",
		`<layerGroup><name>base</name><mode>SINGLE</mode><layers><layer>roads</layer></layers></layerGroup>`)
	if rec.Code != 201 {
		t.Fatalf("POST lg = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/layergroups/base", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<layer>roads</layer>") {
		t.Fatalf("GET lg = %d %s", rec.Code, rec.Body.String())
	}
}
```

(add `"net/http"` to the test file imports)

Run → FAIL

- [ ] **Step 5: Implement layers.go**

```go
package rest

import (
	"io"
	"net/http"
	"strings"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

type ftXML struct {
	XMLName    struct{} `xml:"featureType"`
	Name       string   `xml:"name"`
	NativeName string   `xml:"nativeName"`
	Title      string   `xml:"title,omitempty"`
	SRS        string   `xml:"srs,omitempty"`
	Enabled    bool     `xml:"enabled"`
}
type ftJSON struct {
	FeatureType struct {
		Name       string `json:"name"`
		NativeName string `json:"nativeName"`
		Title      string `json:"title,omitempty"`
		SRS        string `json:"srs,omitempty"`
		Enabled    bool   `json:"enabled"`
	} `json:"featureType"`
}
type covXML struct {
	XMLName    struct{} `xml:"coverage"`
	Name       string   `xml:"name"`
	NativeName string   `xml:"nativeName"`
	Title      string   `xml:"title,omitempty"`
	SRS        string   `xml:"srs,omitempty"`
	Enabled    bool     `xml:"enabled"`
}
type layerXML struct {
	XMLName      struct{}       `xml:"layer"`
	Name         string         `xml:"name"`
	Type         string         `xml:"type"`
	DefaultStyle *layerStyleRef `xml:"defaultStyle,omitempty"`
}
type layerStyleRef struct {
	Name string `xml:"name"`
}
type styleXML struct {
	XMLName  struct{} `xml:"style"`
	Name     string   `xml:"name"`
	Format   string   `xml:"format,omitempty"`
	Filename string   `xml:"filename,omitempty"`
}
type lgXML struct {
	XMLName struct{} `xml:"layerGroup"`
	Name    string   `xml:"name"`
	Mode    string   `xml:"mode"`
	Layers  []string `xml:"layers>layer"`
}

func (a *api) layerRoutes(mux *http.ServeMux) {
	// featuretypes
	mux.HandleFunc("POST /rest/workspaces/{ws}/datastores/{ds}/featuretypes", a.createFeatureType)
	mux.HandleFunc("GET /rest/workspaces/{ws}/datastores/{ds}/featuretypes", a.listFeatureTypes)
	mux.HandleFunc("GET /rest/workspaces/{ws}/datastores/{ds}/featuretypes/{ft}", a.getFeatureType)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}/datastores/{ds}/featuretypes/{ft}", a.deleteFeatureType)
	// coverages
	mux.HandleFunc("POST /rest/workspaces/{ws}/coveragestores/{cs}/coverages", a.createCoverage)
	mux.HandleFunc("GET /rest/workspaces/{ws}/coveragestores/{cs}/coverages/{c}", a.getCoverage)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}/coveragestores/{cs}/coverages/{c}", a.deleteCoverage)
	// layers
	mux.HandleFunc("GET /rest/layers", a.listLayers)
	mux.HandleFunc("GET /rest/layers/{layer}", a.getLayer)
	mux.HandleFunc("PUT /rest/layers/{layer}", a.updateLayer)
	mux.HandleFunc("DELETE /rest/layers/{layer}", a.deleteLayer)
	// styles (global + workspace)
	mux.HandleFunc("POST /rest/styles", a.createStyle(""))
	mux.HandleFunc("GET /rest/styles", a.listStyles(""))
	mux.HandleFunc("GET /rest/styles/{s}", a.getStyle(""))
	mux.HandleFunc("PUT /rest/styles/{s}", a.updateStyle(""))
	mux.HandleFunc("DELETE /rest/styles/{s}", a.deleteStyle(""))
	mux.HandleFunc("POST /rest/workspaces/{ws}/styles", a.createStyleWS)
	mux.HandleFunc("GET /rest/workspaces/{ws}/styles/{s}", a.getStyleWS)
	// layergroups
	mux.HandleFunc("POST /rest/workspaces/{ws}/layergroups", a.createLayerGroup)
	mux.HandleFunc("GET /rest/workspaces/{ws}/layergroups/{lg}", a.getLayerGroup)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}/layergroups/{lg}", a.deleteLayerGroup)
	mux.HandleFunc("POST /rest/layergroups", a.createLayerGroup)     // ws="" global
	mux.HandleFunc("GET /rest/layergroups/{lg}", a.getLayerGroup)    // ws=""
	mux.HandleFunc("DELETE /rest/layergroups/{lg}", a.deleteLayerGroup)
}

func (a *api) createFeatureType(w http.ResponseWriter, r *http.Request) {
	ws, ds := r.PathValue("ws"), r.PathValue("ds")
	var x ftXML
	var j ftJSON
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ft := model.FeatureType{Workspace: ws, Store: ds,
		Name: x.Name, NativeName: x.NativeName, Title: x.Title, SRS: x.SRS, Enabled: x.Enabled}
	if j.FeatureType.Name != "" {
		b := j.FeatureType
		ft.Name, ft.NativeName, ft.Title, ft.SRS, ft.Enabled = b.Name, b.NativeName, b.Title, b.SRS, b.Enabled
	}
	if ft.Name == "" {
		http.Error(w, "featureType name required", http.StatusBadRequest)
		return
	}
	if ft.NativeName == "" {
		ft.NativeName = ft.Name
	}
	if ft.SRS == "" {
		ft.SRS = "EPSG:4326"
	}
	if err := a.s.CreateFeatureType(r.Context(), ft); err != nil {
		httpErr(w, err)
		return
	}
	// GeoServer auto-publishes a layer for every new featuretype.
	if err := a.s.CreateLayer(r.Context(), model.Layer{Workspace: ws, Name: ft.Name,
		Type: "VECTOR", ResourceName: ft.Name, DefaultStyle: "generic", Enabled: true}); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.featuretype.created",
		map[string]string{"name": ft.Name, "workspace": ws})
	a.pub.Publish("catalog.layer.created",
		map[string]string{"name": ft.Name, "workspace": ws})
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(ft.Name))
}

func (a *api) getFeatureType(w http.ResponseWriter, r *http.Request) {
	ws, ds, name := r.PathValue("ws"), r.PathValue("ds"), trimFormat(r.PathValue("ft"))
	ft, err := a.s.GetFeatureType(r.Context(), ws, ds, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	var j ftJSON
	j.FeatureType.Name, j.FeatureType.NativeName = ft.Name, ft.NativeName
	j.FeatureType.Title, j.FeatureType.SRS, j.FeatureType.Enabled = ft.Title, ft.SRS, ft.Enabled
	writePayload(w, r, ftXML{Name: ft.Name, NativeName: ft.NativeName,
		Title: ft.Title, SRS: ft.SRS, Enabled: ft.Enabled}, j)
}

func (a *api) listFeatureTypes(w http.ResponseWriter, r *http.Request) {
	ws, ds := r.PathValue("ws"), trimFormat(r.PathValue("ds"))
	fts, err := a.s.ListFeatureTypes(r.Context(), ws, ds)
	if err != nil {
		httpErr(w, err)
		return
	}
	type ref struct {
		Name string `json:"name" xml:"name"`
	}
	refs := []ref{}
	for _, ft := range fts {
		refs = append(refs, ref{Name: ft.Name})
	}
	type listXML struct {
		XMLName struct{} `xml:"featureTypes"`
		Items   []ref    `xml:"featureType"`
	}
	writePayload(w, r, listXML{Items: refs},
		map[string]any{"featureTypes": map[string]any{"featureType": refs}})
}

func (a *api) deleteFeatureType(w http.ResponseWriter, r *http.Request) {
	ws, ds, name := r.PathValue("ws"), r.PathValue("ds"), trimFormat(r.PathValue("ft"))
	if r.URL.Query().Get("recurse") == "true" {
		a.s.DeleteLayer(r.Context(), ws, name)
	}
	if err := a.s.DeleteFeatureType(r.Context(), ws, ds, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.featuretype.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

func (a *api) createCoverage(w http.ResponseWriter, r *http.Request) {
	ws, cs := r.PathValue("ws"), r.PathValue("cs")
	var x covXML
	var j struct {
		Coverage struct {
			Name       string `json:"name"`
			NativeName string `json:"nativeName"`
			Title      string `json:"title"`
			SRS        string `json:"srs"`
			Enabled    bool   `json:"enabled"`
		} `json:"coverage"`
	}
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c := model.Coverage{Workspace: ws, Store: cs,
		Name: x.Name, NativeName: x.NativeName, Title: x.Title, SRS: x.SRS, Enabled: x.Enabled}
	if j.Coverage.Name != "" {
		b := j.Coverage
		c.Name, c.NativeName, c.Title, c.SRS, c.Enabled = b.Name, b.NativeName, b.Title, b.SRS, b.Enabled
	}
	if c.Name == "" {
		http.Error(w, "coverage name required", http.StatusBadRequest)
		return
	}
	if c.NativeName == "" {
		c.NativeName = c.Name
	}
	if c.SRS == "" {
		c.SRS = "EPSG:4326"
	}
	if err := a.s.CreateCoverage(r.Context(), c); err != nil {
		httpErr(w, err)
		return
	}
	if err := a.s.CreateLayer(r.Context(), model.Layer{Workspace: ws, Name: c.Name,
		Type: "RASTER", ResourceName: c.Name, DefaultStyle: "raster", Enabled: true}); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.coverage.created",
		map[string]string{"name": c.Name, "workspace": ws})
	a.pub.Publish("catalog.layer.created",
		map[string]string{"name": c.Name, "workspace": ws})
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(c.Name))
}

func (a *api) getCoverage(w http.ResponseWriter, r *http.Request) {
	ws, cs, name := r.PathValue("ws"), r.PathValue("cs"), trimFormat(r.PathValue("c"))
	c, err := a.s.GetCoverage(r.Context(), ws, cs, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	writePayload(w, r, covXML{Name: c.Name, NativeName: c.NativeName,
		Title: c.Title, SRS: c.SRS, Enabled: c.Enabled},
		map[string]any{"coverage": map[string]any{"name": c.Name, "nativeName": c.NativeName,
			"title": c.Title, "srs": c.SRS, "enabled": c.Enabled}})
}

func (a *api) deleteCoverage(w http.ResponseWriter, r *http.Request) {
	ws, cs, name := r.PathValue("ws"), r.PathValue("cs"), trimFormat(r.PathValue("c"))
	if r.URL.Query().Get("recurse") == "true" {
		a.s.DeleteLayer(r.Context(), ws, name)
	}
	if err := a.s.DeleteCoverage(r.Context(), ws, cs, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.coverage.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

// splitLayer parses "ws:name" (GeoServer qualified layer) or bare "name".
func splitLayer(qualified string) (ws, name string) {
	qualified = trimFormat(qualified)
	if i := strings.IndexByte(qualified, ':'); i >= 0 {
		return qualified[:i], qualified[i+1:]
	}
	return "", qualified
}

func (a *api) getLayer(w http.ResponseWriter, r *http.Request) {
	ws, name := splitLayer(r.PathValue("layer"))
	l, err := a.s.GetLayer(r.Context(), ws, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	writePayload(w, r,
		layerXML{Name: l.Name, Type: l.Type, DefaultStyle: &layerStyleRef{Name: l.DefaultStyle}},
		map[string]any{"layer": map[string]any{"name": l.Name, "type": l.Type,
			"defaultStyle": map[string]any{"name": l.DefaultStyle}}})
}

func (a *api) listLayers(w http.ResponseWriter, r *http.Request) {
	ls, err := a.s.ListLayers(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	type ref struct {
		Name string `json:"name" xml:"name"`
	}
	refs := []ref{}
	for _, l := range ls {
		n := l.Name
		if l.Workspace != "" {
			n = l.Workspace + ":" + l.Name
		}
		refs = append(refs, ref{Name: n})
	}
	type listXML struct {
		XMLName struct{} `xml:"layers"`
		Items   []ref    `xml:"layer"`
	}
	writePayload(w, r, listXML{Items: refs},
		map[string]any{"layers": map[string]any{"layer": refs}})
}

func (a *api) updateLayer(w http.ResponseWriter, r *http.Request) {
	ws, name := splitLayer(r.PathValue("layer"))
	var x layerXML
	var j struct {
		Layer struct {
			DefaultStyle struct {
				Name string `json:"name"`
			} `json:"defaultStyle"`
			Enabled bool `json:"enabled"`
		} `json:"layer"`
	}
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cur, err := a.s.GetLayer(r.Context(), ws, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	if x.DefaultStyle != nil && x.DefaultStyle.Name != "" {
		cur.DefaultStyle = x.DefaultStyle.Name
	}
	if j.Layer.DefaultStyle.Name != "" {
		cur.DefaultStyle = j.Layer.DefaultStyle.Name
	}
	if err := a.s.UpdateLayer(r.Context(), ws, name, cur); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layer.updated",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

func (a *api) deleteLayer(w http.ResponseWriter, r *http.Request) {
	ws, name := splitLayer(r.PathValue("layer"))
	if err := a.s.DeleteLayer(r.Context(), ws, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layer.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

func isSLDUpload(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.Contains(ct, "sld") || strings.Contains(ct, "application/xml") &&
		r.URL.Query().Get("name") != ""
}

func (a *api) createStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isSLDUpload(r) {
			name := r.URL.Query().Get("name")
			if name == "" {
				http.Error(w, "name query param required for SLD upload", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			st := model.Style{Workspace: ws, Name: name, Format: "sld",
				Filename: name + ".sld", Body: string(body)}
			if err := a.s.CreateStyle(r.Context(), st); err != nil {
				httpErr(w, err)
				return
			}
			a.pub.Publish("catalog.style.created",
				map[string]string{"name": name, "workspace": ws})
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(name))
			return
		}
		var x styleXML
		var j struct {
			Style struct {
				Name     string `json:"name"`
				Format   string `json:"format"`
				Filename string `json:"filename"`
			} `json:"style"`
		}
		if err := readPayload(r, &x, &j); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st := model.Style{Workspace: ws, Name: x.Name, Format: x.Format, Filename: x.Filename}
		if j.Style.Name != "" {
			st.Name, st.Format, st.Filename = j.Style.Name, j.Style.Format, j.Style.Filename
		}
		if st.Format == "" {
			st.Format = "sld"
		}
		if err := a.s.CreateStyle(r.Context(), st); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.style.created",
			map[string]string{"name": st.Name, "workspace": ws})
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(st.Name))
	}
}

func (a *api) createStyleWS(w http.ResponseWriter, r *http.Request) {
	a.createStyle(r.PathValue("ws"))(w, r)
}

func (a *api) getStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.PathValue("s")
		if strings.HasSuffix(raw, ".sld") {
			st, err := a.s.GetStyle(r.Context(), ws, strings.TrimSuffix(raw, ".sld"))
			if err != nil {
				httpErr(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.ogc.sld+xml")
			w.Write([]byte(st.Body))
			return
		}
		st, err := a.s.GetStyle(r.Context(), ws, trimFormat(raw))
		if err != nil {
			httpErr(w, err)
			return
		}
		writePayload(w, r,
			styleXML{Name: st.Name, Format: st.Format, Filename: st.Filename},
			map[string]any{"style": map[string]any{"name": st.Name,
				"format": st.Format, "filename": st.Filename}})
	}
}

func (a *api) getStyleWS(w http.ResponseWriter, r *http.Request) {
	a.getStyle(r.PathValue("ws"))(w, r)
}

func (a *api) listStyles(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sts, err := a.s.ListStyles(r.Context(), ws)
		if err != nil {
			httpErr(w, err)
			return
		}
		type ref struct {
			Name string `json:"name" xml:"name"`
		}
		refs := []ref{}
		for _, st := range sts {
			refs = append(refs, ref{Name: st.Name})
		}
		type listXML struct {
			XMLName struct{} `xml:"styles"`
			Items   []ref    `xml:"style"`
		}
		writePayload(w, r, listXML{Items: refs},
			map[string]any{"styles": map[string]any{"style": refs}})
	}
}

func (a *api) updateStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := trimFormat(strings.TrimSuffix(r.PathValue("s"), ".sld"))
		cur, err := a.s.GetStyle(r.Context(), ws, name)
		if err != nil {
			httpErr(w, err)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cur.Body = string(body)
		if err := a.s.UpdateStyle(r.Context(), ws, name, cur); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.style.updated",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}

func (a *api) deleteStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := trimFormat(r.PathValue("s"))
		if err := a.s.DeleteStyle(r.Context(), ws, name); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.style.deleted",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}

func (a *api) createLayerGroup(w http.ResponseWriter, r *http.Request) {
	ws := r.PathValue("ws") // empty for global route
	var x lgXML
	var j struct {
		LayerGroup struct {
			Name   string `json:"name"`
			Mode   string `json:"mode"`
			Layers struct {
				Layer []string `json:"layer"`
			} `json:"layers"`
		} `json:"layerGroup"`
	}
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lg := model.LayerGroup{Workspace: ws, Name: x.Name, Mode: x.Mode, Layers: x.Layers}
	if j.LayerGroup.Name != "" {
		lg.Name, lg.Mode, lg.Layers = j.LayerGroup.Name, j.LayerGroup.Mode, j.LayerGroup.Layers.Layer
	}
	if lg.Mode == "" {
		lg.Mode = "SINGLE"
	}
	if lg.Name == "" {
		http.Error(w, "layerGroup name required", http.StatusBadRequest)
		return
	}
	if err := a.s.CreateLayerGroup(r.Context(), lg); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layergroup.created",
		map[string]string{"name": lg.Name, "workspace": ws})
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(lg.Name))
}

func (a *api) getLayerGroup(w http.ResponseWriter, r *http.Request) {
	ws := r.PathValue("ws")
	name := trimFormat(r.PathValue("lg"))
	lg, err := a.s.GetLayerGroup(r.Context(), ws, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	writePayload(w, r, lgXML{Name: lg.Name, Mode: lg.Mode, Layers: lg.Layers},
		map[string]any{"layerGroup": map[string]any{"name": lg.Name, "mode": lg.Mode,
			"layers": map[string]any{"layer": lg.Layers}}})
}

func (a *api) deleteLayerGroup(w http.ResponseWriter, r *http.Request) {
	ws := r.PathValue("ws")
	name := trimFormat(r.PathValue("lg"))
	if err := a.s.DeleteLayerGroup(r.Context(), ws, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layergroup.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}
```

Register in `rest.go` `Mount`: add `a.layerRoutes(inner)`.

- [ ] **Step 6: Run all tests → PASS; commit**

```bash
GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test ./...
git add services/catalog
git commit -m "feat(catalog): featuretypes, coverages, layers, styles, layergroups over /rest"
```

---

### Task 8: NATS events, /api/v1, e2e smoke, docs, close out

**Files:**
- Create: `services/catalog/internal/events/events.go`, `services/catalog/internal/rest/apiv1.go`, `services/catalog/internal/rest/apiv1_test.go`, `docs/services/catalog.md`
- Modify: `services/catalog/main.go`, `docs/architecture.md` (status), `task.md` (check Sprint 2)

**Interfaces:**
- Consumes: everything above; NATS at `nats:4222` (compose).
- Produces:
  - `events.NewNATS(nc *nats.Conn) rest.Publisher` — JSON-marshals payload, `nc.Publish(subject, data)`; errors logged, never fail requests.
  - `/api/v1/layers` — flat JSON list for the frontend: `[{"workspace":"topp","name":"roads","type":"VECTOR","defaultStyle":"generic"}]`
  - `/api/v1/workspaces` — `[{"name":"topp"}]`

- [ ] **Step 1: Implement events.go**

```go
// Package events publishes catalog change notifications on NATS.
package events

import (
	"encoding/json"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type natsPub struct{ nc *nats.Conn }

func NewNATS(nc *nats.Conn) *natsPub { return &natsPub{nc: nc} }

// Publish is fire-and-forget: catalog mutations must not fail on event errors.
func (p *natsPub) Publish(subject string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("event marshal", "subject", subject, "err", err)
		return
	}
	if err := p.nc.Publish(subject, data); err != nil {
		slog.Error("event publish", "subject", subject, "err", err)
	}
}
```

- [ ] **Step 2: Failing /api/v1 test**

`services/catalog/internal/rest/apiv1_test.go`:

```go
package rest

import (
	"strings"
	"testing"
)

func TestAPIV1Layers(t *testing.T) {
	mux, _ := testMux(t)
	do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>topp</name></workspace>`)
	do(t, mux, "POST", "/rest/workspaces/topp/datastores", "application/xml",
		`<dataStore><name>files</name><type>Directory</type><enabled>true</enabled><connectionParameters/></dataStore>`)
	do(t, mux, "POST", "/rest/workspaces/topp/datastores/files/featuretypes",
		"application/xml", `<featureType><name>roads</name><enabled>true</enabled></featureType>`)

	rec := do(t, mux, "GET", "/api/v1/layers", "", "")
	if rec.Code != 200 {
		t.Fatalf("api layers = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"workspace":"topp"`) || !strings.Contains(body, `"defaultStyle":"generic"`) {
		t.Fatalf("api layers body = %s", body)
	}
	rec = do(t, mux, "GET", "/api/v1/workspaces", "", "")
	if !strings.Contains(rec.Body.String(), `"name":"topp"`) {
		t.Fatalf("api workspaces = %s", rec.Body.String())
	}
}
```

Run → FAIL (404)

- [ ] **Step 3: Implement apiv1.go**

```go
package rest

import (
	"encoding/json"
	"net/http"
)

// apiV1Routes serves the clean JSON API consumed by the Geoson frontend.
func (a *api) apiV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/workspaces", func(w http.ResponseWriter, r *http.Request) {
		list, err := a.s.ListWorkspaces(r.Context())
		if err != nil {
			httpErr(w, err)
			return
		}
		type ws struct {
			Name string `json:"name"`
		}
		out := []ws{}
		for _, item := range list {
			out = append(out, ws{Name: item.Name})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /api/v1/layers", func(w http.ResponseWriter, r *http.Request) {
		list, err := a.s.ListLayers(r.Context())
		if err != nil {
			httpErr(w, err)
			return
		}
		type layer struct {
			Workspace    string `json:"workspace"`
			Name         string `json:"name"`
			Type         string `json:"type"`
			DefaultStyle string `json:"defaultStyle"`
		}
		out := []layer{}
		for _, l := range list {
			out = append(out, layer{Workspace: l.Workspace, Name: l.Name,
				Type: l.Type, DefaultStyle: l.DefaultStyle})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
}
```

In `rest.go` `Mount`: register `a.apiV1Routes(inner)` and route `/api/v1/` on the outer mux:

```go
	mux.Handle("/api/v1/", inner)
```

- [ ] **Step 4: Wire NATS publisher in main.go**

Change `newHandler` to accept publisher already in deps; in `main()` after NATS connect:

```go
	var pub rest.Publisher = noopPub{}
	if d.nc != nil {
		pub = events.NewNATS(d.nc)
	}
	d.pub = pub
```

(`deps` gains field `pub rest.Publisher`; `newHandler` passes `d.pub` to `rest.Mount`; when `d.pub == nil` use `noopPub{}`.)

- [ ] **Step 5: Full test + e2e smoke against compose**

```bash
cd /home/madson/geoson
GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson go test github.com/geoson/geoson/...
cd deploy/compose && docker compose up -d --build catalog
sleep 3
# through traefik:
curl -fsS -X POST -H "Content-Type: application/xml" \
  -d '<workspace><name>demo</name></workspace>' http://localhost/geoserver/rest/workspaces
curl -fsS http://localhost/geoserver/rest/workspaces.json
# NATS event visible:
docker compose exec nats sh -c "nats sub 'catalog.>' --count=1 --timeout=5s" 2>/dev/null || true
```

Expected: workspace created (201), JSON list contains `demo`.

- [ ] **Step 6: Write docs/services/catalog.md**

```markdown
# catalog

Configuration system of record. Go + Postgres.

## Endpoints
- `/geoserver/rest/*`, `/rest/*` — GeoServer-compatible config API (XML + JSON)
  - workspaces, datastores, coveragestores, featuretypes, coverages, layers,
    styles (incl. raw SLD upload/download), layergroups
- `/api/v1/workspaces`, `/api/v1/layers` — clean JSON API for the frontend
- `/healthz`, `/readyz`

## Store types
PostGIS (live validation + introspection), Shapefile, GeoPackage, GeoJSON,
GeoTIFF/COG (magic-byte validation), GeoParquet (DuckDB schema introspection).

## Events
Every mutation publishes `catalog.<entity>.<created|updated|deleted>`
(JSON `{"name","workspace"}`) on NATS. Consumers: tiles (cache invalidation),
wms/wfs (config cache drop).

## Env
`GEOSON_HTTP_ADDR`, `GEOSON_DATABASE_URL`, `GEOSON_NATS_URL`
```

Update `docs/architecture.md` catalog row status to `done (Sprint 2)`.

- [ ] **Step 7: Check Sprint 2 in task.md, final commit**

```bash
cd /home/madson/geoson
# flip: - [ ] **Sprint 2 — Catalog + stores** → - [x]
git add -A
git commit -m "feat(catalog): nats events, /api/v1, docs; complete sprint 2"
```
