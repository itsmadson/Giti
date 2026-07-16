# Sprint 4 — Auth + GeoFence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Auth service: users/groups/roles (argon2id), JWT + HTTP Basic (GeoServer default `admin`/`geoserver`), GeoFence-style priority rule engine (ALLOW/DENY/LIMIT with CQL/attribute limits), Redis decision cache, `/rest/security` + `/rest/geofence/rules` compat, internal `/check` consumed by a new gateway auth middleware.

**Architecture:** `services/auth` mirrors catalog's structure (pgx repo + embedded migrations + REST handlers). Rule evaluation is a pure function over ordered rules (first ALLOW/DENY match decides; earlier LIMIT matches attach CQL/attribute constraints). Gateway calls `GET auth:/check` per OWS request when `GITI_AUTH_URL` set; decisions cached in Redis keyed by (user, service, request, ws, layer) + generation counter bumped on any security mutation.

**Tech Stack:** Go 1.26, pgx/v5, `golang.org/x/crypto/argon2`, `github.com/golang-jwt/jwt/v5`, `github.com/redis/go-redis/v9`.

## Global Constraints

- Health convention: `/healthz`, `/readyz`, `GITI_HTTP_ADDR`, `health.Serve` (Sprint 1).
- Go tests: `go test github.com/giti/giti/...`; GOPROXY `https://goproxy.cn`; integration tests skip without `GITI_TEST_DATABASE_URL` (compose Postgres `127.0.0.1:5433`).
- Dockerfiles: `GOWORK=off`, copy only own module + `libs/` (Sprint 3 fix).
- GeoServer compat: default admin user `admin` password `geoserver` seeded (warn in logs); `/rest/security/usergroup/users`, `/rest/security/roles` XML+JSON shapes; GeoFence rules under `/rest/geofence/rules` (JSON, GeoFence REST shape).
- Default when no rule matches: `GITI_AUTH_DEFAULT` env, `ALLOW` (GeoServer-like open read) — settable to `DENY` (GeoFence-like).
- Commit after every task, Conventional Commits.

## File Structure

```
services/auth/
  go.mod  main.go  main_test.go  Dockerfile
  internal/store/store.go              # users/groups/roles/rules repo
  internal/store/migrate.go            # embedded migrations (copy of catalog pattern)
  internal/store/migrations/0001_init.sql
  internal/store/store_test.go
  internal/password/password.go        # argon2id hash+verify
  internal/password/password_test.go
  internal/token/token.go              # JWT issue/verify (HS256)
  internal/token/token_test.go
  internal/rules/rules.go              # pure evaluation engine
  internal/rules/rules_test.go
  internal/api/api.go                  # router assembly (/rest/security, /rest/geofence, /api/v1/auth, /check)
  internal/api/security.go             # users/groups/roles REST
  internal/api/geofence.go             # rules REST
  internal/api/check.go                # login + /check + redis cache
  internal/api/*_test.go
services/gateway/authz.go              # gateway middleware calling auth /check
services/gateway/authz_test.go
```

---

### Task 1: Auth service scaffold + compose + CI

**Files:**
- Create: `services/auth/go.mod`, `services/auth/main.go`, `services/auth/main_test.go`, `services/auth/Dockerfile`
- Modify: `go.work`, `deploy/compose/docker-compose.yml`, `.github/workflows/ci.yml` (docker build line)

**Interfaces:**
- Consumes: `libs/ogc-kit/health`.
- Produces: `newHandler(d deps) http.Handler`; `type deps struct { db *pgxpool.Pool; rdb *redis.Client; secret []byte; defaultAllow bool }`. Service DNS `auth:8080`. Env: `GITI_DATABASE_URL`, `GITI_REDIS_URL` (e.g. `redis:6379`), `GITI_JWT_SECRET`, `GITI_AUTH_DEFAULT`.

- [x] **Step 1: Init module**

```bash
cd /home/madson/giti
mkdir -p services/auth
( cd services/auth && go mod init github.com/giti/giti/services/auth )
go work use ./services/auth
cd services/auth
go mod edit -require=github.com/giti/giti/libs/ogc-kit@v0.0.0
go mod edit -replace=github.com/giti/giti/libs/ogc-kit=../../libs/ogc-kit
go get github.com/jackc/pgx/v5/pgxpool@latest github.com/redis/go-redis/v9@latest \
  github.com/golang-jwt/jwt/v5@latest golang.org/x/crypto@latest
```

- [x] **Step 2: Failing smoke test** — `services/auth/main_test.go`:

```go
package main

import (
	"net/http/httptest"
	"testing"
)

func TestAuthServesHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler(deps{}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q", rec.Code, rec.Body.String())
	}
}
```

Run: `go test ./...` → FAIL `undefined: newHandler`

- [x] **Step 3: main.go**

```go
// Command auth is the Giti authentication and authorization service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/giti/giti/libs/ogc-kit/health"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type deps struct {
	db           *pgxpool.Pool
	rdb          *redis.Client
	secret       []byte
	defaultAllow bool
}

func newHandler(d deps) http.Handler {
	checks := map[string]health.Check{}
	if d.db != nil {
		checks["postgres"] = func(ctx context.Context) error { return d.db.Ping(ctx) }
	}
	if d.rdb != nil {
		checks["redis"] = func(ctx context.Context) error { return d.rdb.Ping(ctx).Err() }
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(checks))
	mux.Handle("/readyz", health.NewMux(checks))
	// Task 5/6 mount api.Mount(mux, ...) here.
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	addr := os.Getenv("GITI_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	d := deps{
		secret:       []byte(os.Getenv("GITI_JWT_SECRET")),
		defaultAllow: os.Getenv("GITI_AUTH_DEFAULT") != "DENY",
	}
	if len(d.secret) == 0 {
		slog.Warn("GITI_JWT_SECRET not set; using insecure dev secret")
		d.secret = []byte("giti-dev-secret")
	}
	if dsn := os.Getenv("GITI_DATABASE_URL"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			slog.Error("postgres connect", "err", err)
			os.Exit(1)
		}
		d.db = pool
	}
	if raddr := os.Getenv("GITI_REDIS_URL"); raddr != "" {
		d.rdb = redis.NewClient(&redis.Options{Addr: raddr})
	}
	slog.Info("auth listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(d)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
```

Run: `go mod tidy && go test ./...` → PASS

- [x] **Step 4: Dockerfile** — `services/auth/Dockerfile`:

```dockerfile
# Build context must be the repo root: docker build -f services/auth/Dockerfile .
FROM golang:1.26-alpine AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY GOWORK=off
WORKDIR /src
COPY libs/ogc-kit/ libs/ogc-kit/
COPY services/auth/ services/auth/
RUN go build -C services/auth -ldflags="-s -w" -o /out/auth .

FROM alpine:3.21
RUN apk add --no-cache curl ca-certificates && adduser -D -u 10001 giti
USER giti
COPY --from=build /out/auth /usr/local/bin/auth
ENV GITI_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["auth"]
```

- [x] **Step 5: Compose service (before `postgres:`) + CI build line**

