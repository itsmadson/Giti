package style

import "strings"

import "testing"

func TestValidateSLD(t *testing.T) {
	good := GenerateDefault("POINT", "#2FA7A1")
	if r := Validate("sld", good); !r.OK {
		t.Fatalf("generated SLD should validate: %+v", r.Errors)
	}
	if r := Validate("sld", "<bad"); r.OK {
		t.Fatal("malformed XML should fail")
	}
	if r := Validate("sld", "<root/>"); r.OK {
		t.Fatal("wrong root should fail")
	}
}

func TestGenerateDefaultGeom(t *testing.T) {
	if !strings.Contains(GenerateDefault("POLYGON", ""), "PolygonSymbolizer") {
		t.Fatal("polygon missing PolygonSymbolizer")
	}
	if !strings.Contains(GenerateDefault("LINESTRING", ""), "LineSymbolizer") {
		t.Fatal("line missing LineSymbolizer")
	}
	if !strings.Contains(GenerateDefault("POINT", ""), "PointSymbolizer") {
		t.Fatal("point missing PointSymbolizer")
	}
}

func TestValidateMBStyle(t *testing.T) {
	if r := Validate("mbstyle", `{"version":8}`); !r.OK {
		t.Fatal("valid json mbstyle should pass")
	}
	if r := Validate("mbstyle", `{bad`); r.OK {
		t.Fatal("invalid json should fail")
	}
}
