package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRateLimitReturns429(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	h := rateLimitMiddleware(1, 1, inner)
	req := httptest.NewRequest("GET", "/giti/wms?service=WMS", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec1.Code != 200 || rec2.Code != 429 {
		t.Fatalf("codes = %d, %d; want 200, 429", rec1.Code, rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "Too many requests") {
		t.Fatalf("body = %s", rec2.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	h := newHandlerWith(backends{byService: nil})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("metrics = %d", rec.Code)
	}
}
