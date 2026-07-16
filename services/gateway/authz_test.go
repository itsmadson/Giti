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
