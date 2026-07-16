package filter

import (
	"reflect"
	"testing"
)

func TestParseComparisons(t *testing.T) {
	e, err := ParseCQL("name = 'road' AND lanes >= 4")
	if err != nil {
		t.Fatal(err)
	}
	want := Logic{Op: "AND", Exprs: []Expr{
		Compare{Op: "=", Left: Property{Name: "name"}, Right: Literal{Value: "road"}},
		Compare{Op: ">=", Left: Property{Name: "lanes"}, Right: Literal{Value: float64(4)}},
	}}
	if !reflect.DeepEqual(e, want) {
		t.Fatalf("got %#v", e)
	}
}

func TestParsePrecedenceOrAnd(t *testing.T) {
	e, err := ParseCQL("a = 1 OR b = 2 AND c = 3")
	if err != nil {
		t.Fatal(err)
	}
	or, ok := e.(Logic)
	if !ok || or.Op != "OR" || len(or.Exprs) != 2 {
		t.Fatalf("got %#v", e)
	}
	if inner, ok := or.Exprs[1].(Logic); !ok || inner.Op != "AND" {
		t.Fatalf("AND must bind tighter: %#v", or.Exprs[1])
	}
}

func TestParseLikeInBetweenNull(t *testing.T) {
	cases := map[string]Expr{
		"name LIKE 'ro%'":       Like{Prop: Property{Name: "name"}, Pattern: "ro%"},
		"name ILIKE 'RO%'":      Like{Prop: Property{Name: "name"}, Pattern: "RO%", CaseInsensitive: true},
		"name NOT LIKE 'x%'":    Like{Prop: Property{Name: "name"}, Pattern: "x%", Negate: true},
		"type IN ('a', 'b')":    In{Prop: Property{Name: "type"}, Values: []Expr{Literal{Value: "a"}, Literal{Value: "b"}}},
		"lanes BETWEEN 2 AND 4": Between{Prop: Property{Name: "lanes"}, Lo: Literal{Value: float64(2)}, Hi: Literal{Value: float64(4)}},
		"name IS NULL":          IsNull{Prop: Property{Name: "name"}},
		"name IS NOT NULL":      IsNull{Prop: Property{Name: "name"}, Negate: true},
		"NOT (a = 1)":           Not{Expr: Compare{Op: "=", Left: Property{Name: "a"}, Right: Literal{Value: float64(1)}}},
		"INCLUDE":               IncludeAll{},
		"active = true":         Compare{Op: "=", Left: Property{Name: "active"}, Right: Literal{Value: true}},
		"note = 'it''s'":        Compare{Op: "=", Left: Property{Name: "note"}, Right: Literal{Value: "it's"}},
	}
	for cql, want := range cases {
		e, err := ParseCQL(cql)
		if err != nil {
			t.Fatalf("%s: %v", cql, err)
		}
		if !reflect.DeepEqual(e, want) {
			t.Fatalf("%s: got %#v want %#v", cql, e, want)
		}
	}
}

func TestParseSpatial(t *testing.T) {
	e, err := ParseCQL("BBOX(geom, -10, -20, 10, 20)")
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := e.(BBox); !ok || b.Prop != "geom" || b.MinX != -10 || b.MaxY != 20 {
		t.Fatalf("got %#v", e)
	}
	e, err = ParseCQL("INTERSECTS(geom, POINT(51.4 35.7))")
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := e.(Spatial); !ok || s.Op != "INTERSECTS" || s.WKT != "POINT(51.4 35.7)" {
		t.Fatalf("got %#v", e)
	}
	e, err = ParseCQL("DWITHIN(geom, POINT(0 0), 1000, meters)")
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := e.(DWithin); !ok || d.Distance != 1000 || d.Units != "meters" {
		t.Fatalf("got %#v", e)
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{"", "name =", "AND", "BBOX(geom,1,2,3)", "name LIKE 4", "a = 'unterminated"} {
		if _, err := ParseCQL(bad); err == nil {
			t.Fatalf("%q: want error", bad)
		}
	}
}
