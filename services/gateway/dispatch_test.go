package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testBackends(t *testing.T, wmsURL string) backends {
	t.Helper()
	b := backends{byService: map[string]*url.URL{}}
	if wmsURL != "" {
		u, err := url.Parse(wmsURL)
		if err != nil {
			t.Fatal(err)
		}
		b.byService["WMS"] = u
	}
	return b
}

func TestDispatchProxiesToWMS(t *testing.T) {
	var got *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		w.Write([]byte("MAP"))
	}))
	defer backend.Close()

	h := newDispatcher(testBackends(t, backend.URL))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET",
		"/geoserver/topp/wms?service=wms&version=1.1.1&request=GetMap&layers=roads", nil))
	if rec.Code != 200 || rec.Body.String() != "MAP" {
		t.Fatalf("proxy = %d %s", rec.Code, rec.Body.String())
	}
	if got.Header.Get("X-Geoson-Workspace") != "topp" {
		t.Fatalf("ws header = %q", got.Header.Get("X-Geoson-Workspace"))
	}
	if got.Header.Get("X-Geoson-Version") != "1.1.1" {
		t.Fatalf("version header = %q", got.Header.Get("X-Geoson-Version"))
	}
	if got.URL.Query().Get("layers") != "roads" {
		t.Fatalf("query lost: %s", got.URL.RawQuery)
	}
}

func TestDispatchLayerVirtualService(t *testing.T) {
	var got *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
	}))
	defer backend.Close()
	h := newDispatcher(testBackends(t, backend.URL))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET",
		"/geoserver/topp/roads/wms?request=GetMap", nil))
	if got.Header.Get("X-Geoson-Layer") != "roads" {
		t.Fatalf("layer header = %q", got.Header.Get("X-Geoson-Layer"))
	}
}

func TestDispatchEndpointImpliesService(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer backend.Close()
	h := newDispatcher(testBackends(t, backend.URL))
	rec := httptest.NewRecorder()
	// no SERVICE param — /wms endpoint implies WMS
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/geoserver/wms?request=GetCapabilities", nil))
	if rec.Code != 200 {
		t.Fatalf("implied service = %d %s", rec.Code, rec.Body.String())
	}
}

func TestDispatchMissingRequestParam(t *testing.T) {
	h := newDispatcher(testBackends(t, "http://wms:8080"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/geoserver/wms?service=WMS&version=1.1.1", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "ServiceExceptionReport") || !strings.Contains(body, "request") {
		t.Fatalf("body = %s", body)
	}
}

func TestDispatchOWSRequiresService(t *testing.T) {
	h := newDispatcher(testBackends(t, "http://wms:8080"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/geoserver/ows?request=GetCapabilities", nil))
	if !strings.Contains(rec.Body.String(), "service") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestDispatchUnavailableBackend(t *testing.T) {
	h := newDispatcher(testBackends(t, "")) // no WMS backend
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET",
		"/geoserver/wms?service=WMS&request=GetCapabilities", nil))
	if !strings.Contains(rec.Body.String(), "not available") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestDispatchPostXML(t *testing.T) {
	var got *http.Request
	var gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
	}))
	defer backend.Close()
	b := testBackends(t, "")
	u, _ := url.Parse(backend.URL)
	b.byService["WFS"] = u
	h := newDispatcher(b)
	xmlBody := `<GetFeature service="WFS" version="2.0.0" xmlns="http://www.opengis.net/wfs/2.0"/>`
	req := httptest.NewRequest("POST", "/geoserver/wfs", strings.NewReader(xmlBody))
	req.Header.Set("Content-Type", "application/xml")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.Header.Get("X-Geoson-Version") != "2.0.0" {
		t.Fatalf("version = %q", got.Header.Get("X-Geoson-Version"))
	}
	if !strings.Contains(gotBody, "GetFeature") {
		t.Fatalf("body not forwarded: %q", gotBody)
	}
}
