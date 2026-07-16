package meta

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedFixture creates the wfstest workspace/store/resource/layer chain and the
// wfs_roads data table. Exported for reuse by the wfs package tests.
func SeedFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	must := func(sql string) {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis`)
	pool.Exec(ctx, `DELETE FROM layers WHERE workspace='wfstest'`)
	pool.Exec(ctx, `DELETE FROM resources WHERE workspace='wfstest'`)
	pool.Exec(ctx, `DELETE FROM stores WHERE workspace='wfstest'`)
	pool.Exec(ctx, `DELETE FROM workspaces WHERE name='wfstest'`)
	must(`INSERT INTO workspaces(name) VALUES('wfstest')`)
	must(`INSERT INTO stores(workspace,name,kind,type,enabled,connection)
		VALUES('wfstest','local','datastore','PostGIS',true,'{"host":"self"}'::jsonb)`)
	must(`DROP TABLE IF EXISTS wfs_roads`)
	must(`CREATE TABLE wfs_roads (
		id serial PRIMARY KEY, name text, lanes int, geom geometry(LineString,4326))`)
	must(`INSERT INTO wfs_roads(name,lanes,geom) VALUES
		('main st', 4, ST_GeomFromText('LINESTRING(0 0, 1 1)', 4326)),
		('back rd', 2, ST_GeomFromText('LINESTRING(2 2, 3 3)', 4326)),
		('ring way', 6, ST_GeomFromText('LINESTRING(10 10, 11 11)', 4326))`)
	must(`INSERT INTO resources(workspace,store,name,kind,native_name,srs,enabled)
		VALUES('wfstest','local','wfs_roads','featuretype','wfs_roads','EPSG:4326',true)`)
	must(`INSERT INTO layers(workspace,name,type,resource_name,default_style,enabled)
		VALUES('wfstest','wfs_roads','VECTOR','wfs_roads','generic',true)`)
}

func testCatalogDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GEOSON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GEOSON_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	SeedFixture(t, pool)
	return pool
}

func TestResolve(t *testing.T) {
	db := testCatalogDB(t)
	m := New(db)
	l, err := m.Resolve(context.Background(), "wfstest", "wfs_roads")
	if err != nil {
		t.Fatal(err)
	}
	if l.Table != "wfs_roads" || l.GeomCol != "geom" || l.SRS != "EPSG:4326" {
		t.Fatalf("layer = %+v", l)
	}
	var names []string
	for _, c := range l.Columns {
		names = append(names, c.Name)
	}
	if len(names) != 3 { // id, name, lanes (geom excluded)
		t.Fatalf("columns = %v", names)
	}
	if _, err := m.Resolve(context.Background(), "wfstest", "nope"); err == nil {
		t.Fatal("want ErrNotFound")
	}
}

func TestListFeatureTypes(t *testing.T) {
	db := testCatalogDB(t)
	m := New(db)
	fts, err := m.ListFeatureTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ft := range fts {
		if ft.Workspace == "wfstest" && ft.Name == "wfs_roads" {
			found = true
		}
	}
	if !found {
		t.Fatalf("wfs_roads not listed: %+v", fts)
	}
}
