package wfs

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/giti/giti/libs/ogc-kit/filter"
	"github.com/giti/giti/libs/ogc-kit/ows"
	"github.com/giti/giti/services/wfs/internal/meta"
)

// splitTypeName parses "ws:layer" or bare "layer" (ws from header/path).
func splitTypeName(tn, headerWS string) (ws, layer string) {
	tn = strings.TrimSpace(tn)
	if i := strings.IndexByte(tn, ':'); i >= 0 {
		return tn[:i], tn[i+1:]
	}
	return headerWS, tn
}

type gfParams struct {
	layer     *meta.Layer
	filter    filter.Expr
	props     []string
	sortCol   string
	sortDir   string
	offset    int
	limit     int
	countOnly bool
	format    string
	outSrid   int    // SRSNAME requested output CRS (0 = native, no reprojection)
	valueRef  string // GetPropertyValue: the single property to return
}

// sridOf extracts the numeric EPSG code from an SRS/CRS string
// ("EPSG:3857", "urn:ogc:def:crs:EPSG::4326", "http://www.opengis.net/def/crs/EPSG/0/3857").
func sridOf(s string) int {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(fields[len(fields)-1])
	return n
}

// parseBBox turns "minx,miny,maxx,maxy[,srs]" into a filter.BBox on geomCol.
func parseBBox(raw, geomCol string) (filter.Expr, error) {
	parts := strings.Split(raw, ",")
	if len(parts) < 4 {
		return nil, fmt.Errorf("BBOX needs 4 numbers")
	}
	f := make([]float64, 4)
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil {
			return nil, err
		}
		f[i] = v
	}
	return filter.BBox{Prop: geomCol, MinX: f[0], MinY: f[1], MaxX: f[2], MaxY: f[3]}, nil
}

func andExpr(a, b filter.Expr) filter.Expr {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return filter.Logic{Op: "AND", Exprs: []filter.Expr{a, b}}
}

func (h *handler) parseGetFeature(r *http.Request, req ows.Request) (*gfParams, error) {
	headerWS := r.Header.Get("X-Giti-Workspace")
	tn := req.Get("typeNames")
	if tn == "" {
		tn = req.Get("typeName")
	}
	// Stored query: the mandatory built-in GetFeatureById (ID = "typeName.fid").
	var storedFid string
	if sq := req.Get("STOREDQUERY_ID"); sq != "" && strings.Contains(sq, "GetFeatureById") {
		id := req.Get("ID")
		if id == "" {
			return nil, fmt.Errorf("stored query GetFeatureById requires ID")
		}
		if i := strings.LastIndex(id, "."); i >= 0 {
			tn, storedFid = id[:i], id[i+1:]
		} else {
			return nil, fmt.Errorf("ID must be typeName.fid")
		}
	}
	if tn == "" {
		return nil, fmt.Errorf("typeNames parameter required")
	}
	ws, name := splitTypeName(tn, headerWS)
	layer, err := h.m.Resolve(r.Context(), ws, name)
	if err != nil {
		return nil, err
	}

	p := &gfParams{layer: layer, format: req.Get("outputFormat")}

	// SRSNAME: reproject output geometry to the requested CRS (WFS 1.1/2.0)
	if srs := req.Get("srsName"); srs != "" {
		if s := sridOf(srs); s > 0 {
			p.outSrid = s
		}
	}

	// filters: CQL_FILTER, BBOX, auth CQL-Read, featureID
	var combined filter.Expr
	if cql := req.Get("CQL_FILTER"); cql != "" {
		e, err := filter.ParseCQL(cql)
		if err != nil {
			return nil, fmt.Errorf("invalid CQL_FILTER: %w", err)
		}
		combined = andExpr(combined, e)
	}
	if fx := req.Get("FILTER"); fx != "" {
		e, err := filter.ParseFilterXML([]byte(fx))
		if err != nil {
			return nil, fmt.Errorf("invalid FILTER: %w", err)
		}
		combined = andExpr(combined, e)
	}
	if bbox := req.Get("BBOX"); bbox != "" {
		e, err := parseBBox(bbox, layer.GeomCol)
		if err != nil {
			return nil, err
		}
		combined = andExpr(combined, e)
	}
	if fid := req.Get("featureID"); fid != "" {
		combined = andExpr(combined, featureIDFilter(fid))
	}
	if storedFid != "" {
		combined = andExpr(combined, featureIDFilter(storedFid))
	}
	if authCQL := r.Header.Get("X-Giti-CQL-Read"); authCQL != "" {
		e, err := filter.ParseCQL(authCQL)
		if err == nil {
			combined = andExpr(combined, e)
		}
	}
	p.filter = combined

	// propertyName subset
	if pn := req.Get("propertyName"); pn != "" {
		for _, c := range strings.Split(pn, ",") {
			p.props = append(p.props, strings.TrimSpace(c))
		}
	}
	// GetPropertyValue: valueReference selects the single property to return
	if vr := req.Get("valueReference"); vr != "" {
		p.valueRef = strings.TrimSpace(vr)
		p.props = []string{p.valueRef}
	}

	// sortBy: "col" or "col A"/"col D" or "col+D"
	if sb := req.Get("sortBy"); sb != "" {
		sb = strings.ReplaceAll(sb, "+", " ")
		fields := strings.Fields(sb)
		p.sortCol = fields[0]
		p.sortDir = "ASC"
		if len(fields) > 1 && (strings.EqualFold(fields[1], "D") || strings.EqualFold(fields[1], "DESC")) {
			p.sortDir = "DESC"
		}
	}

	// paging
	p.offset = atoiDefault(req.Get("startIndex"), 0)
	count := req.Get("count")
	if count == "" {
		count = req.Get("maxFeatures")
	}
	p.limit = atoiDefault(count, 0)

	if strings.EqualFold(req.Get("resultType"), "hits") {
		p.countOnly = true
	}
	return p, nil
}

