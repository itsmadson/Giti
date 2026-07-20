package rest

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CSW 2.0.2 (Catalogue Service for the Web) — Dublin Core records over the
// catalog's published layers. Served at /csw (via gateway), /giti/csw, /api/v1/csw.
func (a *api) cswRoutes(mux *http.ServeMux) {
	for _, base := range []string{"/csw", "/giti/csw", "/api/v1/csw"} {
		mux.HandleFunc("GET "+base, a.csw)
		mux.HandleFunc("POST "+base, a.csw)
	}
}

func (a *api) csw(w http.ResponseWriter, r *http.Request) {
	req := r.URL.Query().Get("request")
	if req == "" && r.Method == http.MethodPost {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		req = cswRootOp(body)
	}
	switch strings.ToLower(req) {
	case "getcapabilities":
		a.cswCapabilities(w, r)
	case "describerecord":
		a.cswDescribeRecord(w)
	case "getrecords":
		a.cswGetRecords(w, r)
	case "getrecordbyid":
		a.cswGetRecordById(w, r)
	default:
		cswException(w, "OperationNotSupported", "request", "unknown CSW request")
	}
}

// cswRootOp guesses the operation from a POST body's root element.
func cswRootOp(body []byte) string {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	for {
		t, err := dec.Token()
		if err != nil {
			return ""
		}
		if se, ok := t.(xml.StartElement); ok {
			return se.Name.Local
		}
	}
}

func cswXML(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml")
	io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+body)
}

func cswException(w http.ResponseWriter, code, locator, msg string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, `<?xml version="1.0"?><ows:ExceptionReport xmlns:ows="http://www.opengis.net/ows" version="1.2.0">`+
		`<ows:Exception exceptionCode="%s" locator="%s"><ows:ExceptionText>%s</ows:ExceptionText></ows:Exception></ows:ExceptionReport>`,
		code, locator, escapeXML(msg))
}

func escapeXML(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (a *api) cswCapabilities(w http.ResponseWriter, r *http.Request) {
	base := r.URL.Path
	ops := func(name string) string {
		return fmt.Sprintf(`<ows:Operation name="%s"><ows:DCP><ows:HTTP>`+
			`<ows:Get xlink:href="%s?"/><ows:Post xlink:href="%s"/></ows:HTTP></ows:DCP></ows:Operation>`, name, base, base)
	}
	cswXML(w, `<csw:Capabilities xmlns:csw="http://www.opengis.net/cat/csw/2.0.2" `+
		`xmlns:ows="http://www.opengis.net/ows" xmlns:xlink="http://www.w3.org/1999/xlink" version="2.0.2">`+
		`<ows:ServiceIdentification><ows:Title>Giti CSW</ows:Title>`+
		`<ows:ServiceType>CSW</ows:ServiceType><ows:ServiceTypeVersion>2.0.2</ows:ServiceTypeVersion></ows:ServiceIdentification>`+
		`<ows:OperationsMetadata>`+
		ops("GetCapabilities")+ops("DescribeRecord")+ops("GetRecords")+ops("GetRecordById")+
		`</ows:OperationsMetadata></csw:Capabilities>`)
}

func (a *api) cswDescribeRecord(w http.ResponseWriter) {
	cswXML(w, `<csw:DescribeRecordResponse xmlns:csw="http://www.opengis.net/cat/csw/2.0.2">`+
		`<csw:SchemaComponent targetNamespace="http://purl.org/dc/elements/1.1/" schemaLanguage="http://www.w3.org/XML/Schema"/>`+
		`</csw:DescribeRecordResponse>`)
}

// record renders one Dublin Core summary record.
func cswRecord(id, title, rtype string) string {
	return fmt.Sprintf(`<csw:SummaryRecord xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dct="http://purl.org/dc/terms/">`+
		`<dc:identifier>%s</dc:identifier><dc:title>%s</dc:title><dc:type>%s</dc:type></csw:SummaryRecord>`,
		escapeXML(id), escapeXML(title), escapeXML(rtype))
}

func (a *api) cswGetRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	start, _ := strconv.Atoi(q.Get("startPosition"))
	if start < 1 {
		start = 1
	}
	maxRec := 10
	if m, err := strconv.Atoi(q.Get("maxRecords")); err == nil && m > 0 {
		maxRec = m
	}
	hitsOnly := strings.EqualFold(q.Get("resultType"), "hits")

	layers, err := a.s.ListLayers(r.Context())
	if err != nil {
		cswException(w, "NoApplicableCode", "", err.Error())
		return
	}
	// CONSTRAINT (CQL_TEXT) → keyword substring filter on the record id/title.
	kw := cswConstraintKeyword(q.Get("constraint"))
	filtered := layers[:0:0]
	for _, l := range layers {
		if kw == "" || strings.Contains(strings.ToLower(l.Workspace+":"+l.Name), kw) {
			filtered = append(filtered, l)
		}
	}
	layers = filtered
	total := len(layers)
	var recs strings.Builder
	returned := 0
	if !hitsOnly {
		for i := start - 1; i < total && returned < maxRec; i++ {
			l := layers[i]
			id := l.Workspace + ":" + l.Name
			recs.WriteString(cswRecord(id, id, "dataset"))
			returned++
		}
	}
	next := start + returned
	if next > total {
		next = 0
	}
	cswXML(w, fmt.Sprintf(`<csw:GetRecordsResponse xmlns:csw="http://www.opengis.net/cat/csw/2.0.2">`+
		`<csw:SearchStatus timestamp="%s"/>`+
		`<csw:SearchResults numberOfRecordsMatched="%d" numberOfRecordsReturned="%d" elementSet="summary" nextRecord="%d">`+
		`%s</csw:SearchResults></csw:GetRecordsResponse>`,
		time.Now().UTC().Format(time.RFC3339), total, returned, next, recs.String()))
}

// cswConstraintKeyword extracts a lower-cased search substring from a CSW
// CQL_TEXT constraint like `AnyText LIKE '%iran%'` or `dc:title = 'iran'`.
func cswConstraintKeyword(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	// prefer a quoted literal
	if i := strings.IndexByte(c, '\''); i >= 0 {
		if j := strings.IndexByte(c[i+1:], '\''); j >= 0 {
			return strings.ToLower(strings.Trim(c[i+1:i+1+j], "%"))
		}
	}
	return strings.ToLower(strings.Trim(c, "%"))
}

func (a *api) cswGetRecordById(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		cswException(w, "MissingParameterValue", "id", "id required")
		return
	}
	layers, err := a.s.ListLayers(r.Context())
	if err != nil {
		cswException(w, "NoApplicableCode", "", err.Error())
		return
	}
	var body strings.Builder
	for _, l := range layers {
		lid := l.Workspace + ":" + l.Name
		if strings.EqualFold(lid, id) {
			body.WriteString(cswRecord(lid, lid, "dataset"))
		}
	}
	cswXML(w, `<csw:GetRecordByIdResponse xmlns:csw="http://www.opengis.net/cat/csw/2.0.2">`+
		body.String()+`</csw:GetRecordByIdResponse>`)
}
