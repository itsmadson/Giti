package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/geoson/geoson/services/catalog/internal/model"
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
	t.Cleanup(pool.Close)
	// isolate: fresh schema per test run
	schema := fmt.Sprintf("t%d", time.Now().UnixNano())
	if _, err := pool.Exec(context.Background(),
		fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
		t.Fatal(err)
	}
	cfg := pool.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool2, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
		pool2.Close()
	})
	return pool2
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestWorkspaceCRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)

	if err := s.CreateWorkspace(ctx, model.Workspace{Name: "topp"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateWorkspace(ctx, model.Workspace{Name: "topp"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup create = %v, want ErrConflict", err)
	}
	got, err := s.GetWorkspace(ctx, "topp")
	if err != nil || got.Name != "topp" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if _, err := s.GetWorkspace(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing get = %v, want ErrNotFound", err)
	}
	list, err := s.ListWorkspaces(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}
	if err := s.UpdateWorkspace(ctx, "topp", model.Workspace{Name: "topp2", Isolated: true}); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetWorkspace(ctx, "topp2")
	if err != nil || !got.Isolated {
		t.Fatalf("after update = %+v, %v", got, err)
	}
	if err := s.DeleteWorkspace(ctx, "topp2", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetWorkspace(ctx, "topp2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete = %v, want ErrNotFound", err)
	}
}

func TestStoreCRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if err := s.CreateWorkspace(ctx, model.Workspace{Name: "topp"}); err != nil {
		t.Fatal(err)
	}
	ds := model.Store{
		Workspace: "topp", Name: "pg", Kind: "datastore", Type: "PostGIS",
		Enabled: true, Connection: map[string]string{"host": "postgres", "port": "5432"},
	}
	if err := s.CreateStore(ctx, ds); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetStore(ctx, "topp", "pg", "datastore")
	if err != nil || got.Connection["host"] != "postgres" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if _, err := s.GetStore(ctx, "topp", "pg", "coveragestore"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong-kind get = %v, want ErrNotFound", err)
	}
	list, err := s.ListStores(ctx, "topp", "datastore")
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}
	ds.Description = "updated"
	if err := s.UpdateStore(ctx, "topp", "pg", ds); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteStore(ctx, "topp", "pg", false); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureTypeLayerStyleLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	s.CreateWorkspace(ctx, model.Workspace{Name: "topp"})
	s.CreateStore(ctx, model.Store{Workspace: "topp", Name: "pg", Kind: "datastore", Type: "PostGIS", Enabled: true})

	ft := model.FeatureType{Workspace: "topp", Store: "pg", Name: "roads",
		NativeName: "roads", SRS: "EPSG:3857", Enabled: true}
	if err := s.CreateFeatureType(ctx, ft); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFeatureType(ctx, "topp", "pg", "roads")
	if err != nil || got.SRS != "EPSG:3857" {
		t.Fatalf("get ft = %+v, %v", got, err)
	}
	fts, err := s.ListFeatureTypes(ctx, "topp", "pg")
	if err != nil || len(fts) != 1 {
		t.Fatalf("list fts = %v, %v", fts, err)
	}

	st, err := s.GetStyle(ctx, "", "generic")
	if err != nil || st.Body == "" {
		t.Fatalf("seed style = %+v, %v", st, err)
	}

	if err := s.CreateLayer(ctx, model.Layer{Workspace: "topp", Name: "roads",
		Type: "VECTOR", ResourceName: "roads", DefaultStyle: "generic", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	l, err := s.GetLayer(ctx, "topp", "roads")
	if err != nil || l.DefaultStyle != "generic" {
		t.Fatalf("layer = %+v, %v", l, err)
	}

	if err := s.CreateLayerGroup(ctx, model.LayerGroup{Workspace: "topp",
		Name: "basemap", Mode: "SINGLE", Layers: []string{"roads"}}); err != nil {
		t.Fatal(err)
	}
	lg, err := s.GetLayerGroup(ctx, "topp", "basemap")
	if err != nil || len(lg.Layers) != 1 {
		t.Fatalf("layergroup = %+v, %v", lg, err)
	}

	if err := s.DeleteFeatureType(ctx, "topp", "pg", "roads"); err != nil {
		t.Fatal(err)
	}
}
