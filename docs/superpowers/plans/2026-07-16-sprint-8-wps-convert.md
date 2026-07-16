# Sprint 8 — WPS + Convert Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Two Go services. `wps`: WPS 1.0 (GetCapabilities/DescribeProcess/Execute sync+async), a process set (buffer, centroid, area, length, reproject, intersection, union, simplify) computed via PostGIS, NATS-queued async jobs with status polling and results stored to a shared volume. `convert`: file ingest pipeline (upload → register store + auto-publish layer via catalog REST) with SSE progress.

**Architecture:** Both reuse `libs/ogc-kit` (health, ows). WPS geometry processes push down to PostGIS: input geometry as WKT → `SELECT ST_AsText(ST_Buffer(ST_GeomFromText($1), $2))` etc. — no in-process geometry engine needed for v1 (the DB is the compute). Execute mode=sync runs inline; mode=async enqueues a NATS job, a worker pool executes and writes results + status JSON to the results dir; clients poll a status URL. `convert` validates uploads with the same connectors as catalog (magic bytes), copies the file into a shared data volume, then calls catalog `/rest` to create a store + featuretype (which auto-publishes a layer), streaming progress over SSE.

**Tech Stack:** Go 1.26, pgx/v5 (PostGIS ops), nats.go (async queue), `libs/ogc-kit`. No new heavy deps.

## Global Constraints

- Health convention, `GEOSON_HTTP_ADDR`, `health.Serve`; Dockerfile `GOWORK=off` copying own module + `libs/ogc-kit/` (established Go pattern); GOPROXY `https://goproxy.cn`.
- Go tests `go test github.com/geoson/geoson/...`; integration tests skip without `GEOSON_TEST_DATABASE_URL` (`127.0.0.1:5433`).
- WPS exceptions use `ows.WriteException` with service "WPS" (ows 1.1 ExceptionReport). Gateway proxies `/geoserver/wps` → `wps:8080/wps` (endpointService already maps `wps`→WPS, needs `GEOSON_WPS_URL`).
- Async job results + status under `GEOSON_WPS_RESULTS_DIR` (default `/var/lib/geoson/wps`), shared volume `wpsresults`.
- Convert copies uploads to `GEOSON_DATA_DIR` (default `/var/lib/geoson/data`), shared volume `geosondata`; calls catalog at `GEOSON_CATALOG_URL` (default `http://catalog:8080`).
- SQL safety: geometry/number inputs bound as parameters; no string interpolation of user WKT.
- Commit after every task, Conventional Commits.

## File Structure

```
services/wps/
  go.mod  main.go  main_test.go  Dockerfile
  internal/process/process.go     # Process registry + Param types
  internal/process/geometry.go    # PostGIS-backed processes (buffer/centroid/...)
  internal/process/process_test.go
  internal/wps/wps.go             # dispatch: GetCapabilities/DescribeProcess/Execute
  internal/wps/execute.go         # sync exec + async enqueue
  internal/wps/jobs.go            # NATS worker + status store (filesystem)
  internal/wps/*_test.go
services/convert/
  go.mod  main.go  main_test.go  Dockerfile
  internal/ingest/ingest.go       # validate + copy + catalog publish
  internal/ingest/ingest_test.go
  internal/api/api.go             # POST /api/v1/convert/import (multipart) + SSE
  internal/api/api_test.go
```

---

### Task 1: WPS service scaffold + process registry

**Files:**
- Create: `services/wps/go.mod`, `services/wps/main.go`, `services/wps/main_test.go`, `services/wps/Dockerfile`, `services/wps/internal/process/process.go`, `services/wps/internal/process/process_test.go`
- Modify: `go.work`, `deploy/compose/docker-compose.yml`, `.github/workflows/ci.yml`, gateway env (`GEOSON_WPS_URL`)

**Interfaces:**
- Produces (`process` package):

```go
type ParamKind int
const ( KindGeometry ParamKind = iota; KindDouble; KindString )
type Param struct { Name, Title string; Kind ParamKind; Required bool }
type Process struct {
    Identifier, Title, Abstract string
    Inputs  []Param
    Outputs []Param
    // Run executes the process. inputs maps param name -> raw string value
    // (geometry = WKT). Returns the output value (WKT or scalar as string).
    Run func(ctx context.Context, db *pgxpool.Pool, inputs map[string]string) (string, error)
}
func Registry() map[string]Process   // identifier -> Process
func Get(id string) (Process, bool)
```

