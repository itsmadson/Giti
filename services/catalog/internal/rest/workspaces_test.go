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
	pool.Exec(context.Background(), `TRUNCATE workspaces CASCADE`)
	pool.Exec(context.Background(), `DELETE FROM styles WHERE workspace <> '' OR name NOT IN ('generic','point','line','polygon','raster')`)
	pool.Exec(context.Background(), `TRUNCATE layer_groups`)
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

func TestGeoserverPrefixAlias(t *testing.T) {
	mux, _ := testMux(t)
	rec := do(t, mux, "GET", "/geoserver/rest/workspaces.json", "", "")
	if rec.Code != 200 {
		t.Fatalf("geoserver-prefixed GET = %d", rec.Code)
	}
}
