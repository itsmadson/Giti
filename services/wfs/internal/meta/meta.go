// Package meta resolves WFS layer metadata from the catalog's Postgres tables
// (read-only) and introspects data-table schemas. It shares the catalog DB.
package meta

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("layer not found")

type Column struct {
	Name string
	Type string // pg data_type
}

type Layer struct {
	Workspace, Name, NativeName, SRS string
	Table, GeomCol, GeomType         string
	Columns                          []Column
	Conn                             *pgxpool.Pool // data store pool
}

type cached struct {
	layer *Layer
	at    time.Time
}

type Meta struct {
	catalog *pgxpool.Pool
	mu      sync.RWMutex
	cache   map[string]cached
	pools   map[string]*pgxpool.Pool // store connection pools by dsn
}

func New(catalogDB *pgxpool.Pool) *Meta {
	return &Meta{catalog: catalogDB, cache: map[string]cached{}, pools: map[string]*pgxpool.Pool{}}
}

const cacheTTL = 60 * time.Second

// Resolve returns metadata for ws:name, introspecting the data table.
func (m *Meta) Resolve(ctx context.Context, ws, name string) (*Layer, error) {
	key := ws + ":" + name
	m.mu.RLock()
	if c, ok := m.cache[key]; ok && time.Since(c.at) < cacheTTL {
		m.mu.RUnlock()
		return c.layer, nil
	}
	m.mu.RUnlock()

	var nativeName, srs, storeName string
	err := m.catalog.QueryRow(ctx, `
		SELECT r.native_name, r.srs, r.store
		FROM resources r
		WHERE r.workspace=$1 AND r.name=$2 AND r.kind='featuretype'`,
		ws, name).Scan(&nativeName, &srs, &storeName)
	if err != nil {
		return nil, ErrNotFound
	}

	// store connection params
	var conn map[string]string
	if err := m.catalog.QueryRow(ctx,
		`SELECT connection FROM stores WHERE workspace=$1 AND name=$2`,
		ws, storeName).Scan(&conn); err != nil {
		return nil, err
	}
	pool, err := m.poolFor(ctx, conn)
	if err != nil {
		return nil, err
	}

	l := &Layer{Workspace: ws, Name: name, NativeName: nativeName, SRS: srs,
		Table: nativeName, Conn: pool}

	// geometry column
	if err := pool.QueryRow(ctx, `
		SELECT f_geometry_column, type FROM geometry_columns
		WHERE f_table_name=$1 LIMIT 1`, nativeName).Scan(&l.GeomCol, &l.GeomType); err != nil {
		return nil, err
	}

	// non-geometry columns
	rows, err := pool.Query(ctx, `
		SELECT column_name, data_type FROM information_schema.columns
		WHERE table_name=$1 AND column_name <> $2
		ORDER BY ordinal_position`, nativeName, l.GeomCol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.Name, &c.Type); err != nil {
			return nil, err
		}
		l.Columns = append(l.Columns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.cache[key] = cached{layer: l, at: time.Now()}
	m.mu.Unlock()
	return l, nil
}

// poolFor returns a pool for the given store connection params. host="self"
// (or missing) uses the catalog pool.
func (m *Meta) poolFor(ctx context.Context, conn map[string]string) (*pgxpool.Pool, error) {
	host := conn["host"]
	if host == "" || host == "self" {
		return m.catalog, nil
	}
	dsn := "postgres://" + conn["user"] + ":" + conn["passwd"] + "@" +
		host + ":" + conn["port"] + "/" + conn["database"]
	if host == "self-env" {
		dsn = os.Getenv("GITI_DATABASE_URL")
	}
	m.mu.RLock()
	p, ok := m.pools[dsn]
	m.mu.RUnlock()
	if ok {
		return p, nil
	}
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.pools[dsn] = p
	m.mu.Unlock()
	return p, nil
}

// ListFeatureTypes lists all vector layers for GetCapabilities.
func (m *Meta) ListFeatureTypes(ctx context.Context) ([]Layer, error) {
	rows, err := m.catalog.Query(ctx, `
		SELECT l.workspace, l.name, r.native_name, r.srs
		FROM layers l
		JOIN resources r ON r.workspace=l.workspace AND r.name=l.resource_name AND r.kind='featuretype'
		WHERE l.type='VECTOR' AND l.enabled
		ORDER BY l.workspace, l.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Layer
	for rows.Next() {
		var l Layer
		if err := rows.Scan(&l.Workspace, &l.Name, &l.NativeName, &l.SRS); err != nil {
			return nil, err
		}
		l.Table = l.NativeName
		out = append(out, l)
	}
	return out, rows.Err()
}
