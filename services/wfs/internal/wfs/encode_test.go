package wfs

import (
	"strings"
	"testing"
)

func clip(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func TestGetFeatureGML32(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&version=2.0.0&request=GetFeature&typeNames=wfstest:wfs_roads", nil)
	body := rec.Body.String()
	for _, want := range []string{
		"wfs:FeatureCollection", `numberMatched="3"`, "gml:LineString",
		"<giti:name>main st</giti:name>", "http://www.opengis.net/wfs/2.0",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, clip(body, 2000))
		}
	}
}

func TestGetFeatureGML2(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&version=1.0.0&request=GetFeature&typeName=wfstest:wfs_roads", nil)
	body := rec.Body.String()
	if !strings.Contains(body, "gml:coordinates") || !strings.Contains(body, "wfs:FeatureCollection") {
		t.Fatalf("gml2 = %s", clip(body, 1500))
	}
}

func TestGetFeatureCSV(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&request=GetFeature&typeNames=wfstest:wfs_roads&outputFormat=csv", nil)
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "FID,") {
		t.Fatalf("csv = %v", lines)
	}
	if !strings.Contains(rec.Body.String(), "LINESTRING") {
		t.Fatal("csv missing WKT geometry")
	}
}

func TestDescribeFeatureType(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&version=2.0.0&request=DescribeFeatureType&typeNames=wfstest:wfs_roads", nil)
	body := rec.Body.String()
	for _, want := range []string{"xsd:schema", `name="name"`, `name="lanes"`, "gml:GeometryPropertyType"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}
