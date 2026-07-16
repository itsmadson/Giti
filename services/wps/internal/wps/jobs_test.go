package wps

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GEOSON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GEOSON_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS postgis")
	t.Cleanup(pool.Close)
	return pool
}

func TestExecNowAndStatus(t *testing.T) {
	db := testDB(t)
	dir := t.TempDir()
	j := NewJobs(dir, nil, db)
	out, err := j.execNow(context.Background(), "geoson:buffer",
		map[string]string{"geom": "POINT(0 0)", "distance": "1"})
	if err != nil || !strings.HasPrefix(out, "POLYGON") {
		t.Fatalf("execNow = %q, %v", out, err)
	}
}

func TestEnqueueWritesAcceptedStatus(t *testing.T) {
	db := testDB(t)
	dir := t.TempDir()
	j := NewJobs(dir, nil, db)
	id, err := j.Enqueue(context.Background(), "geoson:centroid",
		map[string]string{"geom": "POLYGON((0 0,0 2,2 2,2 0,0 0))"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := j.Status(id)
	if err != nil || st.Status != "accepted" || st.Process != "geoson:centroid" {
		t.Fatalf("status = %+v, %v", st, err)
	}
}
