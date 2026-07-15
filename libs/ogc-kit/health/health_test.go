package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestHealthzAlwaysOK(t *testing.T) {
	mux := NewMux(map[string]Check{
		"broken": func(ctx context.Context) error { return errors.New("down") },
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q, want 200 ok", rec.Code, rec.Body.String())
	}
}

func TestReadyzReadyWhenChecksPass(t *testing.T) {
	mux := NewMux(map[string]Check{
		"redis": func(ctx context.Context) error { return nil },
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 200 {
		t.Fatalf("readyz code = %d, want 200", rec.Code)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ready" || body.Checks["redis"] != "ok" {
		t.Fatalf("body = %+v, want ready/redis ok", body)
	}
}

func TestReadyzUnreadyWhenCheckFails(t *testing.T) {
	mux := NewMux(map[string]Check{
		"postgres": func(ctx context.Context) error { return errors.New("conn refused") },
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 503 {
		t.Fatalf("readyz code = %d, want 503", rec.Code)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "unready" || body.Checks["postgres"] != "conn refused" {
		t.Fatalf("body = %+v, want unready/conn refused", body)
	}
}
