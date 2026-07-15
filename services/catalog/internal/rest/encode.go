package rest

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
)

// wantsJSON: .json suffix (GeoServer style) or Accept: application/json.
func wantsJSON(r *http.Request) bool {
	if strings.HasSuffix(r.URL.Path, ".json") {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// trimFormat strips a trailing .json/.xml from the last path segment.
func trimFormat(name string) string {
	name = strings.TrimSuffix(name, ".json")
	return strings.TrimSuffix(name, ".xml")
}

func writePayload(w http.ResponseWriter, r *http.Request, xmlBody, jsonBody any) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonBody)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(xmlBody)
}

func readPayload(r *http.Request, xmlDst, jsonDst any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		return err
	}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "json") {
		return json.Unmarshal(body, jsonDst)
	}
	return xml.Unmarshal(body, xmlDst)
}
