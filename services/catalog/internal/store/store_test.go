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
