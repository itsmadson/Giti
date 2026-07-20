package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/giti/giti/libs/ogc-kit/filter"
)

// FeaturesGeoJSON returns an OGC API - Features GeoJSON FeatureCollection for a
// published featuretype, honoring limit and an optional bbox (minx,miny,maxx,maxy
// in EPSG:4326). Reads the table from this database (host=self stores).
func (s *Store) FeaturesGeoJSON(ctx context.Context, ws, name string, limit, offset int, bbox []float64, cql string) ([]byte, error) {
	d, err := s.GetLayerDetail(ctx, ws, name)
	if err != nil {
		return nil, err
	}
	if d.GeomColumn == "" || !validTable(d.Table) || !validTable(d.GeomColumn) {
		return nil, fmt.Errorf("layer %s has no readable geometry", name)
	}
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	preds := []string{}
	args := []any{}
	if len(bbox) == 4 {
		preds = append(preds, fmt.Sprintf(
			`ST_Intersects(ST_Transform("%s",4326), ST_MakeEnvelope($1,$2,$3,$4,4326))`, d.GeomColumn))
		args = append(args, bbox[0], bbox[1], bbox[2], bbox[3])
	}
	if strings.TrimSpace(cql) != "" {
		e, perr := filter.ParseCQL(cql)
		if perr != nil {
			return nil, fmt.Errorf("invalid filter: %w", perr)
		}
		frag, fargs, ferr := filter.ToSQL(e, len(args)+1)
		if ferr != nil {
			return nil, ferr
		}
		preds = append(preds, "("+frag+")")
		args = append(args, fargs...)
	}
	where := ""
	if len(preds) > 0 {
		where = "WHERE " + strings.Join(preds, " AND ")
	}

	// FeatureCollection with numberMatched/numberReturned per OGC API-Features.
	q := fmt.Sprintf(`
		SELECT jsonb_build_object(
			'type','FeatureCollection',
			'numberMatched', (SELECT count(*) FROM "%s" %s),
			'numberReturned', COALESCE(jsonb_array_length(f.features),0),
			'features', COALESCE(f.features,'[]'::jsonb)
		)::text
		FROM (
			SELECT jsonb_agg(jsonb_build_object(
				'type','Feature',
				'geometry', ST_AsGeoJSON(ST_Transform("%s",4326))::jsonb,
				'properties', to_jsonb(t) - '%s'
			)) features
			FROM (SELECT * FROM "%s" %s LIMIT %d OFFSET %d) t
		) f`,
		d.Table, where, d.GeomColumn, d.GeomColumn, d.Table, where, limit, offset)
	var out string
	if err := s.db.QueryRow(ctx, q, args...).Scan(&out); err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// ParseBbox parses a comma-separated "minx,miny,maxx,maxy" string.
func ParseBbox(s string) []float64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil
	}
	out := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil
		}
		out[i] = v
	}
	return out
}
