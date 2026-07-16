# Sprint 3 — Gateway (OWS Front Door) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gateway parses OWS requests (KVP + POST XML, case-insensitive), negotiates service/version/request, renders GeoServer-exact exception formats, resolves virtual-service URLs, and proxies to internal services with metrics + rate limiting.

**Architecture:** Reusable OWS logic lives in `libs/ogc-kit/ows` (request parsing, negotiation rules, exception rendering) so wms/wfs/tiles reuse it later. `services/gateway` adds URL/virtual-service resolution, a dispatch table (service → backend URL from env), reverse proxy with `X-Giti-Workspace`/`X-Giti-Layer` context headers, Prometheus `/metrics`, and per-IP token-bucket rate limiting.

**Tech Stack:** Go 1.26, stdlib `net/http` + `httputil.ReverseProxy`, `prometheus/client_golang`, `golang.org/x/time/rate`.

## Global Constraints

- Health convention (Sprint 1): `/healthz`, `/readyz`, `GITI_HTTP_ADDR`; graceful SIGTERM drain via `health.Serve`.
- Go tests: `go test github.com/giti/giti/...`; GOPROXY workaround `https://goproxy.cn` already in go env + compose .env.
- Drop-in compat: KVP keys case-insensitive (`SERVICE`, `service`, `Service` identical); exception XML must match GeoServer shapes byte-structure (element names, namespaces, content types).
- Supported (routed) services this sprint: WMS → `GITI_WMS_URL`, WFS → `GITI_WFS_URL`, WMTS → `GITI_TILES_URL`, WPS → `GITI_WPS_URL`. Unset backend → OWS exception `NoApplicableCode` / "Service unavailable", HTTP 503.
- Commit after every task, Conventional Commits.

## File Structure

```
libs/ogc-kit/ows/
  request.go        # Request type, ParseKVP, case-insensitive param access
  xmlreq.go         # ParseXML: POST body → Request (root element + attrs)
  negotiate.go      # Negotiate(service, requested version) per OGC rules
  exception.go      # WriteException: WMS 1.1.1/1.3.0, OWS 1.0/1.1, JSON
  *_test.go
services/gateway/
  main.go           # wiring: env, mux, middleware chain
  dispatch.go       # virtual-service URL parsing + proxy dispatch
  middleware.go     # prometheus metrics + per-IP rate limit
  dispatch_test.go  main_test.go
```

---

### Task 1: ows.Request + KVP parsing

**Files:**
- Create: `libs/ogc-kit/ows/request.go`, `libs/ogc-kit/ows/request_test.go`

**Interfaces:**
- Produces (package `ows`):

```go
type Request struct {
	Service string // upper-cased: WMS, WFS, WMTS, WPS ("" if absent)
	Version string // as given, e.g. "1.3.0"
	Request string // operation, e.g. "GetMap" (original case preserved)
	params  map[string][]string // upper-cased keys
}
func ParseKVP(q url.Values) Request
func (r Request) Get(key string) string        // case-insensitive, first value
func (r Request) Has(key string) bool
```

- [x] **Step 1: Failing test**

`libs/ogc-kit/ows/request_test.go`:

```go
package ows

import (
	"net/url"
	"testing"
)

func TestParseKVPCaseInsensitive(t *testing.T) {
	q, _ := url.ParseQuery("service=wms&VeRsIoN=1.3.0&ReQuEsT=GetMap&LaYeRs=topp:roads&CQL_FILTER=name%3D%27x%27")
	r := ParseKVP(q)
	if r.Service != "WMS" || r.Version != "1.3.0" || r.Request != "GetMap" {
		t.Fatalf("parsed = %+v", r)
	}
	if r.Get("layers") != "topp:roads" || r.Get("LAYERS") != "topp:roads" {
		t.Fatalf("Get layers = %q", r.Get("layers"))
	}
	if r.Get("cql_filter") != "name='x'" {
		t.Fatalf("cql = %q", r.Get("cql_filter"))
	}
	if r.Has("missing") {
		t.Fatal("Has(missing) = true")
	}
}
```

- [x] **Step 2: Run** `cd libs/ogc-kit && go test ./ows/` → FAIL `undefined: ParseKVP`

- [x] **Step 3: Implement request.go**

```go
// Package ows implements shared OGC Web Service request handling:
// KVP/XML parsing, version negotiation, and exception rendering.
package ows

import (
	"net/url"
	"strings"
)

type Request struct {
	Service string
	Version string
	Request string
	params  map[string][]string
}

// ParseKVP parses OGC KVP params. Keys are case-insensitive per OGC 06-121r9.
func ParseKVP(q url.Values) Request {
	params := make(map[string][]string, len(q))
	for k, v := range q {
		params[strings.ToUpper(k)] = v
	}
	r := Request{params: params}
	r.Service = strings.ToUpper(first(params, "SERVICE"))
	r.Version = first(params, "VERSION")
	r.Request = first(params, "REQUEST")
	return r
}

func first(m map[string][]string, k string) string {
	if v := m[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (r Request) Get(key string) string { return first(r.params, strings.ToUpper(key)) }
func (r Request) Has(key string) bool {
	_, ok := r.params[strings.ToUpper(key)]
	return ok
}
```

