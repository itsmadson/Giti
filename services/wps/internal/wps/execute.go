package wps

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
	"github.com/geoson/geoson/services/wps/internal/process"
)

// parseDataInputs parses "name=value;name=value" (WPS KVP DataInputs).
func parseDataInputs(raw string) map[string]string {
	m := map[string]string{}
	for _, pair := range strings.Split(raw, ";") {
		if pair == "" {
			continue
		}
		k, v, found := strings.Cut(pair, "=")
		if found {
			m[strings.TrimSpace(k)] = v
		}
	}
	return m
}

func (h *handler) execute(w http.ResponseWriter, r *http.Request, req ows.Request) {
	id := req.Get("identifier")
	if _, ok := process.Get(id); !ok {
		writeException(w, ows.CodeInvalidParameterValue, "identifier", "unknown process "+id)
		return
	}
	inputs := parseDataInputs(req.Get("DataInputs"))

	async := strings.EqualFold(req.Get("storeExecuteResponse"), "true") ||
		strings.EqualFold(req.Get("mode"), "async")

	if async {
		jobID, err := h.jobs.Enqueue(r.Context(), id, inputs)
		if err != nil {
			writeException(w, ows.CodeNoApplicableCode, "", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
			`<wps:ExecuteResponse xmlns:wps="http://www.opengis.net/wps/1.0.0" `+
			`xmlns:ows="http://www.opengis.net/ows/1.1" statusLocation="/wps/status/%s">`+
			`<wps:Process><ows:Identifier>%s</ows:Identifier></wps:Process>`+
			`<wps:Status>accepted</wps:Status></wps:ExecuteResponse>`, jobID, id)
		return
	}

	out, err := h.jobs.execNow(r.Context(), id, inputs)
	if err != nil {
		writeException(w, ows.CodeNoApplicableCode, "", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
		`<wps:ExecuteResponse xmlns:wps="http://www.opengis.net/wps/1.0.0" `+
		`xmlns:ows="http://www.opengis.net/ows/1.1">`+
		`<wps:Process><ows:Identifier>%s</ows:Identifier></wps:Process>`+
		`<wps:Status>succeeded</wps:Status>`+
		`<wps:ProcessOutputs><wps:Output><ows:Identifier>result</ows:Identifier>`+
		`<wps:Data><wps:LiteralData>%s</wps:LiteralData></wps:Data></wps:Output>`+
		`</wps:ProcessOutputs></wps:ExecuteResponse>`, id, xmlEscape(out))
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
