package connect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/giti/giti/services/catalog/internal/model"
)

func TestCSVConnector(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cities.csv")
	os.WriteFile(p, []byte("name,lat,lon\nTehran,35.7,51.4\n"), 0644)
	st := model.Store{Type: "CSV", Connection: map[string]string{"url": p, "latField": "lat", "lonField": "lon"}}
	c, _ := ForType("CSV")
	if err := c.Validate(context.Background(), st); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := c.Introspect(context.Background(), st)
	if err != nil || len(res) != 1 || res[0].GeometryType != "Point" {
		t.Fatalf("introspect: %v %+v", err, res)
	}
}

func TestKMLConnector(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "places.kml")
	os.WriteFile(p, []byte(`<?xml version="1.0"?><kml xmlns="http://www.opengis.net/kml/2.2"><Document/></kml>`), 0644)
	st := model.Store{Type: "KML", Connection: map[string]string{"url": p}}
	c, _ := ForType("KML")
	if err := c.Validate(context.Background(), st); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestWizardHasNewTypes(t *testing.T) {
	got := map[string]bool{}
	for _, m := range StoreTypes() {
		got[m.Type] = true
	}
	for _, want := range []string{"CSV", "KML", "SQLServer", "WMS", "WMTS"} {
		if !got[want] {
			t.Errorf("store type %q not advertised", want)
		}
	}
}