- [x] **Step 4: Run** → PASS
- [x] **Step 5: Commit** `git add libs/ogc-kit/ows && git commit -m "feat(ogc-kit): ows request type with case-insensitive KVP"`

---

### Task 2: Version negotiation

**Files:**
- Create: `libs/ogc-kit/ows/negotiate.go`, `libs/ogc-kit/ows/negotiate_test.go`

**Interfaces:**
- Produces:

```go
// Supported versions, newest first.
var Versions = map[string][]string{
	"WMS": {"1.3.0", "1.1.1"}, "WFS": {"2.0.0", "1.1.0", "1.0.0"},
	"WMTS": {"1.0.0"}, "WPS": {"1.0.0"},
}
// Negotiate returns the version to use per OGC rules:
// exact match; else highest supported < requested; else lowest supported.
// Empty requested -> highest supported. Unknown service -> "".
func Negotiate(service, requested string) string
```

- [x] **Step 1: Failing test**

```go
package ows

import "testing"

func TestNegotiate(t *testing.T) {
	cases := []struct{ svc, req, want string }{
		{"WMS", "1.3.0", "1.3.0"},
		{"WMS", "1.1.1", "1.1.1"},
		{"WMS", "", "1.3.0"},          // none -> highest
		{"WMS", "9.9.9", "1.3.0"},     // above all -> highest below
		{"WMS", "1.2.0", "1.1.1"},     // between -> highest below
		{"WMS", "0.9.0", "1.1.1"},     // below all -> lowest
		{"WFS", "1.1.0", "1.1.0"},
		{"WFS", "2.0.0", "2.0.0"},
		{"NOPE", "1.0.0", ""},
	}
	for _, c := range cases {
		if got := Negotiate(c.svc, c.req); got != c.want {
			t.Errorf("Negotiate(%s,%s) = %s, want %s", c.svc, c.req, got, c.want)
		}
	}
}
```

- [x] **Step 2: Run** → FAIL
- [x] **Step 3: Implement negotiate.go**

```go
package ows

import "strings"

var Versions = map[string][]string{
	"WMS":  {"1.3.0", "1.1.1"},
	"WFS":  {"2.0.0", "1.1.0", "1.0.0"},
	"WMTS": {"1.0.0"},
	"WPS":  {"1.0.0"},
}

// cmpVer compares dotted numeric versions: -1, 0, 1.
func cmpVer(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			for _, ch := range as[i] {
				av = av*10 + int(ch-'0')
			}
		}
		if i < len(bs) {
			for _, ch := range bs[i] {
				bv = bv*10 + int(ch-'0')
			}
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func Negotiate(service, requested string) string {
	supported, ok := Versions[strings.ToUpper(service)]
	if !ok {
		return ""
	}
	if requested == "" {
		return supported[0]
	}
	// exact, else highest below requested (list is newest-first)
	for _, v := range supported {
		if cmpVer(v, requested) <= 0 {
			return v
		}
	}
	return supported[len(supported)-1] // below all -> lowest
}
```

- [x] **Step 4: Run** → PASS
- [x] **Step 5: Commit** `git commit -m "feat(ogc-kit): ows version negotiation"`

---

### Task 3: Exception rendering (GeoServer-exact formats)

**Files:**
- Create: `libs/ogc-kit/ows/exception.go`, `libs/ogc-kit/ows/exception_test.go`

**Interfaces:**
- Produces:

```go
type ServiceError struct {
	Code    string // e.g. InvalidParameterValue, OperationNotSupported, NoApplicableCode
	Locator string // offending param name, may be ""
	Message string
	Status  int    // HTTP status; GeoServer uses 200 for WMS 1.1.1/1.3.0 XML, real codes for OWS reports
}
// WriteException renders err in the correct format for (service, version, exceptionsParam).
// exceptionsParam = EXCEPTIONS KVP value ("application/json" -> JSON) or "".
func WriteException(w http.ResponseWriter, service, version, exceptionsParam string, err ServiceError)
```

