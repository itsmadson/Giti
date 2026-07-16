package wfs

import (
	"context"
	"net/http"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
	"github.com/geoson/geoson/services/wfs/internal/meta"
)

// streamCSV and streamGML are implemented in Task 6.
func (h *handler) streamCSV(w http.ResponseWriter, ctx context.Context, p *gfParams,
	cols []meta.Column, where string, args []any) error {
	return nil
}

func (h *handler) streamGML(w http.ResponseWriter, ctx context.Context, p *gfParams,
	cols []meta.Column, where string, args []any, matched int, version string, gmlVer int) error {
	return nil
}

// transaction is implemented in Task 8.
func (h *handler) transaction(w http.ResponseWriter, r *http.Request, body []byte, version string) {
	writeException(w, version, ows.CodeOperationNotSupported, "request",
		"Transaction pending", 501)
}
