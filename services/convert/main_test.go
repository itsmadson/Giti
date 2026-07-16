package main

import (
	"net/http/httptest"
	"testing"
)

func TestConvertServesHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler("http://catalog:8080", "/tmp").ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q", rec.Code, rec.Body.String())
	}
}
