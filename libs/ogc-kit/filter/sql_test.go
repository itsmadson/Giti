package filter

import (
	"reflect"
	"testing"
)

func mustSQL(t *testing.T, cql string) (string, []any) {
	t.Helper()
	e, err := ParseCQL(cql)
	if err != nil {
		t.Fatal(err)
	}
	sql, args, err := ToSQL(e, 1)
	if err != nil {
		t.Fatal(err)
	}
	return sql, args
}

func TestSQLBasics(t *testing.T) {
	cases := []struct {
		cql  string
		sql  string
		args []any
	}{
		{"name = 'road'", `"name" = $1`, []any{"road"}},
		{"lanes >= 4", `"lanes" >= $1`, []any{float64(4)}},
		{"a = 1 AND b <> 'x'", `("a" = $1 AND "b" <> $2)`, []any{float64(1), "x"}},
		{"a = 1 OR NOT b = 2", `("a" = $1 OR NOT ("b" = $2))`, []any{float64(1), float64(2)}},
		{"name ILIKE 'ro%'", `"name" ILIKE $1`, []any{"ro%"}},
		{"t IN ('a','b')", `"t" IN ($1, $2)`, []any{"a", "b"}},
		{"n BETWEEN 1 AND 3", `"n" BETWEEN $1 AND $2`, []any{float64(1), float64(3)}},
		{"n IS NOT NULL", `"n" IS NOT NULL`, nil},
		{"INCLUDE", `TRUE`, nil},
	}
	for _, c := range cases {
		sql, args := mustSQL(t, c.cql)
		if sql != c.sql || !reflect.DeepEqual(args, c.args) {
			t.Fatalf("%s:\n got  %q %v\n want %q %v", c.cql, sql, args, c.sql, c.args)
		}
	}
}

func TestSQLSpatial(t *testing.T) {
	sql, args := mustSQL(t, "BBOX(geom, -10, -20, 10, 20)")
	if sql != `"geom" && ST_MakeEnvelope($1, $2, $3, $4)` || len(args) != 4 {
		t.Fatalf("bbox: %q %v", sql, args)
	}
	sql, args = mustSQL(t, "INTERSECTS(geom, POINT(1 2))")
	if sql != `ST_Intersects("geom", ST_GeomFromText($1))` || args[0] != "POINT(1 2)" {
		t.Fatalf("intersects: %q %v", sql, args)
	}
	sql, args = mustSQL(t, "DWITHIN(geom, POINT(0 0), 500, meters)")
	if sql != `ST_DWithin("geom", ST_GeomFromText($1), $2)` || args[1] != float64(500) {
		t.Fatalf("dwithin: %q %v", sql, args)
	}
}

func TestSQLInjectionBlocked(t *testing.T) {
	if _, err := ParseCQL(`"na""me; DROP TABLE x" = 1`); err == nil {
		t.Fatal("quoted ident with injection parsed")
	}
	e, err := ParseCQL("name = 'x''; DROP TABLE y; --'")
	if err != nil {
		t.Fatal(err)
	}
	sql, args, err := ToSQL(e, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sql != `"name" = $1` || args[0] != "x'; DROP TABLE y; --" {
		t.Fatalf("injection not parameterized: %q %v", sql, args)
	}
}

func TestSQLStartArgOffset(t *testing.T) {
	e, _ := ParseCQL("a = 1")
	sql, _, _ := ToSQL(e, 5)
	if sql != `"a" = $5` {
		t.Fatalf("offset: %q", sql)
	}
}