Formats (byte structure matched to GeoServer):
- WMS 1.1.1 (or WMS w/ unknown version): content-type `application/vnd.ogc.se_xml`, HTTP 200:
```xml
<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<!DOCTYPE ServiceExceptionReport SYSTEM "http://schemas.opengis.net/wms/1.1.1/WMS_exception_1_1_1.dtd">
<ServiceExceptionReport version="1.1.1">
  <ServiceException code="InvalidParameterValue" locator="layers">msg</ServiceException>
</ServiceExceptionReport>
```
- WMS 1.3.0: content-type `text/xml`, HTTP 200:
```xml
<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<ServiceExceptionReport version="1.3.0" xmlns="http://www.opengis.net/ogc" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/ogc http://schemas.opengis.net/wms/1.3.0/exceptions_1_3_0.xsd">
  <ServiceException code="..." locator="...">msg</ServiceException>
</ServiceExceptionReport>
```
- WFS 1.0.0: OGC ServiceExceptionReport version 1.2.0 xmlns `http://www.opengis.net/ogc`, HTTP 200, `text/xml`.
- WFS 1.1.0 / default OWS: `ows:ExceptionReport` xmlns `http://www.opengis.net/ows` version 1.0.0, HTTP status from err.Status (default 400), `application/xml`.
- WFS 2.0.0 / WPS / WMTS: `ows:ExceptionReport` xmlns `http://www.opengis.net/ows/1.1` version 2.0.0 (WFS) / 1.1.0 (WPS, WMTS), `application/xml`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<ows:ExceptionReport xmlns:ows="http://www.opengis.net/ows/1.1" version="2.0.0">
  <ows:Exception exceptionCode="..." locator="...">
    <ows:ExceptionText>msg</ows:ExceptionText>
  </ows:Exception>
</ows:ExceptionReport>
```
- `exceptionsParam` contains `json`: `application/json`, GeoServer shape:
```json
{"version":"1.3.0","exceptions":[{"code":"...","locator":"...","text":"msg"}]}
```

- [x] **Step 1: Failing tests** — one per format, asserting content-type, HTTP status, and key substrings:

```go
package ows

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWMS111Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WMS", "1.1.1", "", ServiceError{Code: "InvalidParameterValue", Locator: "layers", Message: "no such layer"})
	body := rec.Body.String()
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/vnd.ogc.se_xml" {
		t.Fatalf("code=%d ct=%s", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		`<!DOCTYPE ServiceExceptionReport SYSTEM "http://schemas.opengis.net/wms/1.1.1/WMS_exception_1_1_1.dtd">`,
		`<ServiceExceptionReport version="1.1.1">`,
		`code="InvalidParameterValue"`, `locator="layers"`, `no such layer`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestWMS130Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WMS", "1.3.0", "", ServiceError{Code: "LayerNotDefined", Message: "x"})
	body := rec.Body.String()
	if rec.Header().Get("Content-Type") != "text/xml" {
		t.Fatalf("ct=%s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, `xmlns="http://www.opengis.net/ogc"`) ||
		!strings.Contains(body, `version="1.3.0"`) {
		t.Fatalf("body=%s", body)
	}
}

func TestWFS20Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WFS", "2.0.0", "", ServiceError{Code: "OperationNotSupported", Locator: "request", Message: "nope", Status: 400})
	body := rec.Body.String()
	if rec.Code != 400 || rec.Header().Get("Content-Type") != "application/xml" {
		t.Fatalf("code=%d ct=%s", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		`xmlns:ows="http://www.opengis.net/ows/1.1"`, `version="2.0.0"`,
		`exceptionCode="OperationNotSupported"`, `<ows:ExceptionText>nope</ows:ExceptionText>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestWFS10Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WFS", "1.0.0", "", ServiceError{Code: "MissingParameterValue", Message: "typename"})
	if !strings.Contains(rec.Body.String(), `version="1.2.0"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestJSONException(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WMS", "1.3.0", "application/json", ServiceError{Code: "NoApplicableCode", Message: "boom"})
	body := rec.Body.String()
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("ct=%s", rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{`"version":"1.3.0"`, `"code":"NoApplicableCode"`, `"text":"boom"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}
```

- [x] **Step 2: Run** → FAIL
- [x] **Step 3: Implement exception.go**

```go
package ows

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
)

type ServiceError struct {
	Code    string
	Locator string
	Message string
	Status  int
}

// Standard OWS exception codes.
const (
	CodeInvalidParameterValue = "InvalidParameterValue"
	CodeMissingParameterValue = "MissingParameterValue"
	CodeOperationNotSupported = "OperationNotSupported"
	CodeNoApplicableCode      = "NoApplicableCode"
	CodeInvalidUpdateSequence = "InvalidUpdateSequence"
)

func WriteException(w http.ResponseWriter, service, version, exceptionsParam string, err ServiceError) {
	service = strings.ToUpper(service)
	if strings.Contains(strings.ToLower(exceptionsParam), "json") {
		writeJSONException(w, version, err)
		return
	}
	switch {
	case service == "WMS" && version == "1.3.0":
		writeWMS130(w, err)
	case service == "WMS":
		writeWMS111(w, err)
	case service == "WFS" && version == "1.0.0":
		writeOGC120(w, err)
	case service == "WFS" && strings.HasPrefix(version, "2."):
		writeOWS11(w, "2.0.0", err)
	case service == "WFS": // 1.1.0
		writeOWS10(w, err)
	default: // WMTS, WPS, unknown
		writeOWS11(w, "1.1.0", err)
	}
}

func locAttr(l string) string {
	if l == "" {
		return ""
	}
	return fmt.Sprintf(" locator=%q", l)
}

func codeAttr(c string) string {
	if c == "" {
		return ""
	}
	return fmt.Sprintf(" code=%q", c)
}

func writeWMS111(w http.ResponseWriter, e ServiceError) {
	w.Header().Set("Content-Type", "application/vnd.ogc.se_xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<!DOCTYPE ServiceExceptionReport SYSTEM "http://schemas.opengis.net/wms/1.1.1/WMS_exception_1_1_1.dtd">
<ServiceExceptionReport version="1.1.1">
  <ServiceException%s%s>%s</ServiceException>
</ServiceExceptionReport>
`, codeAttr(e.Code), locAttr(e.Locator), xmlEscape(e.Message))
}

func writeWMS130(w http.ResponseWriter, e ServiceError) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<ServiceExceptionReport version="1.3.0" xmlns="http://www.opengis.net/ogc" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/ogc http://schemas.opengis.net/wms/1.3.0/exceptions_1_3_0.xsd">
  <ServiceException%s%s>%s</ServiceException>
</ServiceExceptionReport>
`, codeAttr(e.Code), locAttr(e.Locator), xmlEscape(e.Message))
}

func writeOGC120(w http.ResponseWriter, e ServiceError) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<ServiceExceptionReport version="1.2.0" xmlns="http://www.opengis.net/ogc" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/ogc http://schemas.opengis.net/wfs/1.0.0/OGC-exception.xsd">
  <ServiceException%s%s>%s</ServiceException>
</ServiceExceptionReport>
`, codeAttr(e.Code), locAttr(e.Locator), xmlEscape(e.Message))
}

func writeOWS10(w http.ResponseWriter, e ServiceError) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status(e))
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<ows:ExceptionReport xmlns:ows="http://www.opengis.net/ows" version="1.0.0">
  <ows:Exception exceptionCode=%q%s>
    <ows:ExceptionText>%s</ows:ExceptionText>
  </ows:Exception>
</ows:ExceptionReport>
`, e.Code, locAttr(e.Locator), xmlEscape(e.Message))
}

func writeOWS11(w http.ResponseWriter, version string, e ServiceError) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status(e))
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<ows:ExceptionReport xmlns:ows="http://www.opengis.net/ows/1.1" version=%q>
  <ows:Exception exceptionCode=%q%s>
    <ows:ExceptionText>%s</ows:ExceptionText>
  </ows:Exception>
</ows:ExceptionReport>
`, version, e.Code, locAttr(e.Locator), xmlEscape(e.Message))
}

