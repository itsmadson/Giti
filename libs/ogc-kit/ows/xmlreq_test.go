package ows

import (
	"strings"
	"testing"
)

func TestParseXMLWFSGetFeature(t *testing.T) {
	body := `<?xml version="1.0"?>
<GetFeature service="WFS" version="2.0.0" xmlns="http://www.opengis.net/wfs/2.0">
  <Query typeNames="topp:roads"/>
</GetFeature>`
	r, err := ParseXML(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if r.Service != "WFS" || r.Version != "2.0.0" || r.Request != "GetFeature" {
		t.Fatalf("parsed = %+v", r)
	}
}

func TestParseXMLNamespaceFallback(t *testing.T) {
	body := `<GetFeature xmlns="http://www.opengis.net/wfs"/>`
	r, err := ParseXML(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if r.Service != "WFS" || r.Request != "GetFeature" {
		t.Fatalf("parsed = %+v", r)
	}
}

func TestParseXMLMalformed(t *testing.T) {
	if _, err := ParseXML(strings.NewReader("<oops")); err == nil {
		t.Fatal("want error")
	}
}
