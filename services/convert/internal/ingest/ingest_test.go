package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDetectType(t *testing.T) {
	cases := map[string]string{
		"roads.shp": "Shapefile", "data.gpkg": "GeoPackage",
		"pts.geojson": "GeoJSON", "table.csv": "CSV", "dem.tif": "GeoTIFF",
	}
	for f, want := range cases {
		got, err := DetectType(f)
		if err != nil || got != want {
			t.Fatalf("%s -> %s, %v (want %s)", f, got, err, want)
		}
	}
	if _, err := DetectType("x.docx"); err == nil {
		t.Fatal("want error for unsupported")
	}
}

func TestImportCallsCatalog(t *testing.T) {
	var paths []string
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer catalog.Close()
	dir := t.TempDir()
	var steps []string
	res, err := Import(context.Background(), catalog.URL, dir, "demo", "points.geojson",
		[]byte(`{"type":"FeatureCollection","features":[]}`), func(s string) { steps = append(steps, s) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Layer != "points" || res.Store != "points" {
		t.Fatalf("result = %+v", res)
	}
	if _, err := os.Stat(res.StoredPath); err != nil {
		t.Fatalf("file not stored: %v", err)
	}
	joined := strings.Join(paths, " ")
	if !strings.Contains(joined, "/rest/workspaces") ||
		!strings.Contains(joined, "/datastores") ||
		!strings.Contains(joined, "/featuretypes") {
		t.Fatalf("catalog calls = %v", paths)
	}
	if len(steps) == 0 {
		t.Fatal("no progress steps")
	}
}