```yaml
  auth:
    build:
      context: ../..
      dockerfile: services/auth/Dockerfile
      args:
        GOPROXY: ${GOPROXY:-https://proxy.golang.org,direct}
    environment:
      GITI_DATABASE_URL: postgres://${POSTGRES_USER:-giti}:${POSTGRES_PASSWORD:-giti-dev-password}@postgres:5432/${POSTGRES_DB:-giti}
      GITI_REDIS_URL: redis:6379
      GITI_JWT_SECRET: ${GITI_JWT_SECRET:-giti-dev-secret}
    labels:
      - traefik.enable=true
      - traefik.http.routers.auth.rule=PathPrefix(`/giti/rest/security`) || PathPrefix(`/giti/rest/geofence`) || PathPrefix(`/api/v1/auth`)
      - traefik.http.routers.auth.priority=20
      - traefik.http.services.auth.loadbalancer.server.port=8080
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }
```

CI `docker-build` job: `- run: docker build -f services/auth/Dockerfile .`
Gateway compose env gains: `GITI_AUTH_URL: http://auth:8080`.

- [x] **Step 6: Verify + commit**

```bash
cd /home/madson/giti && go test github.com/giti/giti/...
docker compose -f deploy/compose/docker-compose.yml config -q
git add -A && git commit -m "feat(auth): service scaffold, compose + ci wiring"
```

---

### Task 2: Password hashing (argon2id)

**Files:**
- Create: `services/auth/internal/password/password.go`, `services/auth/internal/password/password_test.go`

**Interfaces:**
- Produces:

```go
// Hash returns a PHC-format argon2id hash: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<key>
func Hash(password string) (string, error)
// Verify reports whether password matches the PHC hash. Constant-time.
func Verify(password, phc string) bool
```

- [x] **Step 1: Failing test**

```go
package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("geoserver")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify("geoserver", h) {
		t.Fatal("verify correct password = false")
	}
	if Verify("wrong", h) {
		t.Fatal("verify wrong password = true")
	}
	h2, _ := Hash("geoserver")
	if h == h2 {
		t.Fatal("hashes must be salted (identical output)")
	}
	if Verify("geoserver", "not-a-phc-hash") {
		t.Fatal("garbage hash must not verify")
	}
}
```

Run → FAIL

- [x] **Step 2: Implement**

```go
// Package password implements argon2id hashing in PHC string format.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memKiB   = 64 * 1024
	timeCost = 1
	threads  = 4
	keyLen   = 32
	saltLen  = 16
)

func Hash(pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, timeCost, memKiB, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memKiB, timeCost, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func Verify(pw, phc string) bool {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
```

- [x] **Step 3: Run** → PASS. **Commit** `git commit -m "feat(auth): argon2id password hashing"`

---

### Task 3: Users/groups/roles/rules storage + seed admin

**Files:**
- Create: `services/auth/internal/store/migrate.go` (identical pattern to catalog's), `services/auth/internal/store/migrations/0001_init.sql`, `services/auth/internal/store/store.go`, `services/auth/internal/store/store_test.go`
- Modify: `services/auth/main.go` (run Migrate + seed at boot)

**Interfaces:**
- Produces (package `store`):

```go
var ErrNotFound, ErrConflict error
func Migrate(ctx, db *pgxpool.Pool) error
func New(db *pgxpool.Pool) *Store
type User struct { Name string; Enabled bool; PasswordHash string }
type Rule struct {
	ID int64; Priority int64
	Username, Rolename, Service, Request, Workspace, Layer string // "*" or "" = any
	Access string // ALLOW | DENY | LIMIT
	CQLRead, CQLWrite string
	Attributes []string
}
// Users: CreateUser(ctx, User) / GetUser(ctx, name) / ListUsers(ctx) / UpdateUser(ctx, name, User) / DeleteUser(ctx, name)
// Groups: CreateGroup(ctx, name) / ListGroups(ctx) / DeleteGroup(ctx, name) / AddUserToGroup(ctx, user, group) / GroupsOf(ctx, user)
// Roles: CreateRole(ctx, name) / ListRoles(ctx) / DeleteRole(ctx, name) /
//        AssignRoleUser(ctx, role, user) / AssignRoleGroup(ctx, role, group) / RolesOf(ctx, user) ([]string, error)  // direct + via groups
// Rules: CreateRule(ctx, Rule) (int64, error) / ListRules(ctx) ([]Rule, error) — ordered by priority / DeleteRule(ctx, id) / SeedAdmin(ctx, hash string) error
```

- [x] **Step 1: 0001_init.sql**

```sql
CREATE TABLE IF NOT EXISTS giti_auth_migrations (
    version int PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth_users (
    name text PRIMARY KEY,
    enabled boolean NOT NULL DEFAULT true,
    password_hash text NOT NULL
);

CREATE TABLE auth_groups (
    name text PRIMARY KEY
);

CREATE TABLE auth_user_groups (
    username text NOT NULL REFERENCES auth_users(name) ON DELETE CASCADE,
    groupname text NOT NULL REFERENCES auth_groups(name) ON DELETE CASCADE,
    PRIMARY KEY (username, groupname)
);

CREATE TABLE auth_roles (
    name text PRIMARY KEY
);

CREATE TABLE auth_role_users (
    rolename text NOT NULL REFERENCES auth_roles(name) ON DELETE CASCADE,
    username text NOT NULL REFERENCES auth_users(name) ON DELETE CASCADE,
    PRIMARY KEY (rolename, username)
);

CREATE TABLE auth_role_groups (
    rolename text NOT NULL REFERENCES auth_roles(name) ON DELETE CASCADE,
    groupname text NOT NULL REFERENCES auth_groups(name) ON DELETE CASCADE,
    PRIMARY KEY (rolename, groupname)
);

CREATE TABLE geofence_rules (
    id bigserial PRIMARY KEY,
    priority bigint NOT NULL,
    username text NOT NULL DEFAULT '*',
    rolename text NOT NULL DEFAULT '*',
    service text NOT NULL DEFAULT '*',
    request text NOT NULL DEFAULT '*',
    workspace text NOT NULL DEFAULT '*',
    layer text NOT NULL DEFAULT '*',
    access text NOT NULL CHECK (access IN ('ALLOW','DENY','LIMIT')),
    cql_read text NOT NULL DEFAULT '',
    cql_write text NOT NULL DEFAULT '',
    attributes jsonb NOT NULL DEFAULT '[]'
);
CREATE INDEX geofence_rules_priority ON geofence_rules(priority);
```

`migrate.go`: copy catalog's `internal/store/migrate.go` verbatim, changing the tracking table name to `giti_auth_migrations` (both in the `CREATE TABLE IF NOT EXISTS` and the `SELECT`/`INSERT` statements).

- [x] **Step 2: Failing test** — `store_test.go` (testDB helper identical to catalog's, same env var):

```go
package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GITI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GITI_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("a%d", time.Now().UnixNano())
	if _, err := pool.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
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

func TestUserGroupRoleLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)

	if err := s.CreateUser(ctx, User{Name: "alice", Enabled: true, PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateGroup(ctx, "editors"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, "alice", "editors"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRole(ctx, "ROLE_EDITOR"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRole(ctx, "ROLE_DIRECT"); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignRoleGroup(ctx, "ROLE_EDITOR", "editors"); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignRoleUser(ctx, "ROLE_DIRECT", "alice"); err != nil {
		t.Fatal(err)
	}
	roles, err := s.RolesOf(ctx, "alice")
	if err != nil || len(roles) != 2 {
		t.Fatalf("roles = %v, %v (want 2)", roles, err)
	}
}

func TestSeedAdminIdempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if err := s.SeedAdmin(ctx, "hash1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedAdmin(ctx, "hash2"); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	u, err := s.GetUser(ctx, "admin")
	if err != nil || u.PasswordHash != "hash1" {
		t.Fatalf("admin = %+v, %v (seed must not overwrite)", u, err)
	}
	roles, _ := s.RolesOf(ctx, "admin")
	if len(roles) != 1 || roles[0] != "ADMIN" {
		t.Fatalf("admin roles = %v", roles)
	}
}

func TestRuleCRUDOrdering(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if _, err := s.CreateRule(ctx, Rule{Priority: 20, Access: "DENY"}); err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateRule(ctx, Rule{Priority: 10, Access: "ALLOW", Workspace: "topp"})
	if err != nil {
		t.Fatal(err)
	}
	rules, err := s.ListRules(ctx)
	if err != nil || len(rules) != 2 || rules[0].Priority != 10 {
		t.Fatalf("rules = %+v, %v", rules, err)
	}
	if err := s.DeleteRule(ctx, id); err != nil {
		t.Fatal(err)
	}
}
```

Run → FAIL

- [x] **Step 3: Implement store.go**

```go
// Package store persists auth users/groups/roles and GeoFence rules.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type User struct {
	Name         string
	Enabled      bool
	PasswordHash string
}

type Rule struct {
	ID         int64
	Priority   int64
	Username   string
	Rolename   string
	Service    string
	Request    string
	Workspace  string
	Layer      string
	Access     string
	CQLRead    string
	CQLWrite   string
	Attributes []string
}

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

func (s *Store) CreateUser(ctx context.Context, u User) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_users(name, enabled, password_hash) VALUES($1,$2,$3)`,
		u.Name, u.Enabled, u.PasswordHash)
	return mapErr(err)
}

