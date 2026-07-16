package api

import (
	"net/http"
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