- `main.go` copies auth's shape: health (postgres check), `GEOSON_DATABASE_URL` → pool, mounts `wps.Mount` (Task 4). Dockerfile = auth's with s/auth/wps/.

- [x] **Step 1: Init module**

```bash
cd /home/madson/geoson
mkdir -p services/wps
( cd services/wps && go mod init github.com/geoson/geoson/services/wps )
go work use ./services/wps
cd services/wps
go mod edit -require=github.com/geoson/geoson/libs/ogc-kit@v0.0.0
go mod edit -replace=github.com/geoson/geoson/libs/ogc-kit=../../libs/ogc-kit
go get github.com/jackc/pgx/v5/pgxpool@latest github.com/nats-io/nats.go@latest
```

- [x] **Step 2: Failing registry test** — `internal/process/process_test.go`:

```go
package process

import "testing"

func TestRegistryHasCoreProcesses(t *testing.T) {
	r := Registry()
	for _, id := range []string{"geoson:buffer", "geoson:centroid", "geoson:area",
		"geoson:length", "geoson:reproject", "geoson:intersection", "geoson:union",
		"geoson:simplify"} {
		if _, ok := r[id]; !ok {
			t.Fatalf("missing process %s", id)
		}
	}
	p, ok := Get("geoson:buffer")
	if !ok || len(p.Inputs) < 2 {
		t.Fatalf("buffer process = %+v", p)
	}
}
```

Run: `go test ./internal/process/` → FAIL

- [x] **Step 3: Implement process.go** (registry skeleton; geometry.go Task 2 fills Run funcs — but registry references them). To keep this task self-contained, define the registry with the process metadata AND stub Run funcs returning `errors.New("not implemented")`; Task 2 replaces the Run funcs. Provide:

```go
// Package process defines WPS processes backed by PostGIS.
package process

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ParamKind int

const (
	KindGeometry ParamKind = iota
	KindDouble
	KindString
)

type Param struct {
	Name, Title string
	Kind        ParamKind
	Required    bool
}

type Process struct {
	Identifier, Title, Abstract string
	Inputs                      []Param
	Outputs                     []Param
	Run                         func(ctx context.Context, db *pgxpool.Pool, inputs map[string]string) (string, error)
}

var notImpl = func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return "", errors.New("not implemented")
}

func geom(name string, req bool) Param  { return Param{Name: name, Title: name, Kind: KindGeometry, Required: req} }
func dbl(name string, req bool) Param    { return Param{Name: name, Title: name, Kind: KindDouble, Required: req} }
func str(name string, req bool) Param    { return Param{Name: name, Title: name, Kind: KindString, Required: req} }

var registry = map[string]Process{
	"geoson:buffer": {Identifier: "geoson:buffer", Title: "Buffer", Abstract: "Buffer a geometry by a distance",
		Inputs: []Param{geom("geom", true), dbl("distance", true)}, Outputs: []Param{geom("result", true)}, Run: runBuffer},
	"geoson:centroid": {Identifier: "geoson:centroid", Title: "Centroid", Abstract: "Centroid of a geometry",
		Inputs: []Param{geom("geom", true)}, Outputs: []Param{geom("result", true)}, Run: runCentroid},
	"geoson:area": {Identifier: "geoson:area", Title: "Area", Abstract: "Area of a geometry",
		Inputs: []Param{geom("geom", true)}, Outputs: []Param{dbl("result", true)}, Run: runArea},
	"geoson:length": {Identifier: "geoson:length", Title: "Length", Abstract: "Length/perimeter of a geometry",
		Inputs: []Param{geom("geom", true)}, Outputs: []Param{dbl("result", true)}, Run: runLength},
	"geoson:reproject": {Identifier: "geoson:reproject", Title: "Reproject", Abstract: "Transform a geometry between SRIDs",
		Inputs: []Param{geom("geom", true), str("sourceSRID", true), str("targetSRID", true)}, Outputs: []Param{geom("result", true)}, Run: runReproject},
	"geoson:intersection": {Identifier: "geoson:intersection", Title: "Intersection", Abstract: "Intersection of two geometries",
		Inputs: []Param{geom("a", true), geom("b", true)}, Outputs: []Param{geom("result", true)}, Run: runIntersection},
	"geoson:union": {Identifier: "geoson:union", Title: "Union", Abstract: "Union of two geometries",
		Inputs: []Param{geom("a", true), geom("b", true)}, Outputs: []Param{geom("result", true)}, Run: runUnion},
	"geoson:simplify": {Identifier: "geoson:simplify", Title: "Simplify", Abstract: "Douglas-Peucker simplify",
		Inputs: []Param{geom("geom", true), dbl("tolerance", true)}, Outputs: []Param{geom("result", true)}, Run: runSimplify},
}

// Run funcs are defined in geometry.go; declared here as stubs replaced there.
var (
	runBuffer       = notImpl
	runCentroid     = notImpl
	runArea         = notImpl
	runLength       = notImpl
	runReproject    = notImpl
	runIntersection = notImpl
	runUnion        = notImpl
	runSimplify     = notImpl
)

func Registry() map[string]Process { return registry }
func Get(id string) (Process, bool) { p, ok := registry[id]; return p, ok }
```

