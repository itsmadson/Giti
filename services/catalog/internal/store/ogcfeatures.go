package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// FeaturesGeoJSON returns an OGC API - Features GeoJSON FeatureCollection for a
// published featuretype, honoring limit and an optional bbox (minx,miny,maxx,maxy
// in EPSG:4326). Reads the table from this database (host=self stores).
func (s *Store) FeaturesGeoJSON(ctx context.Context, ws, name string, limit int, bbox []float64) ([]byte, error) {
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
	where := ""
	args := []any{}
	if len(bbox) == 4 {
		where = fmt.Sprintf(
			`WHERE ST_Intersects(ST_Transform("%s",4326), ST_MakeEnvelope($1,$2,$3,$4,4326))`,
			d.GeomColumn)
		args = append(args, bbox[0], bbox[1], bbox[2], bbox[3])
	}
	// Build a FeatureCollection with ST_AsGeoJSON per row, properties = all
	// non-geometry columns.
	q := fmt.Sprintf(`
		SELECT COALESCE(jsonb_build_object(
			'type','FeatureCollection',
			'features', COALESCE(jsonb_agg(jsonb_build_object(
				'type','Feature',
				'geometry', ST_AsGeoJSON(ST_Transform("%s",4326))::jsonb,
				'properties', to_jsonb(t) - '%s'
			)), '[]'::jsonb)
		))::text
		FROM (SELECT * FROM "%s" %s LIMIT %d) t`,
		d.GeomColumn, d.GeomColumn, d.Table, where, limit)
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
