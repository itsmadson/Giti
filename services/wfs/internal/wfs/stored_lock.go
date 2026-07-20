package wfs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/giti/giti/libs/ogc-kit/ows"
)

// streamValues emits a wfs:ValueCollection for GetPropertyValue.
func (h *handler) streamValues(w http.ResponseWriter, ctx context.Context, p *gfParams,
	where string, args []any, matched int) error {
	table, err := qi(p.layer.Table)
	if err != nil {
		return err
	}
	col, err := qi(p.valueRef)
	if err != nil {
		return err
	}
	sql := "SELECT " + col + "::text FROM " + table + where + orderLimitOffset(p, len(args))
	rows, err := p.layer.Conn.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "application/xml")
	returned := 0
	var b strings.Builder
	for rows.Next() {
		var v *string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		val := ""
		if v != nil {
			val = escapeXML(*v)
		}
		b.WriteString(fmt.Sprintf("  <wfs:member><giti:%s>%s</giti:%s></wfs:member>\n",
			p.valueRef, val, p.valueRef))
		returned++
	}
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
		`<wfs:ValueCollection xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:giti="giti" `+
		`numberMatched="%d" numberReturned="%d">`+"\n%s</wfs:ValueCollection>\n",
		matched, returned, b.String())
	return nil
}

func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

const getFeatureByIdURN = "urn:ogc:def:query:OGC-WFS::GetFeatureById"

// listStoredQueries advertises the mandatory built-in stored query.
func (h *handler) listStoredQueries(w http.ResponseWriter, version string) {
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
		`<wfs:ListStoredQueriesResponse xmlns:wfs="http://www.opengis.net/wfs/2.0">`+
		`<wfs:StoredQuery id="%s"><wfs:Title>Get feature by identifier</wfs:Title>`+
		`<wfs:ReturnFeatureType>ogc:AbstractFeatureType</wfs:ReturnFeatureType></wfs:StoredQuery>`+
		`</wfs:ListStoredQueriesResponse>`, getFeatureByIdURN)
}

// describeStoredQueries describes the built-in GetFeatureById query.
func (h *handler) describeStoredQueries(w http.ResponseWriter, version string) {
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
		`<wfs:DescribeStoredQueriesResponse xmlns:wfs="http://www.opengis.net/wfs/2.0">`+
		`<wfs:StoredQueryDescription id="%s"><wfs:Title>Get feature by identifier</wfs:Title>`+
		`<wfs:Parameter name="ID" type="xsd:string"/>`+
		`<wfs:QueryExpressionText isPrivate="false" language="urn:ogc:def:queryLanguage:OGC-WFS::WFS_QueryExpression" returnFeatureTypes=""/>`+
		`</wfs:StoredQueryDescription></wfs:DescribeStoredQueriesResponse>`, getFeatureByIdURN)
}

// --- advisory feature locks (in-memory, with expiry) ---

type lockEntry struct {
	expires time.Time
}

var (
	lockMu    sync.Mutex
	lockStore = map[string]lockEntry{}
)

func newLockID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "GITI_" + hex.EncodeToString(b)
}

func lockExpiry(min int) time.Duration {
	if min <= 0 {
		min = 5
	}
	return time.Duration(min) * time.Minute
}

// lockFeature matches features like GetFeature and returns a LockId. Locks are
// advisory (tracked with expiry); Transaction does not yet enforce lockId.
func (h *handler) lockFeature(w http.ResponseWriter, r *http.Request, req ows.Request, version string) {
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
	id := newLockID()
	exp := 5
	if v := req.Get("expiry"); v != "" {
		fmt.Sscanf(v, "%d", &exp)
	}
	lockMu.Lock()
	lockStore[id] = lockEntry{expires: time.Now().Add(lockExpiry(exp))}
	lockMu.Unlock()

	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
		`<wfs:LockFeatureResponse xmlns:wfs="http://www.opengis.net/wfs/2.0" lockId="%s" `+
		`numberLocked="%d"><wfs:FeaturesLocked/></wfs:LockFeatureResponse>`, id, matched)
}
