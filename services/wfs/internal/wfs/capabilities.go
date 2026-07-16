package wfs

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
	"github.com/geoson/geoson/services/wfs/internal/meta"
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

func (h *handler) getCapabilities(w http.ResponseWriter, r *http.Request, version string) {
	fts, err := h.m.ListFeatureTypes(r.Context())
	if err != nil {
		writeException(w, version, ows.CodeNoApplicableCode, "", err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/xml")
	switch {
	case strings.HasPrefix(version, "1.0"):
		writeCaps10(w, fts)
	case strings.HasPrefix(version, "1.1"):
		writeCaps11(w, fts)
	default:
		writeCaps20(w, fts)
	}
}

func qualified(ws, name string) string {
	if ws == "" {
		return name
	}
	return ws + ":" + name
}

func writeCaps20(w http.ResponseWriter, fts []meta.Layer) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<wfs:WFS_Capabilities version="2.0.0" ` +
		`xmlns:wfs="http://www.opengis.net/wfs/2.0" ` +
		`xmlns:ows="http://www.opengis.net/ows/1.1">` + "\n")
	b.WriteString(`  <ows:OperationsMetadata>` +
		`<ows:Operation name="GetCapabilities"/><ows:Operation name="DescribeFeatureType"/>` +
		`<ows:Operation name="GetFeature"/><ows:Operation name="Transaction"/>` +
		`</ows:OperationsMetadata>` + "\n")
	b.WriteString("  <wfs:FeatureTypeList>\n")
	for _, ft := range fts {
		fmt.Fprintf(&b, `    <wfs:FeatureType><wfs:Name>%s</wfs:Name>`+
			`<wfs:DefaultCRS>urn:ogc:def:crs:EPSG::4326</wfs:DefaultCRS>`+
			`<ows:WGS84BoundingBox><ows:LowerCorner>-180 -90</ows:LowerCorner>`+
			`<ows:UpperCorner>180 90</ows:UpperCorner></ows:WGS84BoundingBox>`+
			`</wfs:FeatureType>`+"\n", qualified(ft.Workspace, ft.Name))
	}
	b.WriteString("  </wfs:FeatureTypeList>\n</wfs:WFS_Capabilities>\n")
	w.Write([]byte(b.String()))
}

func writeCaps11(w http.ResponseWriter, fts []meta.Layer) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<wfs:WFS_Capabilities version="1.1.0" ` +
		`xmlns:wfs="http://www.opengis.net/wfs" ` +
		`xmlns:ows="http://www.opengis.net/ows">` + "\n")
	b.WriteString("  <wfs:FeatureTypeList>\n")
	for _, ft := range fts {
		fmt.Fprintf(&b, `    <wfs:FeatureType><wfs:Name>%s</wfs:Name>`+
			`<wfs:DefaultSRS>urn:ogc:def:crs:EPSG::4326</wfs:DefaultSRS>`+
			`<ows:WGS84BoundingBox><ows:LowerCorner>-180 -90</ows:LowerCorner>`+
			`<ows:UpperCorner>180 90</ows:UpperCorner></ows:WGS84BoundingBox>`+
			`</wfs:FeatureType>`+"\n", qualified(ft.Workspace, ft.Name))
	}
	b.WriteString("  </wfs:FeatureTypeList>\n</wfs:WFS_Capabilities>\n")
	w.Write([]byte(b.String()))
}

func writeCaps10(w http.ResponseWriter, fts []meta.Layer) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<WFS_Capabilities version="1.0.0" xmlns="http://www.opengis.net/wfs">` + "\n")
	b.WriteString("  <FeatureTypeList>\n")
	for _, ft := range fts {
		fmt.Fprintf(&b, `    <FeatureType><Name>%s</Name><SRS>EPSG:4326</SRS>`+
			`<LatLongBoundingBox minx="-180" miny="-90" maxx="180" maxy="90"/>`+
			`</FeatureType>`+"\n", qualified(ft.Workspace, ft.Name))
	}
	b.WriteString("  </FeatureTypeList>\n</WFS_Capabilities>\n")
	w.Write([]byte(b.String()))
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
