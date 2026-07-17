package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type geojsonFC struct {
	Features []struct {
		Geometry   json.RawMessage        `json:"geometry"`
		Properties map[string]any         `json:"properties"`
	} `json:"features"`
}

// sanitizeIdent lowercases and keeps [a-z0-9_], ensuring a valid SQL identifier.
func sanitizeIdent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for i, r := range s {
		if r == '_' || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		out = "t_" + out
	}
	return out
}

// IngestGeoJSON creates a PostGIS table from a GeoJSON FeatureCollection and
// loads its features. Properties become typed columns; geometry is stored in
// EPSG:4326 so geometry_columns picks it up (making it servable by wms/wfs).
// Returns the geometry type of the first feature.
func (s *Store) IngestGeoJSON(ctx context.Context, table string, data []byte) (string, error) {
	table = sanitizeIdent(table)
	if !validTable(table) {
		return "", fmt.Errorf("invalid table name")
	}
	var fc geojsonFC
	if err := json.Unmarshal(data, &fc); err != nil {
		return "", fmt.Errorf("invalid GeoJSON: %w", err)
	}
	if len(fc.Features) == 0 {
		return "", fmt.Errorf("GeoJSON has no features")
	}

	// Infer property columns: numeric if every present value is a JSON number.
	numeric := map[string]bool{}
	order := []string{}
	seen := map[string]bool{}
	for _, f := range fc.Features {
		for k, v := range f.Properties {
			col := sanitizeIdent(k)
			if col == "" || col == "geom" || col == "gid" {
				continue
			}
			if !seen[col] {
				seen[col] = true
				order = append(order, col)
				numeric[col] = true
			}
			if v != nil {
				if _, ok := v.(float64); !ok {
					numeric[col] = false
				}
			}
		}
	}

	// geometry type of the first feature
	geomType := "Geometry"
	if len(fc.Features) > 0 {
		var g struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(fc.Features[0].Geometry, &g) == nil && g.Type != "" {
			geomType = g.Type
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	cols := []string{`gid bigserial primary key`, `geom geometry(Geometry,4326)`}
	for _, c := range order {
		t := "text"
		if numeric[c] {
			t = "double precision"
		}
		cols = append(cols, fmt.Sprintf(`"%s" %s`, c, t))
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, table)); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`CREATE TABLE "%s" (%s)`, table, strings.Join(cols, ", "))); err != nil {
		return "", err
	}

	// Insert each feature.
	for _, f := range fc.Features {
		if len(f.Geometry) == 0 || string(f.Geometry) == "null" {
			continue
		}
		names := []string{"geom"}
		placeholders := []string{"ST_SetSRID(ST_GeomFromGeoJSON($1),4326)"}
		args := []any{string(f.Geometry)}
		for _, c := range order {
			v, ok := f.Properties[origKey(f.Properties, c)]
			if !ok {
				continue
			}
			names = append(names, fmt.Sprintf(`"%s"`, c))
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
			if numeric[c] {
				args = append(args, v)
			} else {
				args = append(args, toText(v))
			}
		}
		q := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`, table,
			strings.Join(names, ", "), strings.Join(placeholders, ", "))
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			return "", fmt.Errorf("insert feature: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return geomType, nil
}

// origKey finds the original property key that sanitizes to col.
func origKey(props map[string]any, col string) string {
	for k := range props {
		if sanitizeIdent(k) == col {
			return k
		}
	}
	return col
}

func toText(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
