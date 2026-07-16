package ows

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

var nsService = map[string]string{
	"http://www.opengis.net/wfs":       "WFS",
	"http://www.opengis.net/wfs/2.0":   "WFS",
	"http://www.opengis.net/wms":       "WMS",
	"http://www.opengis.net/wps/1.0.0": "WPS",
	"http://www.opengis.net/wmts/1.0":  "WMTS",
}

// ParseXML extracts service/version/operation from an OWS POST XML body:
// operation = root element local name, service/version from root attributes,
// namespace sniffing as fallback for service.
func ParseXML(body io.Reader) (Request, error) {
	dec := xml.NewDecoder(io.LimitReader(body, 10<<20))
	for {
		tok, err := dec.Token()
		if err != nil {
			return Request{}, errors.New("malformed xml request: " + err.Error())
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		r := Request{Request: start.Name.Local, params: map[string][]string{}}
		for _, a := range start.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "service":
				r.Service = strings.ToUpper(a.Value)
			case "version":
				r.Version = a.Value
			}
			r.params[strings.ToUpper(a.Name.Local)] = []string{a.Value}
		}
		if r.Service == "" {
			if svc, ok := nsService[start.Name.Space]; ok {
				r.Service = svc
			}
		}
		return r, nil
	}
}
