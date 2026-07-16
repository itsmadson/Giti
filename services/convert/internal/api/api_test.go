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