func (s *Store) GetUser(ctx context.Context, name string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		`SELECT name, enabled, password_hash FROM auth_users WHERE name=$1`, name,
	).Scan(&u.Name, &u.Enabled, &u.PasswordHash)
	return u, mapErr(err)
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.Query(ctx, `SELECT name, enabled, password_hash FROM auth_users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Name, &u.Enabled, &u.PasswordHash); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, name string, u User) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE auth_users SET enabled=$2, password_hash=$3 WHERE name=$1`,
		name, u.Enabled, u.PasswordHash)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM auth_users WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateGroup(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO auth_groups(name) VALUES($1)`, name)
	return mapErr(err)
}

func (s *Store) ListGroups(ctx context.Context) ([]string, error) {
	return s.listNames(ctx, `SELECT name FROM auth_groups ORDER BY name`)
}

func (s *Store) DeleteGroup(ctx context.Context, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM auth_groups WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddUserToGroup(ctx context.Context, user, group string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_user_groups(username, groupname) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		user, group)
	return mapErr(err)
}

func (s *Store) GroupsOf(ctx context.Context, user string) ([]string, error) {
	return s.listNames(ctx,
		`SELECT groupname FROM auth_user_groups WHERE username=$1 ORDER BY groupname`, user)
}

func (s *Store) CreateRole(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO auth_roles(name) VALUES($1)`, name)
	return mapErr(err)
}

func (s *Store) ListRoles(ctx context.Context) ([]string, error) {
	return s.listNames(ctx, `SELECT name FROM auth_roles ORDER BY name`)
}

func (s *Store) DeleteRole(ctx context.Context, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM auth_roles WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AssignRoleUser(ctx context.Context, role, user string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_role_users(rolename, username) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		role, user)
	return mapErr(err)
}

func (s *Store) AssignRoleGroup(ctx context.Context, role, group string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_role_groups(rolename, groupname) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		role, group)
	return mapErr(err)
}

// RolesOf returns roles assigned directly and via group membership.
func (s *Store) RolesOf(ctx context.Context, user string) ([]string, error) {
	return s.listNames(ctx, `
		SELECT rolename FROM auth_role_users WHERE username=$1
		UNION
		SELECT rg.rolename FROM auth_role_groups rg
		JOIN auth_user_groups ug ON ug.groupname = rg.groupname
		WHERE ug.username=$1
		ORDER BY 1`, user)
}

func (s *Store) listNames(ctx context.Context, sql string, args ...any) ([]string, error) {
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateRule(ctx context.Context, r Rule) (int64, error) {
	if r.Attributes == nil {
		r.Attributes = []string{}
	}
	norm := func(v string) string {
		if v == "" {
			return "*"
		}
		return v
	}
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO geofence_rules(priority, username, rolename, service, request, workspace, layer, access, cql_read, cql_write, attributes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		r.Priority, norm(r.Username), norm(r.Rolename), norm(r.Service), norm(r.Request),
		norm(r.Workspace), norm(r.Layer), r.Access, r.CQLRead, r.CQLWrite, r.Attributes,
	).Scan(&id)
	return id, mapErr(err)
}

func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, priority, username, rolename, service, request, workspace, layer, access, cql_read, cql_write, attributes
		FROM geofence_rules ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Priority, &r.Username, &r.Rolename, &r.Service,
			&r.Request, &r.Workspace, &r.Layer, &r.Access, &r.CQLRead, &r.CQLWrite,
			&r.Attributes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRule(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM geofence_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SeedAdmin creates the GeoServer-default admin user and ADMIN role once.
func (s *Store) SeedAdmin(ctx context.Context, hash string) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO auth_users(name, enabled, password_hash) VALUES('admin', true, $1)
		 ON CONFLICT (name) DO NOTHING`, hash); err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx,
		`INSERT INTO auth_roles(name) VALUES('ADMIN') ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_role_users(rolename, username) VALUES('ADMIN','admin')
		 ON CONFLICT DO NOTHING`)
	return err
}
```

- [x] **Step 4: Wire into main** — after pool creation in `main()`:

```go
		if err := store.Migrate(ctx, pool); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
		hash, err := password.Hash("geoserver")
		if err != nil {
			slog.Error("seed hash", "err", err)
			os.Exit(1)
		}
		if err := store.New(pool).SeedAdmin(ctx, hash); err != nil {
			slog.Error("seed admin", "err", err)
			os.Exit(1)
		}
		slog.Warn("default admin user active — change the password", "user", "admin")
```

(imports: `internal/password`, `internal/store`)

- [x] **Step 5: Run tests** (with `GITI_TEST_DATABASE_URL`) → PASS. **Commit** `git commit -m "feat(auth): users/groups/roles/rules storage with seeded admin"`

---

### Task 4: Rule evaluation engine (pure)

**Files:**
- Create: `services/auth/internal/rules/rules.go`, `services/auth/internal/rules/rules_test.go`

**Interfaces:**
- Consumes: `store.Rule`.
- Produces (package `rules`):

```go
type Subject struct { Username string; Roles []string } // Username "" = anonymous
type Query struct { Service, Request, Workspace, Layer string }
type Decision struct {
	Allow      bool     `json:"allow"`
	CQLRead    string   `json:"cqlRead,omitempty"`
	CQLWrite   string   `json:"cqlWrite,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}
// Evaluate walks rules (already priority-ordered). GeoFence semantics:
// LIMIT matches accumulate constraints; first matching ALLOW/DENY decides.
// No ALLOW/DENY match -> defaultAllow.
func Evaluate(rs []store.Rule, sub Subject, q Query, defaultAllow bool) Decision
```

Matching: field matches when rule value is `*` or equals query value case-insensitively; `Username` matches subject username; `Rolename` matches any subject role.

- [x] **Step 1: Failing test**

```go
package rules

import (
	"testing"

	"github.com/giti/giti/services/auth/internal/store"
)

func r(pri int64, user, role, svc, ws, access, cql string) store.Rule {
	return store.Rule{Priority: pri, Username: user, Rolename: role, Service: svc,
		Request: "*", Workspace: ws, Layer: "*", Access: access, CQLRead: cql}
}

func TestFirstMatchDecides(t *testing.T) {
	rs := []store.Rule{
		r(10, "*", "*", "WMS", "secret", "DENY", ""),
		r(20, "*", "*", "*", "*", "ALLOW", ""),
	}
	d := Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "secret"}, true)
	if d.Allow {
		t.Fatal("secret workspace must be denied")
	}
	d = Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "open"}, true)
	if !d.Allow {
		t.Fatal("open workspace must be allowed")
	}
}

