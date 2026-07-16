package wps

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
	"github.com/geoson/geoson/services/wps/internal/process"
)

// Mount registers the WPS endpoints.
func Mount(mux *http.ServeMux, jobs *Jobs) {
	h := &handler{jobs: jobs}
	mux.HandleFunc("/wps", h.serve)
	mux.HandleFunc("/wps/status/{id}", h.status)
}

type handler struct {
	jobs *Jobs
}

func writeException(w http.ResponseWriter, code, locator, msg string) {
	ows.WriteException(w, "WPS", "1.0.0", "", ows.ServiceError{
		Code: code, Locator: locator, Message: msg, Status: 400})
}

func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	req := ows.ParseKVP(r.URL.Query())
	switch strings.ToLower(req.Request) {
	case "getcapabilities":
		h.getCapabilities(w)
	case "describeprocess":
		h.describeProcess(w, req.Get("identifier"))
	case "execute":
		h.execute(w, r, req)
	default:
		writeException(w, ows.CodeOperationNotSupported, "request",
			"Operation '"+req.Request+"' not supported")
	}
}

func (h *handler) getCapabilities(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/xml")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<wps:Capabilities xmlns:wps="http://www.opengis.net/wps/1.0.0" ` +
		`xmlns:ows="http://www.opengis.net/ows/1.1" version="1.0.0" service="WPS">` + "\n")
	b.WriteString("<wps:ProcessOfferings>\n")
	for _, p := range process.Registry() {
		fmt.Fprintf(&b, `  <wps:Process wps:processVersion="1.0"><ows:Identifier>%s</ows:Identifier>`+
			`<ows:Title>%s</ows:Title></wps:Process>`+"\n", p.Identifier, p.Title)
	}
	b.WriteString("</wps:ProcessOfferings>\n</wps:Capabilities>\n")
	w.Write([]byte(b.String()))
}

func (h *handler) describeProcess(w http.ResponseWriter, id string) {
	p, ok := process.Get(id)
	if !ok {
		writeException(w, ows.CodeInvalidParameterValue, "identifier", "unknown process "+id)
		return
	}
	w.Header().Set("Content-Type", "text/xml")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<wps:ProcessDescriptions xmlns:wps="http://www.opengis.net/wps/1.0.0" ` +
		`xmlns:ows="http://www.opengis.net/ows/1.1" version="1.0.0">` + "\n")
	fmt.Fprintf(&b, `<ProcessDescription><ows:Identifier>%s</ows:Identifier>`+
		`<ows:Title>%s</ows:Title><ows:Abstract>%s</ows:Abstract>`+"\n",
		p.Identifier, p.Title, p.Abstract)
	b.WriteString("<DataInputs>\n")
	for _, in := range p.Inputs {
		fmt.Fprintf(&b, `  <Input><ows:Identifier>%s</ows:Identifier><ows:Title>%s</ows:Title></Input>`+"\n",
			in.Name, in.Title)
	}
	b.WriteString("</DataInputs>\n<ProcessOutputs>\n")
	for _, out := range p.Outputs {
		fmt.Fprintf(&b, `  <Output><ows:Identifier>%s</ows:Identifier><ows:Title>%s</ows:Title></Output>`+"\n",
			out.Name, out.Title)
	}
	b.WriteString("</ProcessOutputs>\n</ProcessDescription>\n</wps:ProcessDescriptions>\n")
	w.Write([]byte(b.String()))
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, err := h.jobs.Status(id)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id":%q,"process":%q,"status":%q,"output":%q,"error":%q}`,
		st.ID, st.Process, st.Status, st.Output, st.Error)
}
