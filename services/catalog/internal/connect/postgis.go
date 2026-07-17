package connect

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/giti/giti/services/catalog/internal/model"
	"github.com/jackc/pgx/v5"
)

func init() {
	register("PostGIS", postgis{})
	registerMeta(StoreTypeMeta{
		Type: "PostGIS", Kind: "datastore", Category: "Vector", Label: "PostGIS Database",
		Params: []ParamField{
			{Key: "host", Label: "Host", Type: "text", Default: "self", Required: true},
			{Key: "port", Label: "Port", Type: "number", Default: "5432", Required: false},
			{Key: "database", Label: "Database", Type: "text", Required: false},
			{Key: "user", Label: "User", Type: "text", Required: false},
			{Key: "passwd", Label: "Password", Type: "password", Required: false},
			{Key: "schema", Label: "Schema", Type: "text", Default: "public", Required: false},
		},
	})
}

type postgis struct{}

// dsn builds a pgx DSN from GeoServer-style connection params
// (host, port, database, user, passwd, schema). The special host "self"
// means "this service's own database" — resolved from GITI_DATABASE_URL.
func (postgis) dsn(st model.Store) string {
	c := st.Connection
	if c["host"] == "self" {
		if env := os.Getenv("GITI_DATABASE_URL"); env != "" {
			return env
		}
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		c["user"], c["passwd"], c["host"], c["port"], c["database"])
}

func (p postgis) Validate(ctx context.Context, st model.Store) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, p.dsn(st))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	return conn.Ping(ctx)
}

var geomTypes = map[string]string{
	"POINT": "Point", "MULTIPOINT": "MultiPoint",
	"LINESTRING": "LineString", "MULTILINESTRING": "MultiLineString",
	"POLYGON": "Polygon", "MULTIPOLYGON": "MultiPolygon",
	"GEOMETRY": "Geometry", "GEOMETRYCOLLECTION": "GeometryCollection",
}

func normalizeGeomType(t string) string {
	if v, ok := geomTypes[t]; ok {
		return v
	}
	return t
}

func (p postgis) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	conn, err := pgx.Connect(ctx, p.dsn(st))
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	schema := st.Connection["schema"]
	if schema == "" {
		schema = "public"
	}
	rows, err := conn.Query(ctx, `
		SELECT f_table_name, type, 'EPSG:' || srid
		FROM geometry_columns
		WHERE f_table_schema = $1
		ORDER BY f_table_name`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceInfo
	for rows.Next() {
		var ri ResourceInfo
		if err := rows.Scan(&ri.Name, &ri.GeometryType, &ri.SRS); err != nil {
			return nil, err
		}
		ri.GeometryType = normalizeGeomType(ri.GeometryType)
		out = append(out, ri)
	}
	return out, rows.Err()
}
