// Package store is the catalog repository over Postgres.
package store

import (
	"context"
	"errors"

	"github.com/geoson/geoson/services/catalog/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

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

func (s *Store) CreateWorkspace(ctx context.Context, w model.Workspace) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO workspaces(name, isolated, namespace_uri) VALUES($1,$2,$3)`,
		w.Name, w.Isolated, w.NamespaceURI)
	return mapErr(err)
}

func (s *Store) GetWorkspace(ctx context.Context, name string) (model.Workspace, error) {
	var w model.Workspace
	err := s.db.QueryRow(ctx,
		`SELECT name, isolated, namespace_uri FROM workspaces WHERE name=$1`, name,
	).Scan(&w.Name, &w.Isolated, &w.NamespaceURI)
	return w, mapErr(err)
}

func (s *Store) ListWorkspaces(ctx context.Context) ([]model.Workspace, error) {
	rows, err := s.db.Query(ctx,
		`SELECT name, isolated, namespace_uri FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Workspace
	for rows.Next() {
		var w model.Workspace
		if err := rows.Scan(&w.Name, &w.Isolated, &w.NamespaceURI); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) UpdateWorkspace(ctx context.Context, name string, w model.Workspace) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspaces SET name=$2, isolated=$3 WHERE name=$1`, name, w.Name, w.Isolated)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteWorkspace(ctx context.Context, name string, recurse bool) error {
	if !recurse {
		var n int
		if err := s.db.QueryRow(ctx,
			`SELECT count(*) FROM stores WHERE workspace=$1`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM workspaces WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