func TestRoleMatch(t *testing.T) {
	rs := []store.Rule{
		r(10, "*", "ROLE_EDITOR", "WFS", "*", "ALLOW", ""),
		r(20, "*", "*", "WFS", "*", "DENY", ""),
	}
	editor := Subject{Username: "alice", Roles: []string{"ROLE_EDITOR"}}
	if d := Evaluate(rs, editor, Query{Service: "WFS"}, true); !d.Allow {
		t.Fatal("editor must be allowed")
	}
	if d := Evaluate(rs, Subject{Username: "bob"}, Query{Service: "WFS"}, true); d.Allow {
		t.Fatal("bob must be denied")
	}
}

func TestLimitAccumulates(t *testing.T) {
	rs := []store.Rule{
		{Priority: 5, Username: "*", Rolename: "*", Service: "*", Request: "*",
			Workspace: "topp", Layer: "roads", Access: "LIMIT",
			CQLRead: "state='CA'", Attributes: []string{"id", "name"}},
		r(10, "*", "*", "*", "topp", "ALLOW", ""),
	}
	d := Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "topp", Layer: "roads"}, false)
	if !d.Allow || d.CQLRead != "state='CA'" || len(d.Attributes) != 2 {
		t.Fatalf("decision = %+v", d)
	}
	// two LIMIT rules AND together
	rs = append([]store.Rule{{Priority: 1, Username: "*", Rolename: "*", Service: "*",
		Request: "*", Workspace: "topp", Layer: "roads", Access: "LIMIT",
		CQLRead: "public=true"}}, rs...)
	d = Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "topp", Layer: "roads"}, false)
	if d.CQLRead != "(public=true) AND (state='CA')" {
		t.Fatalf("cql = %q", d.CQLRead)
	}
}

func TestDefaultWhenNoMatch(t *testing.T) {
	if d := Evaluate(nil, Subject{}, Query{Service: "WMS"}, true); !d.Allow {
		t.Fatal("default allow")
	}
	if d := Evaluate(nil, Subject{}, Query{Service: "WMS"}, false); d.Allow {
		t.Fatal("default deny")
	}
}

func TestCaseInsensitiveMatch(t *testing.T) {
	rs := []store.Rule{r(10, "*", "*", "wms", "TOPP", "DENY", "")}
	if d := Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "topp"}, true); d.Allow {
		t.Fatal("case-insensitive match failed")
	}
}
```

- [x] **Step 2: Run** → FAIL
- [x] **Step 3: Implement rules.go**

```go
// Package rules implements GeoFence-style access rule evaluation.
package rules

import (
	"strings"

	"github.com/giti/giti/services/auth/internal/store"
)

type Subject struct {
	Username string
	Roles    []string
}

type Query struct {
	Service, Request, Workspace, Layer string
}

