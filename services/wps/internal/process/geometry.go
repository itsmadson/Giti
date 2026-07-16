package process

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

func init() {
	runBuffer = wkt1f("SELECT ST_AsText(ST_Buffer(ST_GeomFromText($1), $2))", "geom", "distance")
	runCentroid = wkt0("SELECT ST_AsText(ST_Centroid(ST_GeomFromText($1)))", "geom")
	runArea = scalar("SELECT ST_Area(ST_GeomFromText($1))", "geom")
	runLength = scalar(
		"SELECT CASE WHEN ST_Length(g)=0 THEN ST_Perimeter(g) ELSE ST_Length(g) END FROM (SELECT ST_GeomFromText($1) g) s",
		"geom")
	runReproject = func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out string
		err := db.QueryRow(ctx,
			"SELECT ST_AsText(ST_Transform(ST_SetSRID(ST_GeomFromText($1), $2::int), $3::int))",
			in["geom"], in["sourceSRID"], in["targetSRID"]).Scan(&out)
		return out, err
	}
	runIntersection = wkt2("SELECT ST_AsText(ST_Intersection(ST_GeomFromText($1), ST_GeomFromText($2)))", "a", "b")
	runUnion = wkt2("SELECT ST_AsText(ST_Union(ST_GeomFromText($1), ST_GeomFromText($2)))", "a", "b")
	runSimplify = wkt1f("SELECT ST_AsText(ST_Simplify(ST_GeomFromText($1), $2))", "geom", "tolerance")
}

func wkt0(sql, g string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out string
		err := db.QueryRow(ctx, sql, in[g]).Scan(&out)
		return out, err
	}
}

func wkt1f(sql, g, f string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		val, err := strconv.ParseFloat(in[f], 64)
		if err != nil {
			return "", err
		}
		var out string
		err = db.QueryRow(ctx, sql, in[g], val).Scan(&out)
		return out, err
	}
}

func wkt2(sql, a, b string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out string
		err := db.QueryRow(ctx, sql, in[a], in[b]).Scan(&out)
		return out, err
	}
}

func scalar(sql, g string) func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return func(ctx context.Context, db *pgxpool.Pool, in map[string]string) (string, error) {
		var out float64
		if err := db.QueryRow(ctx, sql, in[g]).Scan(&out); err != nil {
			return "", err
		}
		return strconv.FormatFloat(out, 'f', -1, 64), nil
	}
}