func writeJSONException(w http.ResponseWriter, version string, e ServiceError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status(e))
	json.NewEncoder(w).Encode(map[string]any{
		"version": version,
		"exceptions": []map[string]string{{
			"code": e.Code, "locator": e.Locator, "text": e.Message,
		}},
	})
}

func status(e ServiceError) int {
	if e.Status != 0 {
		return e.Status
	}
	return http.StatusBadRequest
}

func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
```

- [x] **Step 4: Run** → PASS
- [x] **Step 5: Commit** `git commit -m "feat(ogc-kit): geoserver-compatible ows exception rendering"`

---

### Task 4: POST XML request parsing

**Files:**
- Create: `libs/ogc-kit/ows/xmlreq.go`, `libs/ogc-kit/ows/xmlreq_test.go`

**Interfaces:**
- Produces:

```go
// ParseXML reads an OWS POST body: operation = root element local name,
// service/version from root attributes (falling back to namespace sniffing
// for WFS/WMS namespaces). Returns error on malformed XML.
func ParseXML(body io.Reader) (Request, error)
```

- [x] **Step 1: Failing test**

```go
package ows

import (
	"strings"
	"testing"
)

func TestParseXMLWFSGetFeature(t *testing.T) {
	body := `<?xml version="1.0"?>
<GetFeature service="WFS" version="2.0.0" xmlns="http://www.opengis.net/wfs/2.0">
  <Query typeNames="topp:roads"/>
</GetFeature>`
	r, err := ParseXML(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if r.Service != "WFS" || r.Version != "2.0.0" || r.Request != "GetFeature" {
		t.Fatalf("parsed = %+v", r)
	}
}

func TestParseXMLNamespaceFallback(t *testing.T) {
	body := `<GetFeature xmlns="http://www.opengis.net/wfs"/>`
	r, err := ParseXML(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if r.Service != "WFS" || r.Request != "GetFeature" {
		t.Fatalf("parsed = %+v", r)
	}
}

func TestParseXMLMalformed(t *testing.T) {
	if _, err := ParseXML(strings.NewReader("<oops")); err == nil {
		t.Fatal("want error")
	}
}
```

- [x] **Step 2: Run** → FAIL
- [x] **Step 3: Implement xmlreq.go**

```go
package ows

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

var nsService = map[string]string{
	"http://www.opengis.net/wfs":     "WFS",
	"http://www.opengis.net/wfs/2.0": "WFS",
	"http://www.opengis.net/wms":     "WMS",
	"http://www.opengis.net/wps/1.0.0": "WPS",
	"http://www.opengis.net/wmts/1.0":  "WMTS",
}

// ParseXML extracts service/version/operation from an OWS POST XML body.
func ParseXML(body io.Reader) (Request, error) {
	dec := xml.NewDecoder(io.LimitReader(body, 10<<20))
	for {
		tok, err := dec.Token()
		if err != nil {
			return Request{}, errors.New("malformed xml request: " + err.Error())
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		r := Request{Request: start.Name.Local, params: map[string][]string{}}
		for _, a := range start.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "service":
				r.Service = strings.ToUpper(a.Value)
			case "version":
				r.Version = a.Value
			}
			r.params[strings.ToUpper(a.Name.Local)] = []string{a.Value}
		}
		if r.Service == "" {
			if svc, ok := nsService[start.Name.Space]; ok {
				r.Service = svc
			}
		}
		return r, nil
	}
}
```

- [x] **Step 4: Run** → PASS (all ows tests)
- [x] **Step 5: Commit** `git commit -m "feat(ogc-kit): ows post xml request parsing"`

---

### Task 5: Gateway dispatcher — virtual services + negotiation + proxy

**Files:**
- Create: `services/gateway/dispatch.go`, `services/gateway/dispatch_test.go`
- Modify: `services/gateway/main.go`

**Interfaces:**
- Consumes: `ows.ParseKVP`, `ows.ParseXML`, `ows.Negotiate`, `ows.WriteException`, `ows.ServiceError`, codes.
- Produces:

```go
type backends struct { // parsed from env once at boot
	byService map[string]*url.URL // "WMS" -> http://wms:8080, etc.
}
func newBackends(getenv func(string) string) backends
// newDispatcher returns the OWS handler mounted at /giti/.
// URL forms: /giti/ows, /giti/{svc}, /giti/{ws}/{svc},
//            /giti/{ws}/{layer}/{svc}  (svc in wms|wfs|ows|wps|gwc)
// Proxied request gains headers X-Giti-Workspace, X-Giti-Layer,
// X-Giti-Version (negotiated), and has /giti prefix stripped.
func newDispatcher(b backends) http.Handler
```

Dispatch rules:
1. Parse path → optional workspace, optional layer, endpoint segment.
2. GET → `ows.ParseKVP(r.URL.Query())`; POST with XML content-type → `ows.ParseXML(r.Body)` (body re-buffered for proxying).
3. Service resolution: KVP/XML SERVICE param wins; else endpoint segment implies it (`/wms` → WMS, `/wfs` → WFS, `/wps` → WPS, `/gwc/service/wmts` → WMTS); `/ows` requires SERVICE else `MissingParameterValue/service`.
4. REQUEST missing → `MissingParameterValue/request` exception in negotiated format.
5. Unknown SERVICE value → `InvalidParameterValue/service`.
6. Backend URL unset → `NoApplicableCode` "Service WMS is not available", Status 503.
7. Else reverse-proxy to backend, path `/{service lowercased}`, original query preserved, context headers set.

- [x] **Step 1: Failing tests**

`services/gateway/dispatch_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testBackends(t *testing.T, wmsURL string) backends {
	t.Helper()
	b := backends{byService: map[string]*url.URL{}}
	if wmsURL != "" {
		u, err := url.Parse(wmsURL)
		if err != nil {
			t.Fatal(err)
		}
		b.byService["WMS"] = u
	}
	return b
}

func TestDispatchProxiesToWMS(t *testing.T) {
	var got *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		w.Write([]byte("MAP"))
	}))
	defer backend.Close()

	h := newDispatcher(testBackends(t, backend.URL))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET",
		"/giti/topp/wms?service=wms&version=1.1.1&request=GetMap&layers=roads", nil))
	if rec.Code != 200 || rec.Body.String() != "MAP" {
		t.Fatalf("proxy = %d %s", rec.Code, rec.Body.String())
	}
	if got.Header.Get("X-Giti-Workspace") != "topp" {
		t.Fatalf("ws header = %q", got.Header.Get("X-Giti-Workspace"))
	}
	if got.Header.Get("X-Giti-Version") != "1.1.1" {
		t.Fatalf("version header = %q", got.Header.Get("X-Giti-Version"))
	}
	if got.URL.Query().Get("layers") != "roads" {
		t.Fatalf("query lost: %s", got.URL.RawQuery)
	}
}

