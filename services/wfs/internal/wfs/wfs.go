// Package wfs implements the WFS protocol handlers.
package wfs

import (
	"io"
	"net/http"
	"strings"

	"github.com/giti/giti/libs/ogc-kit/ows"
	"github.com/giti/giti/services/wfs/internal/meta"
)

type handler struct {
	m *meta.Meta
}

// Mount registers the /wfs endpoint (gateway rewrites paths to /wfs).
func Mount(mux *http.ServeMux, m *meta.Meta) {
	h := &handler{m: m}
	mux.HandleFunc("/wfs", h.serve)
	mux.HandleFunc("/wfs/", h.serve)
}

func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	var req ows.Request
	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		parsed, err := ows.ParseXML(strings.NewReader(string(body)))
		if err != nil {
			writeException(w, "", ows.CodeNoApplicableCode, "", err.Error(), 400)
			return
		}
		req = parsed
		h.dispatch(w, r, req, body)
		return
	}
	req = ows.ParseKVP(r.URL.Query())
	h.dispatch(w, r, req, nil)
}

func (h *handler) dispatch(w http.ResponseWriter, r *http.Request, req ows.Request, body []byte) {
	version := req.Version
	if version == "" {
		version = r.Header.Get("X-Giti-Version")
	}
	if version == "" {
		version = "2.0.0"
	}
	switch strings.ToLower(req.Request) {
	case "getcapabilities":
		h.getCapabilities(w, r, version)
	case "describefeaturetype":
		h.describeFeatureType(w, r, req, version)
	case "getfeature", "getpropertyvalue":
		h.getFeature(w, r, req, version)
	case "transaction":
		h.transaction(w, r, body, version)
	default:
		writeException(w, version, ows.CodeOperationNotSupported, "request",
			"Operation '"+req.Request+"' not supported", 400)
	}
}

func writeException(w http.ResponseWriter, version, code, locator, msg string, status int) {
	ows.WriteException(w, "WFS", version, "", ows.ServiceError{
		Code: code, Locator: locator, Message: msg, Status: status})
}
