package connect

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "tests", "testdata", name)
}

func fileStore(typ, path string) model.Store {
	return model.Store{Type: typ, Connection: map[string]string{"url": path}}
}

func TestFileConnectorsValidate(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		typ, file string
		wantName  string
	}{
		{"Shapefile", "roads.shp", "roads"},
		{"GeoPackage", "data.gpkg", "data"},
		{"GeoJSON", "points.geojson", "points"},
		{"GeoTIFF", "dem.tif", "dem"},
	}
	for _, tc := range cases {
		c, err := ForType(tc.typ)
		if err != nil {
			t.Fatalf("%s: %v", tc.typ, err)
		}
		st := fileStore(tc.typ, testdata(t, tc.file))
		if err := c.Validate(ctx, st); err != nil {
			t.Fatalf("%s validate: %v", tc.typ, err)
		}
		if err := c.Validate(ctx, fileStore(tc.typ, testdata(t, "missing.bin"))); err == nil {
			t.Fatalf("%s: want error for missing file", tc.typ)
		}
		// wrong magic: geojson file fed to binary formats must fail
		if tc.typ != "GeoJSON" {
			if err := c.Validate(ctx, fileStore(tc.typ, testdata(t, "points.geojson"))); err == nil {
				t.Fatalf("%s: want magic-byte error", tc.typ)
			}
		}
		infos, err := c.Introspect(ctx, st)
		if err != nil || len(infos) != 1 || infos[0].Name != tc.wantName {
			t.Fatalf("%s introspect = %+v, %v", tc.typ, infos, err)
		}
	}
}

func TestGeoParquetIntrospect(t *testing.T) {
	ctx := context.Background()
	c, err := ForType("GeoParquet")
	if err != nil {
		t.Fatal(err)
	}
	st := fileStore("GeoParquet", testdata(t, "places.parquet"))
	if err := c.Validate(ctx, st); err != nil {
		t.Fatalf("validate: %v", err)
	}
	infos, err := c.Introspect(ctx, st)
	if err != nil || len(infos) != 1 || infos[0].Name != "places" {
		t.Fatalf("introspect = %+v, %v", infos, err)
	}
}