type Decision struct {
	Allow      bool     `json:"allow"`
	CQLRead    string   `json:"cqlRead,omitempty"`
	CQLWrite   string   `json:"cqlWrite,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}

func fieldMatch(ruleVal, queryVal string) bool {
	return ruleVal == "*" || ruleVal == "" || strings.EqualFold(ruleVal, queryVal)
}

func roleMatch(ruleRole string, roles []string) bool {
	if ruleRole == "*" || ruleRole == "" {
		return true
	}
	for _, r := range roles {
		if strings.EqualFold(ruleRole, r) {
			return true
		}
	}
	return false
}

func matches(r store.Rule, sub Subject, q Query) bool {
	return fieldMatch(r.Username, sub.Username) &&
		roleMatch(r.Rolename, sub.Roles) &&
		fieldMatch(r.Service, q.Service) &&
		fieldMatch(r.Request, q.Request) &&
		fieldMatch(r.Workspace, q.Workspace) &&
		fieldMatch(r.Layer, q.Layer)
}

func andCQL(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return "(" + a + ") AND (" + b + ")"
}

// Evaluate walks priority-ordered rules. LIMIT matches accumulate
// constraints; the first matching ALLOW/DENY decides. No decision rule
// matched -> defaultAllow (constraints still apply when allowed).
func Evaluate(rs []store.Rule, sub Subject, q Query, defaultAllow bool) Decision {
	var d Decision
	for _, r := range rs {
		if !matches(r, sub, q) {
			continue
		}
		switch r.Access {
		case "LIMIT":
			d.CQLRead = andCQL(d.CQLRead, r.CQLRead)
			d.CQLWrite = andCQL(d.CQLWrite, r.CQLWrite)
			d.Attributes = append(d.Attributes, r.Attributes...)
		case "ALLOW":
			d.Allow = true
			return d
		case "DENY":
			return Decision{Allow: false}
		}
	}
	d.Allow = defaultAllow
	if !d.Allow {
		return Decision{Allow: false}
	}
	return d
}
```

- [x] **Step 4: Run** → PASS. **Commit** `git commit -m "feat(auth): geofence rule evaluation engine"`

---

### Task 5: JWT tokens + login + /check endpoint with Redis cache

**Files:**
- Create: `services/auth/internal/token/token.go`, `services/auth/internal/token/token_test.go`, `services/auth/internal/api/api.go`, `services/auth/internal/api/check.go`, `services/auth/internal/api/check_test.go`
- Modify: `services/auth/main.go` (mount)

**Interfaces:**
- Produces (package `token`):

```go
func Issue(secret []byte, username string, roles []string, ttl time.Duration) (string, error)
func Verify(secret []byte, tok string) (username string, roles []string, err error)
```

- Produces (package `api`):

```go
// Mount registers: POST /api/v1/auth/login {"username","password"} ->
//   {"token":"...","expiresIn":28800}  (401 on bad creds)
// GET /check  headers: Authorization (Basic|Bearer, optional),
//   X-Giti-Service, X-Giti-Request, X-Giti-Workspace, X-Giti-Layer
//   -> 200 {"allow":bool,"user":"...","roles":[...],"cqlRead":...}
//   -> 401 {"allow":false} when credentials present but invalid
// Security REST + geofence REST are registered by Task 6 files.
func Mount(mux *http.ServeMux, s *store.Store, rdb *redis.Client, secret []byte, defaultAllow bool)
```

- Redis cache: key `authz:{gen}:{user}:{svc}:{req}:{ws}:{layer}` TTL 60s; generation from `authz:gen` key, INCR'd by any /rest/security or /rest/geofence mutation (helper `bumpGen(ctx)` in api package). rdb == nil -> no caching (tests).

- [x] **Step 1: token test**

```go
package token

import (
	"testing"
	"time"
)

func TestIssueVerifyRoundtrip(t *testing.T) {
	secret := []byte("s")
	tok, err := Issue(secret, "alice", []string{"ROLE_EDITOR"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	user, roles, err := Verify(secret, tok)
	if err != nil || user != "alice" || len(roles) != 1 || roles[0] != "ROLE_EDITOR" {
		t.Fatalf("verify = %s %v %v", user, roles, err)
	}
	if _, _, err := Verify([]byte("other"), tok); err == nil {
		t.Fatal("wrong secret must fail")
	}
	expired, _ := Issue(secret, "alice", nil, -time.Minute)
	if _, _, err := Verify(secret, expired); err == nil {
		t.Fatal("expired must fail")
	}
}
```

- [x] **Step 2: Implement token.go**

```go
// Package token issues and verifies Giti JWTs (HS256).
package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type claims struct {
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

func Issue(secret []byte, username string, roles []string, ttl time.Duration) (string, error) {
	c := claims{
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "giti",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
}

func Verify(secret []byte, tok string) (string, []string, error) {
	var c claims
	t, err := jwt.ParseWithClaims(tok, &c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !t.Valid {
		return "", nil, errors.New("invalid token")
	}
	return c.Subject, c.Roles, nil
}
```

Run token tests → PASS.

- [x] **Step 3: api scaffolding + check + login (check_test.go needs DB; uses testMux helper)**

`api.go`:

```go
// Package api serves auth HTTP endpoints: login, authorization checks,
// and GeoServer-compatible security configuration REST.
package api

import (
	"context"
	"net/http"

	"github.com/giti/giti/services/auth/internal/store"
	"github.com/redis/go-redis/v9"
)

type api struct {
	s            *store.Store
	rdb          *redis.Client
	secret       []byte
	defaultAllow bool
}

func Mount(mux *http.ServeMux, s *store.Store, rdb *redis.Client, secret []byte, defaultAllow bool) {
	a := &api{s: s, rdb: rdb, secret: secret, defaultAllow: defaultAllow}
	a.checkRoutes(mux)
	a.securityRoutes(mux)
	a.geofenceRoutes(mux)
}

// bumpGen invalidates all cached authz decisions.
func (a *api) bumpGen(ctx context.Context) {
	if a.rdb != nil {
		a.rdb.Incr(ctx, "authz:gen")
	}
}
```

`check.go`:

```go
package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/giti/giti/services/auth/internal/password"
	"github.com/giti/giti/services/auth/internal/rules"
	"github.com/giti/giti/services/auth/internal/token"
)

const tokenTTL = 8 * time.Hour

func (a *api) checkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("GET /check", a.check)
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var body struct{ Username, Password string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	u, err := a.s.GetUser(r.Context(), body.Username)
	if err != nil || !u.Enabled || !password.Verify(body.Password, u.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	roles, err := a.s.RolesOf(r.Context(), u.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tok, err := token.Issue(a.secret, u.Name, roles, tokenTTL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token": tok, "expiresIn": int(tokenTTL.Seconds()),
	})
}

// authenticate resolves the Authorization header to a subject.
// ok=false means credentials were presented but are invalid.
func (a *api) authenticate(r *http.Request) (sub rules.Subject, ok bool) {
	h := r.Header.Get("Authorization")
	switch {
	case strings.HasPrefix(h, "Basic "):
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "Basic "))
		if err != nil {
			return sub, false
		}
		name, pw, found := strings.Cut(string(raw), ":")
		if !found {
			return sub, false
		}
		u, err := a.s.GetUser(r.Context(), name)
		if err != nil || !u.Enabled || !password.Verify(pw, u.PasswordHash) {
			return sub, false
		}
		roles, _ := a.s.RolesOf(r.Context(), name)
		return rules.Subject{Username: name, Roles: roles}, true
	case strings.HasPrefix(h, "Bearer "):
		name, roles, err := token.Verify(a.secret, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return sub, false
		}
		return rules.Subject{Username: name, Roles: roles}, true
	default:
		return rules.Subject{}, true // anonymous
	}
}

func (a *api) check(w http.ResponseWriter, r *http.Request) {
	sub, ok := a.authenticate(r)
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"allow": false})
		return
	}
	q := rules.Query{
		Service:   r.Header.Get("X-Giti-Service"),
		Request:   r.Header.Get("X-Giti-Request"),
		Workspace: r.Header.Get("X-Giti-Workspace"),
		Layer:     r.Header.Get("X-Giti-Layer"),
	}

	var cacheKey string
	if a.rdb != nil {
		gen, _ := a.rdb.Get(r.Context(), "authz:gen").Result()
		cacheKey = fmt.Sprintf("authz:%s:%s:%s:%s:%s:%s",
			gen, sub.Username, q.Service, q.Request, q.Workspace, q.Layer)
		if cached, err := a.rdb.Get(r.Context(), cacheKey).Result(); err == nil {
			w.Write([]byte(cached))
			return
		}
	}

	rs, err := a.s.ListRules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d := rules.Evaluate(rs, sub, q, a.defaultAllow)
	out := map[string]any{
		"allow": d.Allow, "user": sub.Username, "roles": sub.Roles,
	}
	if d.CQLRead != "" {
		out["cqlRead"] = d.CQLRead
	}
	if d.CQLWrite != "" {
		out["cqlWrite"] = d.CQLWrite
	}
	if len(d.Attributes) > 0 {
		out["attributes"] = d.Attributes
	}
	buf, _ := json.Marshal(out)
	if a.rdb != nil {
		a.rdb.Set(r.Context(), cacheKey, buf, 60*time.Second)
	}
	w.Write(buf)
}
```

`check_test.go`:

```go
package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/giti/giti/services/auth/internal/password"
	"github.com/giti/giti/services/auth/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testMux(t *testing.T, defaultAllow bool) (*http.ServeMux, *store.Store) {
	t.Helper()
	dsn := os.Getenv("GITI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GITI_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool.Exec(ctx, `TRUNCATE auth_users CASCADE`)
	pool.Exec(ctx, `TRUNCATE auth_groups CASCADE`)
	pool.Exec(ctx, `TRUNCATE auth_roles CASCADE`)
	pool.Exec(ctx, `TRUNCATE geofence_rules`)
	s := store.New(pool)
	mux := http.NewServeMux()
	Mount(mux, s, nil, []byte("test-secret"), defaultAllow)
	return mux, s
}

func basic(user, pw string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pw))
}

func seedUser(t *testing.T, s *store.Store, name, pw string, roles ...string) {
	t.Helper()
	ctx := context.Background()
	h, _ := password.Hash(pw)
	if err := s.CreateUser(ctx, store.User{Name: name, Enabled: true, PasswordHash: h}); err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		s.CreateRole(ctx, role)
		s.AssignRoleUser(ctx, role, name)
	}
}

func TestLoginAndBearerCheck(t *testing.T) {
	mux, s := testMux(t, true)
	seedUser(t, s, "alice", "pw123", "ROLE_EDITOR")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login",
		strings.NewReader(`{"username":"alice","password":"pw123"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("login = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/auth/login",
		strings.NewReader(`{"username":"alice","password":"nope"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("bad login = %d", rec.Code)
	}
}

func TestCheckDeniesByRule(t *testing.T) {
	mux, s := testMux(t, true)
	seedUser(t, s, "bob", "pw")
	ctx := context.Background()
	s.CreateRule(ctx, store.Rule{Priority: 10, Service: "WMS", Workspace: "secret", Access: "DENY"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", basic("bob", "pw"))
	req.Header.Set("X-Giti-Service", "WMS")
	req.Header.Set("X-Giti-Workspace", "secret")
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"allow":false`) {
		t.Fatalf("check = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", basic("bob", "pw"))
	req.Header.Set("X-Giti-Service", "WMS")
	req.Header.Set("X-Giti-Workspace", "open")
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"allow":true`) {
		t.Fatalf("open check = %s", rec.Body.String())
	}
}

func TestCheckBadCredentials401(t *testing.T) {
	mux, _ := testMux(t, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", basic("ghost", "x"))
	mux.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("check = %d", rec.Code)
	}
}

func TestCheckAnonymousUsesDefault(t *testing.T) {
	mux, _ := testMux(t, false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("X-Giti-Service", "WMS")
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"allow":false`) {
		t.Fatalf("anonymous default-deny = %s", rec.Body.String())
	}
}
```

Note: `Mount` references `securityRoutes`/`geofenceRoutes` (Task 6). To keep this task compiling, create stub files now that Task 6 replaces:

`security.go` (stub): `package api\n\nimport "net/http"\n\nfunc (a *api) securityRoutes(mux *http.ServeMux) {}`
`geofence.go` (stub): `package api\n\nimport "net/http"\n\nfunc (a *api) geofenceRoutes(mux *http.ServeMux) {}`

- [x] **Step 4: Wire Mount in main.go** `newHandler` (guard `d.db != nil`):

```go
	if d.db != nil {
		api.Mount(mux, store.New(d.db), d.rdb, d.secret, d.defaultAllow)
	}
```

- [x] **Step 5: Run all auth tests** → PASS. **Commit** `git commit -m "feat(auth): jwt login and /check authorization endpoint"`

---

### Task 6: /rest/security + /rest/geofence/rules compat REST

**Files:**
- Replace: `services/auth/internal/api/security.go`, `services/auth/internal/api/geofence.go`
- Create: `services/auth/internal/api/security_test.go`

**Interfaces:**
- GeoServer usergroup REST shapes (XML default, `.json` suffix for JSON — reuse pattern):
  - `GET /rest/security/usergroup/users` → `{"users":[{"userName":"admin","enabled":true}]}` / `<users><user><userName>admin</userName><enabled>true</enabled></user></users>`
  - `POST /rest/security/usergroup/users` body `{"user":{"userName":"x","password":"y","enabled":true}}` → 201
  - `DELETE /rest/security/usergroup/user/{u}` → 200
  - `GET /rest/security/roles` → `{"roles":["ADMIN"]}` / `<roles><role>ADMIN</role></roles>`
  - `POST /rest/security/roles/role/{r}` → 201; `DELETE /rest/security/roles/role/{r}` → 200
  - `POST /rest/security/roles/role/{r}/user/{u}` → 200 (associate)
- GeoFence REST (JSON): `GET /rest/geofence/rules` → `{"count":N,"rules":[{"id":1,"priority":10,"userName":"*","roleName":"*","service":"*","request":"*","workspace":"*","layer":"*","access":"ALLOW"}]}`; `POST /rest/geofence/rules` body `{"rule":{...}}` → 201 with id; `DELETE /rest/geofence/rules/id/{id}` → 200.
- All mutations call `a.bumpGen(r.Context())`. Both prefixes: mount under `/rest/...` and `/giti/rest/...` (register both patterns).

- [x] **Step 1: Failing tests** — `security_test.go`:

```go
package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func doReq(t *testing.T, mux http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestUserREST(t *testing.T) {
	mux, _ := testMux(t, true)
	rec := doReq(t, mux, "POST", "/rest/security/usergroup/users",
		`{"user":{"userName":"carol","password":"pw","enabled":true}}`)
	if rec.Code != 201 {
		t.Fatalf("POST user = %d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, mux, "GET", "/rest/security/usergroup/users.json", "")
	if !strings.Contains(rec.Body.String(), `"userName":"carol"`) {
		t.Fatalf("list = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("password leaked: %s", rec.Body.String())
	}
	rec = doReq(t, mux, "DELETE", "/rest/security/usergroup/user/carol", "")
	if rec.Code != 200 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
}

func TestRoleREST(t *testing.T) {
	mux, _ := testMux(t, true)
	doReq(t, mux, "POST", "/rest/security/usergroup/users",
		`{"user":{"userName":"dave","password":"pw","enabled":true}}`)
	rec := doReq(t, mux, "POST", "/rest/security/roles/role/ROLE_X", "")
	if rec.Code != 201 {
		t.Fatalf("POST role = %d", rec.Code)
	}
	rec = doReq(t, mux, "POST", "/rest/security/roles/role/ROLE_X/user/dave", "")
	if rec.Code != 200 {
		t.Fatalf("associate = %d", rec.Code)
	}
	rec = doReq(t, mux, "GET", "/rest/security/roles.json", "")
	if !strings.Contains(rec.Body.String(), "ROLE_X") {
		t.Fatalf("roles = %s", rec.Body.String())
	}
}

func TestGeofenceRulesREST(t *testing.T) {
	mux, _ := testMux(t, true)
	rec := doReq(t, mux, "POST", "/rest/geofence/rules",
		`{"rule":{"priority":10,"service":"WMS","workspace":"secret","access":"DENY"}}`)
	if rec.Code != 201 {
		t.Fatalf("POST rule = %d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, mux, "GET", "/rest/geofence/rules", "")
	body := rec.Body.String()
	if !strings.Contains(body, `"count":1`) || !strings.Contains(body, `"workspace":"secret"`) {
		t.Fatalf("rules = %s", body)
	}
}
```

(add `"net/http"` import)

- [x] **Step 2: Implement security.go**

```go
package api

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"strings"

	"github.com/giti/giti/services/auth/internal/password"
	"github.com/giti/giti/services/auth/internal/store"
)

func handleBoth(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
	// pattern like "GET /rest/..."; also register /giti-prefixed form.
	method, path, _ := strings.Cut(pattern, " ")
	mux.HandleFunc(pattern, h)
	mux.HandleFunc(method+" /giti"+path, h)
}

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

func wantsJSON(r *http.Request) bool {
	return strings.HasSuffix(r.URL.Path, ".json") ||
		strings.Contains(r.Header.Get("Accept"), "application/json")
}

func (a *api) securityRoutes(mux *http.ServeMux) {
	handleBoth(mux, "GET /rest/security/usergroup/users", a.listUsers)
	handleBoth(mux, "GET /rest/security/usergroup/users.json", a.listUsers)
	handleBoth(mux, "POST /rest/security/usergroup/users", a.createUser)
	handleBoth(mux, "DELETE /rest/security/usergroup/user/{u}", a.deleteUser)
	handleBoth(mux, "GET /rest/security/roles", a.listRoles)
	handleBoth(mux, "GET /rest/security/roles.json", a.listRoles)
	handleBoth(mux, "POST /rest/security/roles/role/{r}", a.createRole)
	handleBoth(mux, "DELETE /rest/security/roles/role/{r}", a.deleteRole)
	handleBoth(mux, "POST /rest/security/roles/role/{r}/user/{u}", a.associateRole)
}

type userJSON struct {
	UserName string `json:"userName" xml:"userName"`
	Enabled  bool   `json:"enabled" xml:"enabled"`
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.s.ListUsers(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	out := make([]userJSON, 0, len(users))
	for _, u := range users {
		out = append(out, userJSON{UserName: u.Name, Enabled: u.Enabled})
	}
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"users": out})
		return
	}
	type usersXML struct {
		XMLName struct{}   `xml:"users"`
		Items   []userJSON `xml:"user"`
	}
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(usersXML{Items: out})
}

func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		User struct {
			UserName string `json:"userName"`
			Password string `json:"password"`
			Enabled  bool   `json:"enabled"`
		} `json:"user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.User.UserName == "" {
		http.Error(w, "invalid user body", http.StatusBadRequest)
		return
	}
	hash, err := password.Hash(body.User.Password)
	if err != nil {
		httpErr(w, err)
		return
	}
	if err := a.s.CreateUser(r.Context(), store.User{
		Name: body.User.UserName, Enabled: body.User.Enabled, PasswordHash: hash}); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusCreated)
}

func (a *api) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteUser(r.Context(), r.PathValue("u")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}

func (a *api) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := a.s.ListRoles(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	if roles == nil {
		roles = []string{}
	}
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"roles": roles})
		return
	}
	type rolesXML struct {
		XMLName struct{} `xml:"roles"`
		Items   []string `xml:"role"`
	}
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(rolesXML{Items: roles})
}

func (a *api) createRole(w http.ResponseWriter, r *http.Request) {
	if err := a.s.CreateRole(r.Context(), r.PathValue("r")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusCreated)
}

func (a *api) deleteRole(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteRole(r.Context(), r.PathValue("r")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}

func (a *api) associateRole(w http.ResponseWriter, r *http.Request) {
	if err := a.s.AssignRoleUser(r.Context(), r.PathValue("r"), r.PathValue("u")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}
```

- [x] **Step 3: Implement geofence.go**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/giti/giti/services/auth/internal/store"
)

type ruleJSON struct {
	ID         int64    `json:"id,omitempty"`
	Priority   int64    `json:"priority"`
	UserName   string   `json:"userName,omitempty"`
	RoleName   string   `json:"roleName,omitempty"`
	Service    string   `json:"service,omitempty"`
	Request    string   `json:"request,omitempty"`
	Workspace  string   `json:"workspace,omitempty"`
	Layer      string   `json:"layer,omitempty"`
	Access     string   `json:"access"`
	CQLRead    string   `json:"cqlFilterRead,omitempty"`
	CQLWrite   string   `json:"cqlFilterWrite,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}

func (a *api) geofenceRoutes(mux *http.ServeMux) {
	handleBoth(mux, "GET /rest/geofence/rules", a.listRules)
	handleBoth(mux, "POST /rest/geofence/rules", a.createRule)
	handleBoth(mux, "DELETE /rest/geofence/rules/id/{id}", a.deleteRule)
}

func (a *api) listRules(w http.ResponseWriter, r *http.Request) {
	rs, err := a.s.ListRules(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	out := make([]ruleJSON, 0, len(rs))
	for _, ru := range rs {
		out = append(out, ruleJSON{ID: ru.ID, Priority: ru.Priority, UserName: ru.Username,
			RoleName: ru.Rolename, Service: ru.Service, Request: ru.Request,
			Workspace: ru.Workspace, Layer: ru.Layer, Access: ru.Access,
			CQLRead: ru.CQLRead, CQLWrite: ru.CQLWrite, Attributes: ru.Attributes})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"count": len(out), "rules": out})
}

func (a *api) createRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rule ruleJSON `json:"rule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid rule body", http.StatusBadRequest)
		return
	}
	b := body.Rule
	if b.Access != "ALLOW" && b.Access != "DENY" && b.Access != "LIMIT" {
		http.Error(w, "access must be ALLOW, DENY or LIMIT", http.StatusBadRequest)
		return
	}
	id, err := a.s.CreateRule(r.Context(), store.Rule{Priority: b.Priority,
		Username: b.UserName, Rolename: b.RoleName, Service: b.Service, Request: b.Request,
		Workspace: b.Workspace, Layer: b.Layer, Access: b.Access,
		CQLRead: b.CQLRead, CQLWrite: b.CQLWrite, Attributes: b.Attributes})
	if err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (a *api) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := a.s.DeleteRule(r.Context(), id); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}
```

- [x] **Step 4: Run all auth tests** → PASS. **Commit** `git commit -m "feat(auth): /rest/security and /rest/geofence compat endpoints"`

---

### Task 7: Gateway auth middleware + compose + e2e + docs + close out

**Files:**
- Create: `services/gateway/authz.go`, `services/gateway/authz_test.go`, `docs/services/auth.md`
- Modify: `services/gateway/main.go`, `deploy/compose/docker-compose.yml` (Task 1 already added auth service + gateway env), `docs/architecture.md`, `task.md`

**Interfaces:**
- Produces (gateway):

```go
// authzMiddleware calls GET {authURL}/check with Authorization forwarded and
// X-Giti-Service/Request/Workspace/Layer set from the parsed request.
// authURL == "" -> pass-through. Deny -> OWS exception (403) or 401 with
// WWW-Authenticate: Basic realm="giti" when anonymous.
// Allowed -> forwards, adding X-Giti-User, X-Giti-Roles,
// X-Giti-CQL-Read headers for downstream services.
func authzMiddleware(authURL string, next http.Handler) http.Handler
```

- [x] **Step 1: Failing test** — `services/gateway/authz_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeAuth(t *testing.T, decision map[string]any, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(decision)
	}))
}

