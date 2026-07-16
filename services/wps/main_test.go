package main

import (
	"net/http/httptest"
	"testing"
)

func TestWPSServesHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler(deps{}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q", rec.Code, rec.Body.String())
	}
}
