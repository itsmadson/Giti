package connect

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/giti/giti/services/catalog/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

// storeFromDSN converts GITI_TEST_DATABASE_URL into PostGIS connection params.
func storeFromDSN(t *testing.T) (model.Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("GITI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GITI_TEST_DATABASE_URL not set")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pw, _ := u.User.Password()
	st := model.Store{Type: "PostGIS", Connection: map[string]string{
		"host":     u.Hostname(),
		"port":     u.Port(),
		"database": strings.TrimPrefix(u.Path, "/"),
		"user":     u.User.Username(),
		"passwd":   pw,
	}}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return st, pool
}

func TestPostGISValidateAndIntrospect(t *testing.T) {
	st, pool := storeFromDSN(t)
	ctx := context.Background()
	pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis`)
	pool.Exec(ctx, `DROP TABLE IF EXISTS conn_test_roads`)
	if _, err := pool.Exec(ctx,
		`CREATE TABLE conn_test_roads (id serial PRIMARY KEY, geom geometry(LineString, 3857))`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DROP TABLE conn_test_roads`) })

	c, err := ForType("PostGIS")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(ctx, st); err != nil {
		t.Fatalf("validate: %v", err)
	}
	bad := st
	bad.Connection = map[string]string{"host": "127.0.0.1", "port": "1", "database": "x", "user": "x", "passwd": "x"}
	if err := c.Validate(ctx, bad); err == nil {
		t.Fatal("validate bad params: want error")
	}
	infos, err := c.Introspect(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	var found *ResourceInfo
	for i := range infos {
		if infos[i].Name == "conn_test_roads" {
			found = &infos[i]
		}
	}
	if found == nil || found.GeometryType != "LineString" || found.SRS != "EPSG:3857" {
		t.Fatalf("introspect = %+v", infos)
	}
}

func TestForTypeUnknown(t *testing.T) {
	if _, err := ForType("Oracle"); err == nil {
		t.Fatal("want error for unknown type")
	}
}
