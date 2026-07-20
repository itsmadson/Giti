package wfs

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/giti/giti/libs/ogc-kit/ows"
	"github.com/giti/giti/services/wfs/internal/meta"
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
	geomExpr := geomCol
	if p.outSrid > 0 && p.outSrid != p.layer.Srid {
		geomExpr = fmt.Sprintf("ST_Transform(%s, %d)", geomCol, p.outSrid)
	}
	sql := "SELECT " + strings.Join(sel, ", ") +
		fmt.Sprintf(", ST_AsGML(%d, %s, 8) FROM %s%s", gmlDim, geomExpr, table, where)
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
		members.WriteString("  <" + memberTag + "><giti:" + p.layer.Table + ` gml:id="` + fid + `">`)
		for i, c := range cols {
			members.WriteString("<giti:" + c.Name + ">")
			members.WriteString(xmlEsc(fmt.Sprintf("%v", deref(vals[i]))))
			members.WriteString("</giti:" + c.Name + ">")
		}
		if g := vals[len(cols)]; g != nil {
			members.WriteString("<giti:" + p.layer.GeomCol + ">")
			members.WriteString(fmt.Sprintf("%v", g))
			members.WriteString("</giti:" + p.layer.GeomCol + ">")
		}
		members.WriteString("</giti:" + p.layer.Table + "></" + memberTag + ">\n")
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
	fmt.Fprintf(w, `<wfs:FeatureCollection xmlns:wfs=%q xmlns:gml="http://www.opengis.net/gml" xmlns:giti="giti"`, wfsNS)
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

// transaction handles WFS-T Insert/Update/Delete from a POST XML body.
func (h *handler) transaction(w http.ResponseWriter, r *http.Request, body []byte, version string) {
	root, err := decodeXMLTree(body)
	if err != nil {
		writeException(w, version, ows.CodeNoApplicableCode, "", err.Error(), 400)
		return
	}
	txLockID := root.attrs["lockId"]
	var inserted, updated, deleted int
	for _, op := range root.children {
		switch op.name {
		case "Insert":
			n, err := h.doInsert(r.Context(), op)
			if err != nil {
				writeException(w, version, ows.CodeNoApplicableCode, "Insert", err.Error(), 400)
				return
			}
			inserted += n
		case "Update":
			n, err := h.doUpdate(r.Context(), r, op, txLockID)
			if err != nil {
				writeException(w, version, ows.CodeNoApplicableCode, "Update", err.Error(), 400)
				return
			}
			updated += n
		case "Delete":
			n, err := h.doDelete(r.Context(), r, op, txLockID)
			if err != nil {
				writeException(w, version, ows.CodeNoApplicableCode, "Delete", err.Error(), 400)
				return
			}
			deleted += n
		}
	}
	w.Header().Set("Content-Type", "text/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
		`<wfs:TransactionResponse xmlns:wfs="http://www.opengis.net/wfs">`+
		`<wfs:TransactionSummary>`+
		`<wfs:totalInserted>%d</wfs:totalInserted>`+
		`<wfs:totalUpdated>%d</wfs:totalUpdated>`+
		`<wfs:totalDeleted>%d</wfs:totalDeleted>`+
		`</wfs:TransactionSummary></wfs:TransactionResponse>`,
		inserted, updated, deleted)
}

// resolveTxLayer resolves a transaction target. Insert uses the feature
// element local name "ws--table"; Update/Delete use typeName="ws:table".
func (h *handler) resolveTxLayer(ctx context.Context, wsTable string) (*meta.Layer, error) {
	ws, name := "", wsTable
	if i := strings.Index(wsTable, "--"); i >= 0 {
		ws, name = wsTable[:i], wsTable[i+2:]
	} else if i := strings.IndexByte(wsTable, ':'); i >= 0 {
		ws, name = wsTable[:i], wsTable[i+1:]
	}
	return h.m.Resolve(ctx, ws, name)
}

func (h *handler) doInsert(ctx context.Context, op *xmlNode) (int, error) {
	count := 0
	for _, feat := range op.children {
		layer, err := h.resolveTxLayer(ctx, feat.name)
		if err != nil {
			return count, err
		}
		table, err := qi(layer.Table)
		if err != nil {
			return count, err
		}
		var colNames []string
		var placeholders []string
		var args []any
		argn := 1
		for _, prop := range feat.children {
			col := prop.name
			q, err := qi(col)
			if err != nil {
				return count, err
			}
			colNames = append(colNames, q)
			if col == layer.GeomCol {
				wkt, ok := gmlNodeToWKT(prop)
				if !ok {
					return count, fmt.Errorf("invalid geometry for %s", col)
				}
				placeholders = append(placeholders,
					fmt.Sprintf("ST_GeomFromText($%d, 4326)", argn))
				args = append(args, wkt)
			} else {
				placeholders = append(placeholders, fmt.Sprintf("$%d", argn))
				args = append(args, strings.TrimSpace(prop.text))
			}
			argn++
		}
		sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			table, strings.Join(colNames, ", "), strings.Join(placeholders, ", "))
		if _, err := layer.Conn.Exec(ctx, sql, args...); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (h *handler) doUpdate(ctx context.Context, r *http.Request, op *xmlNode, txLockID string) (int, error) {
	layer, err := h.resolveTxLayer(ctx, op.attrs["typeName"])
	if err != nil {
		return 0, err
	}
	table, err := qi(layer.Table)
	if err != nil {
		return 0, err
	}
	// lock enforcement: resolve the op's target features (standalone WHERE)
	if lw, la, werr := txFilterWhere(r, op, 1); werr == nil {
		if err := h.enforceLocks(ctx, layer, lw, la, txLockID); err != nil {
			return 0, err
		}
	}
	var sets []string
	var args []any
	argn := 1
	for _, prop := range op.children {
		if prop.name != "Property" {
			continue
		}
		var name, value string
		for _, c := range prop.children {
			switch c.name {
			case "Name", "ValueReference":
				name = strings.TrimSpace(c.text)
			case "Value":
				value = strings.TrimSpace(c.text)
			}
		}
		q, err := qi(name)
		if err != nil {
			return 0, err
		}
		sets = append(sets, fmt.Sprintf("%s = $%d", q, argn))
		args = append(args, value)
		argn++
	}
	where, wargs, err := txFilterWhere(r, op, argn)
	if err != nil {
		return 0, err
	}
	args = append(args, wargs...)
	sql := fmt.Sprintf("UPDATE %s SET %s%s", table, strings.Join(sets, ", "), where)
	tag, err := layer.Conn.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (h *handler) doDelete(ctx context.Context, r *http.Request, op *xmlNode, txLockID string) (int, error) {
	layer, err := h.resolveTxLayer(ctx, op.attrs["typeName"])
	if err != nil {
		return 0, err
	}
	table, err := qi(layer.Table)
	if err != nil {
		return 0, err
	}
	where, args, err := txFilterWhere(r, op, 1)
	if err != nil {
		return 0, err
	}
	if err := h.enforceLocks(ctx, layer, where, args, txLockID); err != nil {
		return 0, err
	}
	sql := fmt.Sprintf("DELETE FROM %s%s", table, where)
	tag, err := layer.Conn.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
