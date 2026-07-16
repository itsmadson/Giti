package wfs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/geoson/geoson/services/wfs/internal/meta"
	"github.com/jackc/pgx/v5"
)

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func qi(name string) (string, error) {
	if !identRe.MatchString(name) {
		return "", fmt.Errorf("invalid identifier %q", name)
	}
	return `"` + name + `"`, nil
}

// wireFormat resolves the output encoding from outputFormat + version.
func wireFormat(outputFormat, version string) string {
	of := strings.ToLower(outputFormat)
	switch {
	case strings.Contains(of, "json"):
		return "geojson"
	case strings.Contains(of, "csv"):
		return "csv"
	case strings.Contains(of, "gml2") || of == "gml" && strings.HasPrefix(version, "1.0"):
		return "gml2"
	case strings.HasPrefix(version, "1.0"):
		return "gml2"
	case strings.HasPrefix(version, "1.1"):
		return "gml3"
	default:
		return "gml32"
	}
}

// countMatched runs SELECT count(*) with the same filter.
func (h *handler) countMatched(ctx context.Context, l *meta.Layer, where string, args []any) (int, error) {
	table, err := qi(l.Table)
	if err != nil {
		return 0, err
	}
	var n int
	err = l.Conn.QueryRow(ctx, "SELECT count(*) FROM "+table+where, args...).Scan(&n)
	return n, err
}

// runGeoJSON streams a FeatureCollection. cols are attribute columns.
func (h *handler) streamGeoJSON(w http.ResponseWriter, ctx context.Context, p *gfParams,
	cols []meta.Column, where string, args []any, matched int) error {
	table, err := qi(p.layer.Table)
	if err != nil {
		return err
	}
	geom, err := qi(p.layer.GeomCol)
	if err != nil {
		return err
	}
	selCols := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		q, err := qi(c.Name)
		if err != nil {
			return err
		}
		selCols = append(selCols, q)
	}
	sql := "SELECT " + strings.Join(selCols, ", ") +
		", ST_AsGeoJSON(" + geom + ") FROM " + table + where
	sql += orderLimitOffset(p, len(args))

	rows, err := p.layer.Conn.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"type":"FeatureCollection","features":[`)
	returned := 0
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return err
		}
		if returned > 0 {
			fmt.Fprint(w, ",")
		}
		props := map[string]any{}
		for i, c := range cols {
			props[c.Name] = vals[i]
		}
		geomJSON := "null"
		if g := vals[len(cols)]; g != nil {
			geomJSON = fmt.Sprint(g)
		}
		propBuf, _ := json.Marshal(props)
		id := ""
		if len(cols) > 0 {
			if v := props[cols[0].Name]; v != nil {
				id = fmt.Sprintf("%s.%v", p.layer.Table, v)
			}
		}
		fmt.Fprintf(w, `{"type":"Feature","id":%q,"geometry":%s,"properties":%s}`,
			id, geomJSON, propBuf)
		returned++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Fprintf(w, `],"totalFeatures":%d,"numberMatched":%d,"numberReturned":%d}`,
		matched, matched, returned)
	return nil
}

func orderLimitOffset(p *gfParams, nargs int) string {
	var b strings.Builder
	if p.sortCol != "" {
		if q, err := qi(p.sortCol); err == nil {
			b.WriteString(" ORDER BY " + q + " " + p.sortDir)
		}
	}
	if p.offset > 0 {
		fmt.Fprintf(&b, " OFFSET %d", p.offset)
	}
	if p.limit > 0 {
		fmt.Fprintf(&b, " LIMIT %d", p.limit)
	}
	return b.String()
}

var _ = pgx.ErrNoRows