func TestDispatchLayerVirtualService(t *testing.T) {
	var got *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
	}))
	defer backend.Close()
	h := newDispatcher(testBackends(t, backend.URL))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET",
		"/giti/topp/roads/wms?request=GetMap", nil))
	if got.Header.Get("X-Giti-Layer") != "roads" {
		t.Fatalf("layer header = %q", got.Header.Get("X-Giti-Layer"))
	}
}

func TestDispatchEndpointImpliesService(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer backend.Close()
	h := newDispatcher(testBackends(t, backend.URL))
	rec := httptest.NewRecorder()
	// no SERVICE param — /wms endpoint implies WMS
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/giti/wms?request=GetCapabilities", nil))
	if rec.Code != 200 {
		t.Fatalf("implied service = %d %s", rec.Code, rec.Body.String())
	}
}

func TestDispatchMissingRequestParam(t *testing.T) {
	h := newDispatcher(testBackends(t, "http://wms:8080"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/giti/wms?service=WMS&version=1.1.1", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "ServiceExceptionReport") || !strings.Contains(body, "request") {
		t.Fatalf("body = %s", body)
	}
}

func TestDispatchOWSRequiresService(t *testing.T) {
	h := newDispatcher(testBackends(t, "http://wms:8080"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/giti/ows?request=GetCapabilities", nil))
	if !strings.Contains(rec.Body.String(), "service") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestDispatchUnavailableBackend(t *testing.T) {
	h := newDispatcher(testBackends(t, "")) // no WMS backend
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET",
		"/giti/wms?service=WMS&request=GetCapabilities", nil))
	if !strings.Contains(rec.Body.String(), "not available") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestDispatchPostXML(t *testing.T) {
	var got *http.Request
	var gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
	}))
	defer backend.Close()
	b := testBackends(t, "")
	u, _ := url.Parse(backend.URL)
	b.byService["WFS"] = u
	h := newDispatcher(b)
	xmlBody := `<GetFeature service="WFS" version="2.0.0" xmlns="http://www.opengis.net/wfs/2.0"/>`
	req := httptest.NewRequest("POST", "/giti/wfs", strings.NewReader(xmlBody))
	req.Header.Set("Content-Type", "application/xml")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.Header.Get("X-Giti-Version") != "2.0.0" {
		t.Fatalf("version = %q", got.Header.Get("X-Giti-Version"))
	}
	if !strings.Contains(gotBody, "GetFeature") {
		t.Fatalf("body not forwarded: %q", gotBody)
	}
}
```

- [x] **Step 2: Run** `cd services/gateway && go test ./...` → FAIL `undefined: backends`
- [x] **Step 3: Implement dispatch.go**

```go
package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/giti/giti/libs/ogc-kit/ows"
)

