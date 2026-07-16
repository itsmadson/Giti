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

// WriteException renders err in the GeoServer-compatible format for
// (service, version). exceptionsParam is the EXCEPTIONS KVP value.
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
