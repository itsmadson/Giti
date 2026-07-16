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

// pgTypeToXSD maps a Postgres data_type to an xsd type.
func pgTypeToXSD(t string) string {
	switch {
	case strings.Contains(t, "int"):
		return "xsd:int"
	case strings.Contains(t, "numeric"), strings.Contains(t, "double"),
		strings.Contains(t, "real"), strings.Contains(t, "float"):
		return "xsd:double"
	case strings.Contains(t, "bool"):
		return "xsd:boolean"
	case strings.Contains(t, "timestamp"), strings.Contains(t, "date"):
		return "xsd:dateTime"
	default:
		return "xsd:string"
	}
}

func (h *handler) describeFeatureType(w http.ResponseWriter, r *http.Request, req ows.Request, version string) {
	tn := req.Get("typeNames")
	if tn == "" {
		tn = req.Get("typeName")
	}
	ws, name := splitTypeName(tn, r.Header.Get("X-Geoson-Workspace"))
	layer, err := h.m.Resolve(r.Context(), ws, name)
	if err != nil {
		writeException(w, version, ows.CodeInvalidParameterValue, "typeNames", err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "text/xml")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" ` +
		`xmlns:gml="http://www.opengis.net/gml" xmlns:geoson="geoson" ` +
		`targetNamespace="geoson" elementFormDefault="qualified">` + "\n")
	b.WriteString(`  <xsd:complexType name="` + layer.Table + `Type">` + "\n")
	b.WriteString(`    <xsd:complexContent><xsd:extension base="gml:AbstractFeatureType"><xsd:sequence>` + "\n")
	for _, c := range layer.Columns {
		fmt.Fprintf(&b, `      <xsd:element name=%q type=%q minOccurs="0"/>`+"\n",
			c.Name, pgTypeToXSD(c.Type))
	}
	fmt.Fprintf(&b, `      <xsd:element name=%q type="gml:GeometryPropertyType" minOccurs="0"/>`+"\n",
		layer.GeomCol)
	b.WriteString(`    </xsd:sequence></xsd:extension></xsd:complexContent>` + "\n")
	b.WriteString(`  </xsd:complexType>` + "\n")
	fmt.Fprintf(&b, `  <xsd:element name=%q type="geoson:%sType" substitutionGroup="gml:_Feature"/>`+"\n",
		layer.Table, layer.Table)
	b.WriteString(`</xsd:schema>` + "\n")
	w.Write([]byte(b.String()))
}
