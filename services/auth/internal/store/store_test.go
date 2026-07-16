package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("a%d", time.Now().UnixNano())
	if _, err := pool.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
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

func TestUserGroupRoleLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)

	if err := s.CreateUser(ctx, User{Name: "alice", Enabled: true, PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateGroup(ctx, "editors"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, "alice", "editors"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRole(ctx, "ROLE_EDITOR"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRole(ctx, "ROLE_DIRECT"); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignRoleGroup(ctx, "ROLE_EDITOR", "editors"); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignRoleUser(ctx, "ROLE_DIRECT", "alice"); err != nil {
		t.Fatal(err)
	}
	roles, err := s.RolesOf(ctx, "alice")
	if err != nil || len(roles) != 2 {
		t.Fatalf("roles = %v, %v (want 2)", roles, err)
	}
}

func TestSeedAdminIdempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if err := s.SeedAdmin(ctx, "hash1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedAdmin(ctx, "hash2"); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	u, err := s.GetUser(ctx, "admin")
	if err != nil || u.PasswordHash != "hash1" {
		t.Fatalf("admin = %+v, %v (seed must not overwrite)", u, err)
	}
	roles, _ := s.RolesOf(ctx, "admin")
	if len(roles) != 1 || roles[0] != "ADMIN" {
		t.Fatalf("admin roles = %v", roles)
	}
}

func TestRuleCRUDOrdering(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if _, err := s.CreateRule(ctx, Rule{Priority: 20, Access: "DENY"}); err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateRule(ctx, Rule{Priority: 10, Access: "ALLOW", Workspace: "topp"})
	if err != nil {
		t.Fatal(err)
	}
	rules, err := s.ListRules(ctx)
	if err != nil || len(rules) != 2 || rules[0].Priority != 10 {
		t.Fatalf("rules = %+v, %v", rules, err)
	}
	if err := s.DeleteRule(ctx, id); err != nil {
		t.Fatal(err)
	}
}
