package wfs

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
)

// writeHits emits a members-less FeatureCollection reporting numberMatched.
func writeHits(w http.ResponseWriter, version string, matched int) {
	w.Header().Set("Content-Type", "text/xml")
	if strings.HasPrefix(version, "2.") {
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+
			`<wfs:FeatureCollection xmlns:wfs="http://www.opengis.net/wfs/2.0" `+
			`numberMatched="%d" numberReturned="0"/>`, matched)
		return
	}
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<wfs:FeatureCollection xmlns:wfs="http://www.opengis.net/wfs" `+
		`numberOfFeatures="%d"/>`, matched)
}

// getCapabilities and describeFeatureType are implemented in Tasks 6-7.
func (h *handler) getCapabilities(w http.ResponseWriter, r *http.Request, version string) {
	writeException(w, version, ows.CodeOperationNotSupported, "request",
		"GetCapabilities pending", 501)
}

func (h *handler) describeFeatureType(w http.ResponseWriter, r *http.Request, req ows.Request, version string) {
	writeException(w, version, ows.CodeOperationNotSupported, "request",
		"DescribeFeatureType pending", 501)
}
