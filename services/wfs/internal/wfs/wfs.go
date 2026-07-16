// Package wfs implements the WFS protocol handlers.
package wfs

import (
	"net/http"

	"github.com/geoson/geoson/services/wfs/internal/meta"
)

type handler struct {
	m *meta.Meta
}

// Mount registers the /wfs endpoint (gateway rewrites paths to /wfs).
func Mount(mux *http.ServeMux, m *meta.Meta) {
	h := &handler{m: m}
	mux.HandleFunc("/wfs", h.serve)
}

func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	// Filled in by Task 5 (dispatch on REQUEST).
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