func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return d
}

// featureIDFilter turns "layer.3" or "3" into a primary-key equality.
func featureIDFilter(fid string) filter.Expr {
	id := fid
	if i := strings.LastIndexByte(fid, '.'); i >= 0 {
		id = fid[i+1:]
	}
	if v, err := strconv.ParseFloat(id, 64); err == nil {
		return filter.Compare{Op: "=", Left: filter.Property{Name: "id"}, Right: filter.Literal{Value: v}}
	}
	return filter.Compare{Op: "=", Left: filter.Property{Name: "id"}, Right: filter.Literal{Value: id}}
}

// selectColumns returns the attribute columns to emit (propertyName ∩ layer cols).
func selectColumns(l *meta.Layer, props []string) []meta.Column {
	if len(props) == 0 {
		return l.Columns
	}
	want := map[string]bool{}
	for _, p := range props {
		want[strings.ToLower(p)] = true
	}
	var out []meta.Column
	for _, c := range l.Columns {
		if want[strings.ToLower(c.Name)] {
			out = append(out, c)
		}
	}
	return out
}

// whereClause builds the WHERE fragment + args (returns "" when no filter).
func whereClause(e filter.Expr, startArg int) (string, []any, error) {
	if e == nil {
		return "", nil, nil
	}
	sql, args, err := filter.ToSQL(e, startArg)
	if err != nil {
		return "", nil, err
	}
	return " WHERE " + sql, args, nil
}

// getFeature parses the request, counts matches, and streams the response
// in the negotiated output format.
func (h *handler) getFeature(w http.ResponseWriter, r *http.Request, req ows.Request, version string) {
	p, err := h.parseGetFeature(r, req)
	if err != nil {
		writeException(w, version, ows.CodeInvalidParameterValue, "typeNames", err.Error(), 400)
		return
	}
	where, args, err := whereClause(p.filter, 1)
	if err != nil {
		writeException(w, version, ows.CodeInvalidParameterValue, "filter", err.Error(), 400)
		return
	}
	matched, err := h.countMatched(r.Context(), p.layer, where, args)
	if err != nil {
		writeException(w, version, ows.CodeNoApplicableCode, "", err.Error(), 500)
		return
	}
	if p.countOnly {
		writeHits(w, version, matched)
		return
	}
	// GetPropertyValue → wfs:ValueCollection of the single property's values.
	if p.valueRef != "" {
		if err := h.streamValues(w, r.Context(), p, where, args, matched); err != nil {
			writeException(w, version, ows.CodeNoApplicableCode, "", err.Error(), 500)
		}
		return
	}
	cols := selectColumns(p.layer, p.props)
	switch wireFormat(p.format, version) {
	case "geojson":
		err = h.streamGeoJSON(w, r.Context(), p, cols, where, args, matched)
	case "csv":
		err = h.streamCSV(w, r.Context(), p, cols, where, args)
	case "gml2":
		err = h.streamGML(w, r.Context(), p, cols, where, args, matched, version, 2)
	case "gml3":
		err = h.streamGML(w, r.Context(), p, cols, where, args, matched, version, 3)
	default:
		err = h.streamGML(w, r.Context(), p, cols, where, args, matched, version, 32)
	}
	if err != nil {
		// headers likely already sent; log-only path — best effort
		fmt.Fprintf(w, "<!-- stream error: %v -->", err)
	}
}
