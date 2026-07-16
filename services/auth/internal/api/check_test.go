package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geoson/geoson/services/auth/internal/password"
	"github.com/geoson/geoson/services/auth/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testMux(t *testing.T, defaultAllow bool) (*http.ServeMux, *store.Store) {
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
	req.Header.Set("X-Geoson-Service", "WMS")
	req.Header.Set("X-Geoson-Workspace", "secret")
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"allow":false`) {
		t.Fatalf("check = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", basic("bob", "pw"))
	req.Header.Set("X-Geoson-Service", "WMS")
	req.Header.Set("X-Geoson-Workspace", "open")
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
	req.Header.Set("X-Geoson-Service", "WMS")
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"allow":false`) {
		t.Fatalf("anonymous default-deny = %s", rec.Body.String())
	}
}