- [x] **Step 4: Implement main.go** (copy auth main.go shape: pool + health; add NATS conn; mount stubbed — `wps.Mount` created Task 4, so for this task keep main referencing only health and add a `// mount in Task 4` comment; ensure it compiles by not importing wps yet).
- [x] **Step 5: Dockerfile** (auth's, s/auth/wps/). **Compose**: add `wps` service (DB url, NATS url, `GEOSON_WPS_RESULTS_DIR: /var/lib/geoson/wps`, volume `wpsresults:/var/lib/geoson/wps`), gateway env `GEOSON_WPS_URL: http://wps:8080`, add `wpsresults` to named volumes. CI docker-build: wps line.
- [x] **Step 6: Run** `go test github.com/geoson/geoson/services/wps/...`; `docker compose config -q`. **Commit** `git commit -m "feat(wps): service scaffold and process registry"`

---

### Task 2: PostGIS-backed geometry processes

**Files:**
- Create: `services/wps/internal/process/geometry.go`, `services/wps/internal/process/geometry_test.go`

**Interfaces:**
- Consumes: registry Run stubs from Task 1 (replaces the `var runBuffer = notImpl` bindings with real funcs).
- Produces: real `runBuffer/...` implementations. Each is `func(ctx, db, inputs) (string, error)`. Geometry inputs are WKT; outputs: geometry processes return WKT, area/length return a formatted float string.

Implementations (PostGIS one-liners, all parameterized):
- buffer: `SELECT ST_AsText(ST_Buffer(ST_GeomFromText($1), $2))` args geom, distance(float)
- centroid: `SELECT ST_AsText(ST_Centroid(ST_GeomFromText($1)))`
- area: `SELECT ST_Area(ST_GeomFromText($1))` → format float
- length: `SELECT ST_Length(ST_GeomFromText($1))` → format float; for polygons use ST_Perimeter — v1: `ST_Length` works for lines, `ST_Perimeter` for polygons; use `COALESCE(NULLIF(ST_Length(g),0), ST_Perimeter(g))` via a CTE.
- reproject: `SELECT ST_AsText(ST_Transform(ST_SetSRID(ST_GeomFromText($1), $2::int), $3::int))` args geom, sourceSRID, targetSRID
- intersection: `SELECT ST_AsText(ST_Intersection(ST_GeomFromText($1), ST_GeomFromText($2)))`
- union: `SELECT ST_AsText(ST_Union(ST_GeomFromText($1), ST_GeomFromText($2)))`
- simplify: `SELECT ST_AsText(ST_Simplify(ST_GeomFromText($1), $2))` args geom, tolerance

- [x] **Step 1: Failing tests** — `geometry_test.go` (needs DB):

```go
package process

import (
	"context"
	"os"
	"strings"
	"testing"

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
	pool.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS postgis")
	t.Cleanup(pool.Close)
	return pool
}

func TestBufferAndArea(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	p, _ := Get("geoson:buffer")
	out, err := p.Run(ctx, db, map[string]string{"geom": "POINT(0 0)", "distance": "1"})
	if err != nil || !strings.HasPrefix(out, "POLYGON") {
		t.Fatalf("buffer = %q, %v", out, err)
	}
	pa, _ := Get("geoson:area")
	area, err := pa.Run(ctx, db, map[string]string{"geom": "POLYGON((0 0,0 2,2 2,2 0,0 0))"})
	if err != nil || !strings.HasPrefix(area, "4") {
		t.Fatalf("area = %q, %v", area, err)
	}
}

func TestCentroidReprojectIntersectionUnionSimplify(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	cases := []struct {
		id     string
		inputs map[string]string
		prefix string
	}{
		{"geoson:centroid", map[string]string{"geom": "POLYGON((0 0,0 2,2 2,2 0,0 0))"}, "POINT"},
		{"geoson:reproject", map[string]string{"geom": "POINT(0 0)", "sourceSRID": "4326", "targetSRID": "3857"}, "POINT"},
		{"geoson:intersection", map[string]string{"a": "POLYGON((0 0,0 2,2 2,2 0,0 0))", "b": "POLYGON((1 1,1 3,3 3,3 1,1 1))"}, "POLYGON"},
		{"geoson:union", map[string]string{"a": "POINT(0 0)", "b": "POINT(1 1)"}, "MULTIPOINT"},
		{"geoson:simplify", map[string]string{"geom": "LINESTRING(0 0,1 0.1,2 0)", "tolerance": "0.5"}, "LINESTRING"},
	}
	for _, c := range cases {
		p, _ := Get(c.id)
		out, err := p.Run(ctx, db, c.inputs)
		if err != nil || !strings.HasPrefix(out, c.prefix) {
			t.Fatalf("%s = %q, %v (want %s)", c.id, out, err, c.prefix)
		}
	}
	pl, _ := Get("geoson:length")
	l, err := pl.Run(ctx, db, map[string]string{"geom": "LINESTRING(0 0,3 0)"})
	if err != nil || !strings.HasPrefix(l, "3") {
		t.Fatalf("length = %q, %v", l, err)
	}
}
```

- [x] **Step 2: Run** (with DB) → FAIL. **Step 3: Implement geometry.go** — assign the real funcs to the package vars in an `init()` or by direct `var runBuffer = func(...){...}` (replace the stub declarations by moving them here; simplest: keep stubs in process.go but reassign in `init()` in geometry.go). Use `init()`:

```go
package process

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

func init() {
	runBuffer = wkt1f("SELECT ST_AsText(ST_Buffer(ST_GeomFromText($1), $2))", "geom", "distance")
	runCentroid = wkt0("SELECT ST_AsText(ST_Centroid(ST_GeomFromText($1)))", "geom")
	runArea = scalar("SELECT ST_Area(ST_GeomFromText($1))", "geom")
	runLength = scalar(
		"SELECT CASE WHEN ST_Length(g)=0 THEN ST_Perimeter(g) ELSE ST_Length(g) END FROM (SELECT ST_GeomFromText($1) g) s", "geom")
	runReproject = func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out string
		err := db.QueryRow(ctx,
			"SELECT ST_AsText(ST_Transform(ST_SetSRID(ST_GeomFromText($1), $2::int), $3::int))",
			in["geom"], in["sourceSRID"], in["targetSRID"]).Scan(&out)
		return out, err
	}
	runIntersection = wkt2("SELECT ST_AsText(ST_Intersection(ST_GeomFromText($1), ST_GeomFromText($2)))", "a", "b")
	runUnion = wkt2("SELECT ST_AsText(ST_Union(ST_GeomFromText($1), ST_GeomFromText($2)))", "a", "b")
	runSimplify = wkt1f("SELECT ST_AsText(ST_Simplify(ST_GeomFromText($1), $2))", "geom", "tolerance")
}

func wkt0(sql, g string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out string
		err := db.QueryRow(ctx, sql, in[g]).Scan(&out)
		return out, err
	}
}
func wkt1f(sql, g, f string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		val, err := strconv.ParseFloat(in[f], 64)
		if err != nil {
			return "", err
		}
		var out string
		err = db.QueryRow(ctx, sql, in[g], val).Scan(&out)
		return out, err
	}
}
func wkt2(sql, a, b string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out string
		err := db.QueryRow(ctx, sql, in[a], in[b]).Scan(&out)
		return out, err
	}
}
func scalar(sql, g string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out float64
		err := db.QueryRow(ctx, sql, in[g]).Scan(&out)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(out, 'f', -1, 64), nil
	}
}
```

- [x] **Step 4: Run** (with DB) → PASS. **Commit** `git commit -m "feat(wps): postgis-backed geometry processes"`

---

### Task 3: Async job store (NATS worker + filesystem status)

**Files:**
- Create: `services/wps/internal/wps/jobs.go`, `services/wps/internal/wps/jobs_test.go`

**Interfaces:**
- Produces (`wps` package):

```go
type JobStatus struct {
	ID       string `json:"id"`
	Process  string `json:"process"`
	Status   string `json:"status"`   // accepted | running | succeeded | failed
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}
type Jobs struct { /* dir, nats conn, db */ }
func NewJobs(dir string, nc *nats.Conn, db *pgxpool.Pool) *Jobs
// Enqueue creates a job, writes status "accepted", publishes to NATS, returns id.
func (j *Jobs) Enqueue(ctx context.Context, procID string, inputs map[string]string) (string, error)
// Status reads a job's status JSON from disk.
func (j *Jobs) Status(id string) (JobStatus, error)
// RunWorker subscribes and executes jobs until ctx is done (call in a goroutine).
func (j *Jobs) RunWorker(ctx context.Context) error
// execNow runs a job synchronously (used by sync Execute and by the worker).
func (j *Jobs) execNow(ctx context.Context, procID string, inputs map[string]string) (string, error)
```

Job payload on NATS subject `wps.jobs`: JSON `{"id","process","inputs":{...}}`. Worker: on message, write status "running", run the process via `process.Get(...).Run`, write "succeeded"+output or "failed"+error. Status stored as `{dir}/{id}.json`. `execNow` shared by sync path.

- [x] **Step 1: Failing test** — `jobs_test.go` (DB; NATS optional — test enqueue+execNow without a running worker):

```go
package wps

import (
	"context"
	"os"
	"strings"
	"testing"

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
	pool.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS postgis")
	t.Cleanup(pool.Close)
	return pool
}

func TestExecNowAndStatus(t *testing.T) {
	db := testDB(t)
	dir := t.TempDir()
	j := NewJobs(dir, nil, db)
	out, err := j.execNow(context.Background(), "geoson:buffer",
		map[string]string{"geom": "POINT(0 0)", "distance": "1"})
	if err != nil || !strings.HasPrefix(out, "POLYGON") {
		t.Fatalf("execNow = %q, %v", out, err)
	}
}

func TestEnqueueWritesAcceptedStatus(t *testing.T) {
	db := testDB(t)
	dir := t.TempDir()
	j := NewJobs(dir, nil, db) // nil nats -> Enqueue skips publish
	id, err := j.Enqueue(context.Background(), "geoson:centroid",
		map[string]string{"geom": "POLYGON((0 0,0 2,2 2,2 0,0 0))"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := j.Status(id)
	if err != nil || st.Status != "accepted" || st.Process != "geoson:centroid" {
		t.Fatalf("status = %+v, %v", st, err)
	}
}
```

- [x] **Step 2: Run** (DB) → FAIL. **Step 3: Implement jobs.go** (uuid via `crypto/rand` hex; `os.WriteFile` status JSON; `execNow` calls `process.Get(procID).Run`; `Enqueue` writes accepted then `nc.Publish` when nc != nil; `RunWorker` subscribes `wps.jobs`, on message runs execNow, updates status). **Step 4: Run** → PASS. **Commit** `git commit -m "feat(wps): async job store with nats worker"`

---

### Task 4: WPS handlers (GetCapabilities/DescribeProcess/Execute)

**Files:**
- Create: `services/wps/internal/wps/wps.go`, `services/wps/internal/wps/execute.go`, `services/wps/internal/wps/wps_test.go`
- Modify: `services/wps/main.go` (mount + start worker)

**Interfaces:**
- Produces: `func Mount(mux *http.ServeMux, jobs *Jobs)` mounting `GET|POST /wps`. Dispatch on REQUEST (KVP via `ows.ParseKVP`, or POST XML via `ows.ParseXML` for Execute):
  - GetCapabilities → WPS 1.0 Capabilities XML listing processes from `process.Registry()`.
  - DescribeProcess → `IDENTIFIER` param → ProcessDescriptions XML with inputs/outputs.
  - Execute → `IDENTIFIER` + data inputs. KVP `DataInputs=geom=POINT(0 0);distance=1` (semicolon-separated name=value) OR POST XML `wps:Execute`. `mode`/`storeExecuteResponse`: sync (default) runs `jobs.execNow` and returns `wps:ExecuteResponse` with the literal/complex output; async (`storeExecuteResponse=true` or `mode=async`) → `jobs.Enqueue`, returns `ExecuteResponse` with `statusLocation` = `/wps/status/{id}` and status "accepted".
  - `GET /wps/status/{id}` → job status as JSON (and/or WPS ExecuteResponse XML).
- Unknown process → `ows.WriteException(WPS, ..., "InvalidParameterValue", "identifier")`.

- [x] **Step 1: Failing tests** — `wps_test.go` (DB):

```go
package wps

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testMux(t *testing.T) *http.ServeMux {
	t.Helper()
	db := testDB(t)
	mux := http.NewServeMux()
	Mount(mux, NewJobs(t.TempDir(), nil, db))
	return mux
}

func get(t *testing.T, mux *http.ServeMux, q string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/wps?"+q, nil))
	return rec
}

func TestCapabilities(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=GetCapabilities")
	if !strings.Contains(rec.Body.String(), "geoson:buffer") ||
		!strings.Contains(rec.Body.String(), "Capabilities") {
		t.Fatalf("caps = %s", rec.Body.String())
	}
}

func TestDescribeProcess(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=DescribeProcess&identifier=geoson:buffer")
	body := rec.Body.String()
	if !strings.Contains(body, "geoson:buffer") || !strings.Contains(body, "distance") {
		t.Fatalf("describe = %s", body)
	}
}

func TestExecuteSyncBuffer(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=Execute&identifier=geoson:buffer"+
		"&DataInputs=geom%3DPOINT(0%200)%3Bdistance%3D1")
	body := rec.Body.String()
	if !strings.Contains(body, "ExecuteResponse") || !strings.Contains(body, "POLYGON") {
		t.Fatalf("execute = %s", body)
	}
}

func TestExecuteAsyncAndPoll(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=Execute&identifier=geoson:centroid"+
		"&DataInputs=geom%3DPOLYGON((0%200%2C0%202%2C2%202%2C2%200%2C0%200))&storeExecuteResponse=true")
	body := rec.Body.String()
	if !strings.Contains(body, "statusLocation") {
		t.Fatalf("async execute = %s", body)
	}
	// extract id from statusLocation=/wps/status/{id}
	i := strings.Index(body, "/wps/status/")
	id := body[i+len("/wps/status/"):]
	id = strings.FieldsFunc(id, func(r rune) bool { return r == '"' || r == '<' || r == ' ' })[0]
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/wps/status/"+id, nil))
	if !strings.Contains(rec.Body.String(), "accepted") &&
		!strings.Contains(rec.Body.String(), "succeeded") {
		t.Fatalf("status = %s", rec.Body.String())
	}
}

func TestExecuteUnknownProcess(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=Execute&identifier=geoson:ghost&DataInputs=x%3D1")
	if !strings.Contains(rec.Body.String(), "ExceptionReport") {
		t.Fatalf("unknown = %s", rec.Body.String())
	}
}
```

- [x] **Step 2: Run** (DB) → FAIL. **Step 3: Implement wps.go + execute.go.** DataInputs KVP parse: split on `;`, each `name=value`. Build ExecuteResponse XML with `<wps:ProcessOutputs><wps:Output><ows:Identifier>result</ows:Identifier><wps:Data><wps:LiteralData>VALUE</wps:LiteralData></wps:Data></wps:Output>`. Mount in main.go and `go j.RunWorker(ctx)` when NATS present. **Step 4: Run** → PASS. **Commit** `git commit -m "feat(wps): getcapabilities/describeprocess/execute sync+async"`

---

### Task 5: Convert service scaffold + ingest pipeline

**Files:**
- Create: `services/convert/go.mod`, `services/convert/main.go`, `services/convert/main_test.go`, `services/convert/Dockerfile`, `services/convert/internal/ingest/ingest.go`, `services/convert/internal/ingest/ingest_test.go`
- Modify: `go.work`, `deploy/compose/docker-compose.yml`, `.github/workflows/ci.yml`

**Interfaces:**
- Produces (`ingest` package):

```go
type Result struct { Workspace, Store, Layer, StoredPath string }
// DetectType returns a Geoson store type from a filename (.shp/.gpkg/.geojson/.csv/.tif).
func DetectType(filename string) (string, error)
// Import copies the uploaded bytes into dataDir, then registers a store +
// featuretype via the catalog REST API (catalogURL), auto-publishing a layer.
// progress receives human-readable step messages.
func Import(ctx context.Context, catalogURL, dataDir, workspace, filename string,
	data []byte, progress func(string)) (Result, error)
```

Catalog calls: POST `/rest/workspaces` (idempotent — ignore 409), POST `/rest/workspaces/{ws}/datastores` with the detected type + `connectionParameters` `url=file:///{stored}` (or `host=self` for CSV→Postgres? v1: file stores use `url`), POST `/rest/workspaces/{ws}/datastores/{store}/featuretypes` with the layer name = base filename. Store name = base filename; layer auto-published by catalog.

- [x] **Step 1: Init module + deps** (go mod init, go work use, ogc-kit replace; `go get` none beyond stdlib + net/http).
- [x] **Step 2: Failing test** — `ingest_test.go` (uses a fake catalog httptest server, no DB):

```go
package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDetectType(t *testing.T) {
	cases := map[string]string{
		"roads.shp": "Shapefile", "data.gpkg": "GeoPackage",
		"pts.geojson": "GeoJSON", "table.csv": "CSV", "dem.tif": "GeoTIFF",
	}
	for f, want := range cases {
		got, err := DetectType(f)
		if err != nil || got != want {
			t.Fatalf("%s -> %s, %v (want %s)", f, got, err, want)
		}
	}
	if _, err := DetectType("x.docx"); err == nil {
		t.Fatal("want error for unsupported")
	}
}

func TestImportCallsCatalog(t *testing.T) {
	var paths []string
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer catalog.Close()
	dir := t.TempDir()
	var steps []string
	res, err := Import(context.Background(), catalog.URL, dir, "demo", "points.geojson",
		[]byte(`{"type":"FeatureCollection","features":[]}`), func(s string) { steps = append(steps, s) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Layer != "points" || res.Store != "points" {
		t.Fatalf("result = %+v", res)
	}
	if _, err := os.Stat(res.StoredPath); err != nil {
		t.Fatalf("file not stored: %v", err)
	}
	joined := strings.Join(paths, " ")
	if !strings.Contains(joined, "/rest/workspaces") ||
		!strings.Contains(joined, "/datastores") ||
		!strings.Contains(joined, "/featuretypes") {
		t.Fatalf("catalog calls = %v", paths)
	}
	if len(steps) == 0 {
		t.Fatal("no progress steps")
	}
}
```

- [x] **Step 3: Implement ingest.go** (DetectType by ext; Import writes `{dataDir}/{ws}/{filename}`, then 3 catalog POSTs via `http.Client`; treat 201/200/409 as ok; call progress before each step). **Step 4: Run** → PASS. **Commit** `git commit -m "feat(convert): ingest pipeline with catalog auto-publish"`

---

### Task 6: Convert HTTP API (multipart upload + SSE)

**Files:**
- Create: `services/convert/internal/api/api.go`, `services/convert/internal/api/api_test.go`
- Modify: `services/convert/main.go` (mount)

**Interfaces:**
- Produces: `func Mount(mux *http.ServeMux, catalogURL, dataDir string)` mounting:
  - `POST /api/v1/convert/import?workspace=demo` multipart form field `file` → runs `ingest.Import`, streams progress as SSE (`text/event-stream`, `data: {"step":"..."}` lines, final `data: {"done":true,"layer":"..."}`).
  - `POST /api/v1/convert/cog?...` (GeoTIFF→COG): v1 returns 501 with a note (needs GDAL) OR a simple copy passthrough marking it a stub. Implement as a 202-accepted stub that records the request; full COG in S12 raster pack. Keep it honest: return `{"status":"pending","note":"COG conversion lands in the raster driver pack"}` with 200.

- [x] **Step 1: Failing test** — `api_test.go`:

```go
package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImportSSE(t *testing.T) {
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer catalog.Close()
	mux := http.NewServeMux()
	Mount(mux, catalog.URL, t.TempDir())

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "points.geojson")
	fw.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/convert/import?workspace=demo", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("code = %d %s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !strings.Contains(out, "data:") || !strings.Contains(out, `"done":true`) {
		t.Fatalf("sse = %s", out)
	}
}

func TestCogStub(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, "http://catalog:8080", t.TempDir())
	req := httptest.NewRequest("POST", "/api/v1/convert/cog", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "pending") {
		t.Fatalf("cog = %d %s", rec.Code, rec.Body.String())
	}
}
```

- [x] **Step 2: Run** → FAIL. **Step 3: Implement api.go** (parse multipart, `http.Flusher` for SSE, call `ingest.Import` with progress → SSE lines). Wire `api.Mount` in main.go with health + `GEOSON_CATALOG_URL`/`GEOSON_DATA_DIR`. Compose: `convert` service (`GEOSON_CATALOG_URL: http://catalog:8080`, `GEOSON_DATA_DIR: /var/lib/geoson/data`, volume `geosondata:/var/lib/geoson/data`), traefik route `/api/v1/convert` priority 15, add `geosondata` volume. Gateway/catalog also mount `geosondata` read paths later (wfs/wms read file stores) — add the volume mount to wfs + wms services too so published file layers are readable. CI docker-build: convert line. **Step 4: Run** → PASS. **Commit** `git commit -m "feat(convert): multipart import api with sse progress"`

---

### Task 7: E2E, docs, close out

**Files:**
- Create: `docs/services/wps.md`, `docs/services/convert.md`
- Modify: `docs/architecture.md`, `task.md`

- [x] **Step 1: Compose e2e**

```bash
cd deploy/compose && docker compose up -d --build wps convert gateway catalog
# WPS buffer sync through gateway
curl -s "http://localhost/geoserver/wps?service=WPS&version=1.0.0&request=Execute&identifier=geoson:buffer&DataInputs=geom%3DPOINT(0%200)%3Bdistance%3D1" | grep -o POLYGON | head -1
# WPS capabilities
curl -s "http://localhost/geoserver/wps?service=WPS&version=1.0.0&request=GetCapabilities" | grep -o 'geoson:buffer' | head -1
# convert import via traefik
printf '{"type":"FeatureCollection","features":[]}' > /tmp/pts.geojson
curl -s -F "file=@/tmp/pts.geojson" "http://localhost/api/v1/convert/import?workspace=demo3" | tail -3
# verify layer published
curl -s http://localhost/geoserver/rest/workspaces/demo3/datastores.json
```

Expected: WPS returns POLYGON buffer + lists geoson:buffer; convert SSE ends with done + the store appears in catalog.

- [x] **Step 2: docs/services/wps.md + convert.md** — endpoints, process list, async flow, ingest flow, COG-stub note.
- [x] **Step 3: architecture.md wps + convert rows → done; task.md Sprint 8 → [x]; plan boxes → [x]**
- [x] **Step 4: Final verify + commit**

```bash
go vet github.com/geoson/geoson/... && \
GEOSON_TEST_DATABASE_URL=postgres://geoson:geoson-dev-password@127.0.0.1:5433/geoson \
  go test github.com/geoson/geoson/...
git add -A && git commit -m "feat(wps,convert): e2e, docs; complete sprint 8"
```