type backends struct {
	byService map[string]*url.URL
}

func newBackends(getenv func(string) string) backends {
	b := backends{byService: map[string]*url.URL{}}
	for svc, env := range map[string]string{
		"WMS": "GITI_WMS_URL", "WFS": "GITI_WFS_URL",
		"WMTS": "GITI_TILES_URL", "WPS": "GITI_WPS_URL",
	} {
		if v := getenv(env); v != "" {
			if u, err := url.Parse(v); err == nil {
				b.byService[svc] = u
			}
		}
	}
	return b
}

// endpointService maps URL endpoint segments to implied services.
var endpointService = map[string]string{
	"wms": "WMS", "wfs": "WFS", "wps": "WPS", "wmts": "WMTS", "gwc": "WMTS",
}

// parsePath splits /giti/[{ws}/[{layer}/]]{endpoint} into parts.
func parsePath(path string) (ws, layer, endpoint string) {
	path = strings.TrimPrefix(path, "/giti")
	path = strings.Trim(path, "/")
	segs := strings.Split(path, "/")
	// gwc/service/wmts collapses to endpoint "gwc"
	for i, s := range segs {
		if _, ok := endpointService[strings.ToLower(s)]; ok || strings.ToLower(s) == "ows" {
			endpoint = strings.ToLower(s)
			segs = segs[:i]
			break
		}
	}
	if len(segs) > 0 {
		ws = segs[0]
	}
	if len(segs) > 1 {
		layer = segs[1]
	}
	return ws, layer, endpoint
}

