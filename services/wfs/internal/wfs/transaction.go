package wfs

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
	"github.com/geoson/geoson/services/wfs/internal/meta"
)

// streamCSV writes CSV: header FID,<cols>,geom then a row per feature (geom as WKT).
func (h *handler) streamCSV(w http.ResponseWriter, ctx context.Context, p *gfParams,
	cols []meta.Column, where string, args []any) error {
	table, err := qi(p.layer.Table)
	if err != nil {
		return err
	}
	geom, err := qi(p.layer.GeomCol)
	if err != nil {
		return err
	}
	sel := make([]string, 0, len(cols)+1)
	header := []string{"FID"}
	for _, c := range cols {
		q, err := qi(c.Name)
		if err != nil {
			return err
		}
		sel = append(sel, q)
		header = append(header, c.Name)
	}
	header = append(header, p.layer.GeomCol)
	sql := "SELECT " + strings.Join(sel, ", ") + ", ST_AsText(" + geom + ") FROM " + table + where
	sql += orderLimitOffset(p, len(args))

	rows, err := p.layer.Conn.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	fmt.Fprintln(w, csvRow(header))
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return err
		}
		rec := make([]string, 0, len(vals)+1)
		fid := ""
		if len(cols) > 0 && vals[0] != nil {
			fid = fmt.Sprintf("%s.%v", p.layer.Table, vals[0])
		}
		rec = append(rec, fid)
		for _, v := range vals {
			rec = append(rec, fmt.Sprintf("%v", deref(v)))
		}
		fmt.Fprintln(w, csvRow(rec))
	}
	return rows.Err()
}

func deref(v any) any {
	if v == nil {
		return ""
	}
	return v
}

func csvRow(fields []string) string {
	out := make([]string, len(fields))
	for i, f := range fields {
		if strings.ContainsAny(f, ",\"\n") {
			out[i] = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
		} else {
			out[i] = f
		}
	}
	return strings.Join(out, ",")
}

// streamGML writes a wfs:FeatureCollection with GML geometry (gmlVer 2, 3, or 32).
func (h *handler) streamGML(w http.ResponseWriter, ctx context.Context, p *gfParams,
	cols []meta.Column, where string, args []any, matched int, version string, gmlVer int) error {
	table, err := qi(p.layer.Table)
	if err != nil {
		return err
	}
	geomCol, err := qi(p.layer.GeomCol)
	if err != nil {
		return err
	}
	sel := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		q, err := qi(c.Name)
		if err != nil {
			return err
		}
		sel = append(sel, q)
	}
	gmlDim := 2
	if gmlVer == 32 || gmlVer == 3 {
		gmlDim = 3
	}
	sql := "SELECT " + strings.Join(sel, ", ") +
		fmt.Sprintf(", ST_AsGML(%d, %s, 8) FROM %s%s", gmlDim, geomCol, table, where)
	sql += orderLimitOffset(p, len(args))

	rows, err := p.layer.Conn.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// member tag differs by GML version: 2.0 uses <wfs:member>, 1.x <gml:featureMember>
	memberTag := "wfs:member"
	if !strings.HasPrefix(version, "2.") {
		memberTag = "gml:featureMember"
	}
	var members strings.Builder
	returned := 0
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return err
		}
		fid := fmt.Sprintf("%s.%v", p.layer.Table, vals[0])
		members.WriteString("  <" + memberTag + "><geoson:" + p.layer.Table + ` gml:id="` + fid + `">`)
		for i, c := range cols {
			members.WriteString("<geoson:" + c.Name + ">")
			members.WriteString(xmlEsc(fmt.Sprintf("%v", deref(vals[i]))))
			members.WriteString("</geoson:" + c.Name + ">")
		}
		if g := vals[len(cols)]; g != nil {
			members.WriteString("<geoson:" + p.layer.GeomCol + ">")
			members.WriteString(fmt.Sprintf("%v", g))
			members.WriteString("</geoson:" + p.layer.GeomCol + ">")
		}
		members.WriteString("</geoson:" + p.layer.Table + "></" + memberTag + ">\n")
		returned++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/xml")
	wfsNS := "http://www.opengis.net/wfs"
	if strings.HasPrefix(version, "2.") {
		wfsNS = "http://www.opengis.net/wfs/2.0"
	}
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n")
	fmt.Fprintf(w, `<wfs:FeatureCollection xmlns:wfs=%q xmlns:gml="http://www.opengis.net/gml" xmlns:geoson="geoson"`, wfsNS)
	if strings.HasPrefix(version, "2.") {
		fmt.Fprintf(w, ` numberMatched="%d" numberReturned="%d"`, matched, returned)
	} else {
		fmt.Fprintf(w, ` numberOfFeatures="%d"`, matched)
	}
	fmt.Fprint(w, ">\n")
	w.Write([]byte(members.String()))
	fmt.Fprint(w, "</wfs:FeatureCollection>\n")
	return nil
}

func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// transaction is implemented in Task 8.
func (h *handler) transaction(w http.ResponseWriter, r *http.Request, body []byte, version string) {
	writeException(w, version, ows.CodeOperationNotSupported, "request",
		"Transaction pending", 501)
}
