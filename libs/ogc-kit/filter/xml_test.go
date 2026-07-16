package filter

import (
	"reflect"
	"testing"
)

func TestFilterXML10Comparison(t *testing.T) {
	x := `<Filter xmlns="http://www.opengis.net/ogc">
	  <And>
	    <PropertyIsEqualTo><PropertyName>name</PropertyName><Literal>road</Literal></PropertyIsEqualTo>
	    <PropertyIsGreaterThanOrEqualTo><PropertyName>lanes</PropertyName><Literal>4</Literal></PropertyIsGreaterThanOrEqualTo>
	  </And></Filter>`
	e, err := ParseFilterXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	want := Logic{Op: "AND", Exprs: []Expr{
		Compare{Op: "=", Left: Property{Name: "name"}, Right: Literal{Value: "road"}},
		Compare{Op: ">=", Left: Property{Name: "lanes"}, Right: Literal{Value: "4"}},
	}}
	if !reflect.DeepEqual(e, want) {
		t.Fatalf("got %#v", e)
	}
}

func TestFilterXML20ValueReference(t *testing.T) {
	x := `<fes:Filter xmlns:fes="http://www.opengis.net/fes/2.0">
	  <fes:PropertyIsEqualTo><fes:ValueReference>name</fes:ValueReference><fes:Literal>x</fes:Literal></fes:PropertyIsEqualTo>
	</fes:Filter>`
	e, err := ParseFilterXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e, Compare{Op: "=", Left: Property{Name: "name"}, Right: Literal{Value: "x"}}) {
		t.Fatalf("got %#v", e)
	}
}

func TestFilterXMLBBox(t *testing.T) {
	x := `<Filter xmlns="http://www.opengis.net/ogc" xmlns:gml="http://www.opengis.net/gml">
	  <BBOX><PropertyName>geom</PropertyName>
	    <gml:Envelope srsName="EPSG:4326"><gml:lowerCorner>-10 -20</gml:lowerCorner><gml:upperCorner>10 20</gml:upperCorner></gml:Envelope>
	  </BBOX></Filter>`
	e, err := ParseFilterXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	b, ok := e.(BBox)
	if !ok || b.Prop != "geom" || b.MinX != -10 || b.MaxY != 20 || b.SRS != "EPSG:4326" {
		t.Fatalf("got %#v", e)
	}
}

func TestFilterXMLIntersectsPoint(t *testing.T) {
	x := `<Filter xmlns="http://www.opengis.net/ogc" xmlns:gml="http://www.opengis.net/gml">
	  <Intersects><PropertyName>geom</PropertyName>
	    <gml:Point><gml:coordinates>1,2</gml:coordinates></gml:Point>
	  </Intersects></Filter>`
	e, err := ParseFilterXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := e.(Spatial)
	if !ok || s.Op != "INTERSECTS" || s.WKT != "POINT(1 2)" {
		t.Fatalf("got %#v", e)
	}
}

func TestFilterXMLErrors(t *testing.T) {
	if _, err := ParseFilterXML([]byte("<not xml")); err == nil {
		t.Fatal("want error on malformed xml")
	}
}