func newDispatcher(b backends) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsName, layer, endpoint := parsePath(r.URL.Path)

		var req ows.Request
		var bodyCopy []byte
		if r.Method == http.MethodPost {
			bodyCopy, _ = io.ReadAll(io.LimitReader(r.Body, 64<<20))
			var err error
			req, err = ows.ParseXML(bytes.NewReader(bodyCopy))
			if err != nil {
				ows.WriteException(w, "", "", "", ows.ServiceError{
					Code: ows.CodeNoApplicableCode, Message: err.Error(), Status: 400})
				return
			}
		} else {
			req = ows.ParseKVP(r.URL.Query())
		}

		service := req.Service
		if service == "" {
			if implied, ok := endpointService[endpoint]; ok {
				service = implied
			}
		}
		exceptions := req.Get("EXCEPTIONS")
		if service == "" {
			ows.WriteException(w, "", "", exceptions, ows.ServiceError{
				Code: ows.CodeMissingParameterValue, Locator: "service",
				Message: "Could not determine service", Status: 400})
			return
		}
		version := ows.Negotiate(service, req.Version)
		if version == "" {
			ows.WriteException(w, "", "", exceptions, ows.ServiceError{
				Code: ows.CodeInvalidParameterValue, Locator: "service",
				Message: "No service: ( " + strings.ToLower(service) + " )", Status: 400})
			return
		}
		if req.Request == "" {
			ows.WriteException(w, service, version, exceptions, ows.ServiceError{
				Code: ows.CodeMissingParameterValue, Locator: "request",
				Message: "Could not determine request", Status: 400})
			return
		}
		backend, ok := b.byService[service]
		if !ok {
			ows.WriteException(w, service, version, exceptions, ows.ServiceError{
				Code: ows.CodeNoApplicableCode,
				Message: "Service " + service + " is not available", Status: 503})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(backend)
		r.Header.Set("X-Giti-Workspace", wsName)
		r.Header.Set("X-Giti-Layer", layer)
		r.Header.Set("X-Giti-Version", version)
		r.URL.Path = "/" + strings.ToLower(service)
		if bodyCopy != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyCopy))
			r.ContentLength = int64(len(bodyCopy))
		}
		proxy.ServeHTTP(w, r)
	})
}
```

- [x] **Step 4: Wire into main.go** — replace `newHandler`:

```go
func newHandler() http.Handler {
	return newHandlerWith(newBackends(os.Getenv))
}

func newHandlerWith(b backends) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/readyz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/giti/", newDispatcher(b))
	return mux
}
```

(keep existing `main()`; add `"os"` import already present)

- [x] **Step 5: Run** `go test ./...` → PASS (incl. existing healthz test)
- [x] **Step 6: Commit** `git commit -m "feat(gateway): ows dispatcher with virtual services and negotiation"`

---

### Task 6: Metrics + rate limiting middleware

**Files:**
- Create: `services/gateway/middleware.go`, `services/gateway/middleware_test.go`
- Modify: `services/gateway/main.go`, `services/gateway/go.mod`

**Interfaces:**
- Produces:

```go
// metricsMiddleware records giti_gateway_requests_total{service,code}
// and giti_gateway_request_seconds{service} histogram; /metrics endpoint.
func metricsMiddleware(next http.Handler) http.Handler
// rateLimitMiddleware: per-client-IP token bucket. Env:
// GITI_RATE_LIMIT (req/s, 0 = disabled, default 0), GITI_RATE_BURST (default 2x limit).
// Exceeded -> 429 with OWS NoApplicableCode exception.
func rateLimitMiddleware(limit float64, burst int, next http.Handler) http.Handler
```

- [x] **Step 1: Deps** `go get github.com/prometheus/client_golang@latest golang.org/x/time@latest`

- [x] **Step 2: Failing test**

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRateLimitReturns429(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	h := rateLimitMiddleware(1, 1, inner)
	req := httptest.NewRequest("GET", "/giti/wms?service=WMS", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec1.Code != 200 || rec2.Code != 429 {
		t.Fatalf("codes = %d, %d; want 200, 429", rec1.Code, rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "Too many requests") {
		t.Fatalf("body = %s", rec2.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	h := newHandlerWith(backends{byService: nil})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("metrics = %d", rec.Code)
	}
}
```

- [x] **Step 3: Implement middleware.go**

