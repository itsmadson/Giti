package process

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GITI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GITI_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS postgis")
	t.Cleanup(pool.Close)
	return pool
}

func TestBufferAndArea(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	p, _ := Get("giti:buffer")
	out, err := p.Run(ctx, db, map[string]string{"geom": "POINT(0 0)", "distance": "1"})
	if err != nil || !strings.HasPrefix(out, "POLYGON") {
		t.Fatalf("buffer = %q, %v", out, err)
	}
	pa, _ := Get("giti:area")
	area, err := pa.Run(ctx, db, map[string]string{"geom": "POLYGON((0 0,0 2,2 2,2 0,0 0))"})
	if err != nil || !strings.HasPrefix(area, "4") {
		t.Fatalf("area = %q, %v", area, err)
	}
}

func TestCentroidReprojectIntersectionUnionSimplify(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	cases := []struct {
		id     string
		inputs map[string]string
		prefix string
	}{
		{"giti:centroid", map[string]string{"geom": "POLYGON((0 0,0 2,2 2,2 0,0 0))"}, "POINT"},
		{"giti:reproject", map[string]string{"geom": "POINT(0 0)", "sourceSRID": "4326", "targetSRID": "3857"}, "POINT"},
		{"giti:intersection", map[string]string{"a": "POLYGON((0 0,0 2,2 2,2 0,0 0))", "b": "POLYGON((1 1,1 3,3 3,3 1,1 1))"}, "POLYGON"},
		{"giti:union", map[string]string{"a": "POINT(0 0)", "b": "POINT(1 1)"}, "MULTIPOINT"},
		{"giti:simplify", map[string]string{"geom": "LINESTRING(0 0,1 0.1,2 0)", "tolerance": "0.5"}, "LINESTRING"},
	}
	for _, c := range cases {
		p, _ := Get(c.id)
		out, err := p.Run(ctx, db, c.inputs)
		if err != nil || !strings.HasPrefix(out, c.prefix) {
			t.Fatalf("%s = %q, %v (want %s)", c.id, out, err, c.prefix)
		}
	}
	pl, _ := Get("giti:length")
	l, err := pl.Run(ctx, db, map[string]string{"geom": "LINESTRING(0 0,3 0)"})
	if err != nil || !strings.HasPrefix(l, "3") {
		t.Fatalf("length = %q, %v", l, err)
	}
}
