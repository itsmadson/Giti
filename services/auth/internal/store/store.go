// Package store persists auth users/groups/roles and GeoFence rules.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type User struct {
	Name         string
	Enabled      bool
	PasswordHash string
}

type Rule struct {
	ID         int64
	Priority   int64
	Username   string
	Rolename   string
	Service    string
	Request    string
	Workspace  string
	Layer      string
	Access     string
	CQLRead    string
	CQLWrite   string
	Attributes []string
}

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) CreateUser(ctx context.Context, u User) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_users(name, enabled, password_hash) VALUES($1,$2,$3)`,
		u.Name, u.Enabled, u.PasswordHash)
	return mapErr(err)
}

func (s *Store) GetUser(ctx context.Context, name string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		`SELECT name, enabled, password_hash FROM auth_users WHERE name=$1`, name,
	).Scan(&u.Name, &u.Enabled, &u.PasswordHash)
	return u, mapErr(err)
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.Query(ctx, `SELECT name, enabled, password_hash FROM auth_users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Name, &u.Enabled, &u.PasswordHash); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, name string, u User) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE auth_users SET enabled=$2, password_hash=$3 WHERE name=$1`,
		name, u.Enabled, u.PasswordHash)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM auth_users WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateGroup(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO auth_groups(name) VALUES($1)`, name)
	return mapErr(err)
}

func (s *Store) ListGroups(ctx context.Context) ([]string, error) {
	return s.listNames(ctx, `SELECT name FROM auth_groups ORDER BY name`)
}

func (s *Store) DeleteGroup(ctx context.Context, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM auth_groups WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddUserToGroup(ctx context.Context, user, group string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_user_groups(username, groupname) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		user, group)
	return mapErr(err)
}

func (s *Store) GroupsOf(ctx context.Context, user string) ([]string, error) {
	return s.listNames(ctx,
		`SELECT groupname FROM auth_user_groups WHERE username=$1 ORDER BY groupname`, user)
}

func (s *Store) CreateRole(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO auth_roles(name) VALUES($1)`, name)
	return mapErr(err)
}

func (s *Store) ListRoles(ctx context.Context) ([]string, error) {
	return s.listNames(ctx, `SELECT name FROM auth_roles ORDER BY name`)
}

func (s *Store) DeleteRole(ctx context.Context, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM auth_roles WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AssignRoleUser(ctx context.Context, role, user string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_role_users(rolename, username) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		role, user)
	return mapErr(err)
}

func (s *Store) AssignRoleGroup(ctx context.Context, role, group string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_role_groups(rolename, groupname) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		role, group)
	return mapErr(err)
}

// RolesOf returns roles assigned directly and via group membership.
func (s *Store) RolesOf(ctx context.Context, user string) ([]string, error) {
	return s.listNames(ctx, `
		SELECT rolename FROM auth_role_users WHERE username=$1
		UNION
		SELECT rg.rolename FROM auth_role_groups rg
		JOIN auth_user_groups ug ON ug.groupname = rg.groupname
		WHERE ug.username=$1
		ORDER BY 1`, user)
}

func (s *Store) listNames(ctx context.Context, sql string, args ...any) ([]string, error) {
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateRule(ctx context.Context, r Rule) (int64, error) {
	if r.Attributes == nil {
		r.Attributes = []string{}
	}
	norm := func(v string) string {
		if v == "" {
			return "*"
		}
		return v
	}
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO geofence_rules(priority, username, rolename, service, request, workspace, layer, access, cql_read, cql_write, attributes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		r.Priority, norm(r.Username), norm(r.Rolename), norm(r.Service), norm(r.Request),
		norm(r.Workspace), norm(r.Layer), r.Access, r.CQLRead, r.CQLWrite, r.Attributes,
	).Scan(&id)
	return id, mapErr(err)
}

func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, priority, username, rolename, service, request, workspace, layer, access, cql_read, cql_write, attributes
		FROM geofence_rules ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Priority, &r.Username, &r.Rolename, &r.Service,
			&r.Request, &r.Workspace, &r.Layer, &r.Access, &r.CQLRead, &r.CQLWrite,
			&r.Attributes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRule(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM geofence_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SeedAdmin creates the GeoServer-default admin user and ADMIN role once.
func (s *Store) SeedAdmin(ctx context.Context, hash string) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO auth_users(name, enabled, password_hash) VALUES('admin', true, $1)
		 ON CONFLICT (name) DO NOTHING`, hash); err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx,
		`INSERT INTO auth_roles(name) VALUES('ADMIN') ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_role_users(rolename, username) VALUES('ADMIN','admin')
		 ON CONFLICT DO NOTHING`)
	return err
}