```go
package main

import (
	"net"
	"net/http"
	"sync"

	"github.com/giti/giti/libs/ogc-kit/ows"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

var (
	reqTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "giti_gateway_requests_total",
		Help: "OWS requests by service and status code.",
	}, []string{"service", "code"})
	reqSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "giti_gateway_request_seconds",
		Help: "OWS request latency.",
	}, []string{"service"})
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (s *statusWriter) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := ows.ParseKVP(r.URL.Query()).Service
		if service == "" {
			service = "unknown"
		}
		sw := &statusWriter{ResponseWriter: w, code: 200}
		timer := prometheus.NewTimer(reqSeconds.WithLabelValues(service))
		next.ServeHTTP(sw, r)
		timer.ObserveDuration()
		reqTotal.WithLabelValues(service, http.StatusText(sw.code)).Inc()
	})
}

func metricsHandler() http.Handler { return promhttp.Handler() }

func rateLimitMiddleware(limit float64, burst int, next http.Handler) http.Handler {
	if limit <= 0 {
		return next
	}
	var mu sync.Mutex
	limiters := map[string]*rate.Limiter{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		mu.Lock()
		l, ok := limiters[ip]
		if !ok {
			l = rate.NewLimiter(rate.Limit(limit), burst)
			limiters[ip] = l
		}
		mu.Unlock()
		if !l.Allow() {
			req := ows.ParseKVP(r.URL.Query())
			ows.WriteException(w, req.Service, req.Version, req.Get("EXCEPTIONS"),
				ows.ServiceError{Code: ows.CodeNoApplicableCode,
					Message: "Too many requests", Status: http.StatusTooManyRequests})
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [x] **Step 4: Wire in main.go** — `newHandlerWith` becomes:

```go
func newHandlerWith(b backends) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/readyz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/metrics", metricsHandler())
	limit, _ := strconv.ParseFloat(os.Getenv("GITI_RATE_LIMIT"), 64)
	burst, _ := strconv.Atoi(os.Getenv("GITI_RATE_BURST"))
	if burst == 0 {
		burst = int(limit * 2)
	}
	mux.Handle("/giti/",
		rateLimitMiddleware(limit, burst, metricsMiddleware(newDispatcher(b))))
	return mux
}
```

(add imports `"strconv"`)

- [x] **Step 5: Run** `go test ./...` → PASS; `go mod tidy`
- [x] **Step 6: Commit** `git commit -m "feat(gateway): prometheus metrics and per-ip rate limiting"`

---

### Task 7: Compose wiring, e2e, docs, close out

**Files:**
- Modify: `deploy/compose/docker-compose.yml` (gateway env + routing rule), `docs/architecture.md`, `task.md`
- Create: `docs/services/gateway.md`

- [x] **Step 1: Compose** — gateway service gains:

```yaml
    environment:
      GITI_WMS_URL: http://wms:8080
```

and its Traefik rule becomes (catalog keeps `/giti/rest` + `/api/v1`; router priorities make rest win):

```yaml
      - traefik.http.routers.gateway.rule=PathPrefix(`/giti`) || PathPrefix(`/healthz`) || PathPrefix(`/readyz`)
      - traefik.http.routers.gateway.priority=1
```

catalog router gains `- traefik.http.routers.catalog.priority=10`.

- [x] **Step 2: e2e**

```bash
cd deploy/compose && docker compose up -d --build gateway
sleep 5
# exception path (wms stub has no OWS yet -> proxied 404 is fine; test negotiation errors)
curl -s "http://localhost/giti/wms?service=WMS&version=1.1.1" | grep ServiceExceptionReport
curl -s "http://localhost/giti/ows?request=GetCapabilities" | grep service
curl -s "http://localhost/giti/wfs?service=WFS&version=2.0.0&request=GetFeature" | grep "not available"
curl -s http://localhost/giti/rest/workspaces.json | grep workspaces   # catalog still wins /rest
```

- [x] **Step 3: docs/services/gateway.md**

```markdown
# gateway

OWS front door. Go.

## URL forms
`/giti/ows` · `/giti/{wms|wfs|wps}` · `/giti/{ws}/{svc}` ·
`/giti/{ws}/{layer}/{svc}` · `/giti/gwc/service/wmts`

## Behavior
- KVP (case-insensitive) + POST XML parsing, OGC version negotiation
- GeoServer-exact exception formats (WMS 1.1.1 DTD report, WMS 1.3.0,
  OGC 1.2.0 for WFS 1.0, ows 1.0/1.1 reports, JSON via EXCEPTIONS=application/json)
- Proxies to backends with X-Giti-Workspace / X-Giti-Layer / X-Giti-Version headers
- `/metrics` Prometheus; per-IP rate limit via GITI_RATE_LIMIT / GITI_RATE_BURST

## Env
GITI_HTTP_ADDR, GITI_WMS_URL, GITI_WFS_URL, GITI_TILES_URL, GITI_WPS_URL,
GITI_RATE_LIMIT (req/s per IP, 0=off), GITI_RATE_BURST
```

- [x] **Step 4: architecture.md** gateway row → `done (Sprint 3) — [docs](services/gateway.md)`; `task.md` Sprint 3 → `[x]`, add plan link.

- [x] **Step 5: Final verify + commit**

```bash
go vet github.com/giti/giti/... && go test github.com/giti/giti/...
git add -A && git commit -m "feat(gateway): compose wiring, docs; complete sprint 3"
```
