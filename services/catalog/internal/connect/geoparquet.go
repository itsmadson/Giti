package connect

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/giti/giti/services/catalog/internal/model"
	_ "github.com/marcboeker/go-duckdb/v2"
)

func init() { register("GeoParquet", geoparquet{}) }

type geoparquet struct{}

func (geoparquet) open() (*sql.DB, error) { return sql.Open("duckdb", "") }

func (g geoparquet) Validate(ctx context.Context, st model.Store) error {
	db, err := g.open()
	if err != nil {
		return err
	}
	defer db.Close()
	path := storePath(st)
	// parquet_schema errors on non-parquet input
	var count int
	err = db.QueryRowContext(ctx,
		`SELECT count(*) FROM parquet_schema($1)`, path).Scan(&count)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if count == 0 {
		return fmt.Errorf("%s: empty parquet schema", path)
	}
	return nil
}

func (g geoparquet) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := g.Validate(ctx, st); err != nil {
		return nil, err
	}
	path := storePath(st)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	db, err := g.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	// Detect geometry column: GeoParquet stores geo metadata in file KV;
	// fall back to a column literally named "geometry"/"geom".
	ri := ResourceInfo{Name: name, SRS: "EPSG:4326"}
	rows, err := db.QueryContext(ctx, `SELECT name FROM parquet_schema($1)`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		if col == "geometry" || col == "geom" {
			ri.GeometryType = "Geometry"
		}
	}
	return []ResourceInfo{ri}, rows.Err()
}