func TestAuthzAllowsAndForwardsContext(t *testing.T) {
	authSrv := fakeAuth(t, map[string]any{"allow": true, "user": "alice",
		"roles": []string{"R"}, "cqlRead": "a=1"}, 200)
	defer authSrv.Close()
	var got *http.Request
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
	})
	h := authzMiddleware(authSrv.URL, inner)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET",
		"/giti/topp/wms?service=WMS&request=GetMap", nil))
	if got == nil {
		t.Fatal("request not forwarded")
	}
	if got.Header.Get("X-Giti-User") != "alice" || got.Header.Get("X-Giti-CQL-Read") != "a=1" {
		t.Fatalf("headers = %v", got.Header)
	}
}

func TestAuthzDeniedAnonymous401(t *testing.T) {
	authSrv := fakeAuth(t, map[string]any{"allow": false}, 200)
	defer authSrv.Close()
	h := authzMiddleware(authSrv.URL, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET",
		"/giti/wms?service=WMS&request=GetMap", nil))
	if rec.Code != 401 || rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("anon deny = %d %v", rec.Code, rec.Header())
	}
}

func TestAuthzDeniedAuthenticated403(t *testing.T) {
	authSrv := fakeAuth(t, map[string]any{"allow": false, "user": "bob"}, 200)
	defer authSrv.Close()
	h := authzMiddleware(authSrv.URL, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/giti/wms?service=WMS&request=GetMap", nil)
	req.Header.Set("Authorization", "Basic Ym9iOnB3")
	h.ServeHTTP(rec, req)
	if rec.Code != 200 && rec.Code != 403 { // WMS exceptions are HTTP 200
		t.Fatalf("authed deny = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Access denied") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAuthzInvalidCreds401(t *testing.T) {
	authSrv := fakeAuth(t, map[string]any{"allow": false}, 401)
	defer authSrv.Close()
	h := authzMiddleware(authSrv.URL, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/giti/wms?service=WMS&request=GetMap", nil)
	req.Header.Set("Authorization", "Basic YmFkOmNyZWRz")
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("invalid creds = %d", rec.Code)
	}
}

func TestAuthzPassThroughWithoutURL(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	authzMiddleware("", inner).ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest("GET", "/giti/wms", nil))
	if !called {
		t.Fatal("pass-through failed")
	}
}
```

- [x] **Step 2: Implement authz.go**

```go
package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/giti/giti/libs/ogc-kit/ows"
)

type authDecision struct {
	Allow      bool     `json:"allow"`
	User       string   `json:"user"`
	Roles      []string `json:"roles"`
	CQLRead    string   `json:"cqlRead"`
	CQLWrite   string   `json:"cqlWrite"`
	Attributes []string `json:"attributes"`
}

var authClient = &http.Client{Timeout: 5 * time.Second}

func authzMiddleware(authURL string, next http.Handler) http.Handler {
	if authURL == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := ows.ParseKVP(r.URL.Query())
		wsName, layer, _ := parsePath(r.URL.Path)

		checkReq, _ := http.NewRequestWithContext(r.Context(), "GET", authURL+"/check", nil)
		if h := r.Header.Get("Authorization"); h != "" {
			checkReq.Header.Set("Authorization", h)
		}
		checkReq.Header.Set("X-Giti-Service", req.Service)
		checkReq.Header.Set("X-Giti-Request", req.Request)
		checkReq.Header.Set("X-Giti-Workspace", wsName)
		checkReq.Header.Set("X-Giti-Layer", layer)

		resp, err := authClient.Do(checkReq)
		if err != nil {
			ows.WriteException(w, req.Service, req.Version, req.Get("EXCEPTIONS"),
				ows.ServiceError{Code: ows.CodeNoApplicableCode,
					Message: "Authorization service unavailable", Status: 503})
			return
		}
		defer resp.Body.Close()
		var d authDecision
		json.NewDecoder(resp.Body).Decode(&d)

		if resp.StatusCode == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="giti"`)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if !d.Allow {
			if d.User == "" { // anonymous: challenge for credentials
				w.Header().Set("WWW-Authenticate", `Basic realm="giti"`)
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			ows.WriteException(w, req.Service, req.Version, req.Get("EXCEPTIONS"),
				ows.ServiceError{Code: ows.CodeNoApplicableCode,
					Message: "Access denied", Status: 403})
			return
		}
		r.Header.Set("X-Giti-User", d.User)
		if len(d.Roles) > 0 {
			buf, _ := json.Marshal(d.Roles)
			r.Header.Set("X-Giti-Roles", string(buf))
		}
		if d.CQLRead != "" {
			r.Header.Set("X-Giti-CQL-Read", d.CQLRead)
		}
		if d.CQLWrite != "" {
			r.Header.Set("X-Giti-CQL-Write", d.CQLWrite)
		}
		next.ServeHTTP(w, r)
	})
}
```

Wire in `main.go` `newHandlerWith` (dispatcher chain):

```go
	mux.Handle("/giti/",
		rateLimitMiddleware(limit, burst,
			metricsMiddleware(
				authzMiddleware(os.Getenv("GITI_AUTH_URL"), newDispatcher(b)))))
```

- [x] **Step 3: Run gateway tests** → PASS. Rebuild stack:

```bash
cd deploy/compose && docker compose up -d --build gateway auth
```

- [x] **Step 4: e2e**

```bash
# login with default admin
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"geoserver"}' http://localhost/api/v1/auth/login
# create DENY rule for workspace "secret"
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"rule":{"priority":10,"workspace":"secret","access":"DENY"}}' \
  http://localhost/giti/rest/geofence/rules
# anonymous request to secret workspace -> 401 challenge
curl -s -o /dev/null -w '%{http_code}\n' \
  "http://localhost/giti/secret/wms?service=WMS&request=GetMap"       # 401
# open workspace still proxies (503/404 from wms stub is fine, not 401)
curl -s -o /dev/null -w '%{http_code}\n' \
  "http://localhost/giti/open/wms?service=WMS&request=GetMap"
# security REST
curl -s http://localhost/giti/rest/security/roles.json
```

- [x] **Step 5: docs/services/auth.md**

```markdown
# auth

Authentication + GeoFence-style authorization. Go + Postgres + Redis.

## Endpoints
- `POST /api/v1/auth/login` → JWT (8h)
- `GET /check` (internal; gateway calls per OWS request)
- `/rest/security/usergroup/*`, `/rest/security/roles*` — GeoServer compat
- `/rest/geofence/rules` — GeoFence-style rules (priority, first ALLOW/DENY wins,
  LIMIT rules accumulate CQL/attribute constraints)

## Defaults
- Seeded admin: `admin` / `geoserver` (change immediately)
- No matching rule → `GITI_AUTH_DEFAULT` (ALLOW; set DENY to lock down)
- Decisions cached in Redis 60s; any security mutation invalidates (generation counter)

## Env
GITI_HTTP_ADDR, GITI_DATABASE_URL, GITI_REDIS_URL, GITI_JWT_SECRET,
GITI_AUTH_DEFAULT
```

- [x] **Step 6: architecture.md auth row → done; task.md Sprint 4 → [x]; final verify + commit**

```bash
go vet github.com/giti/giti/... && \
GITI_TEST_DATABASE_URL=postgres://giti:giti-dev-password@127.0.0.1:5433/giti \
  go test github.com/giti/giti/...
git add -A && git commit -m "feat(auth): gateway authz middleware, e2e, docs; complete sprint 4"
```
