package ows

import (
	"net/url"
	"testing"
)

func TestParseKVPCaseInsensitive(t *testing.T) {
	q, _ := url.ParseQuery("service=wms&VeRsIoN=1.3.0&ReQuEsT=GetMap&LaYeRs=topp:roads&CQL_FILTER=name%3D%27x%27")
	r := ParseKVP(q)
	if r.Service != "WMS" || r.Version != "1.3.0" || r.Request != "GetMap" {
		t.Fatalf("parsed = %+v", r)
	}
	if r.Get("layers") != "topp:roads" || r.Get("LAYERS") != "topp:roads" {
		t.Fatalf("Get layers = %q", r.Get("layers"))
	}
	if r.Get("cql_filter") != "name='x'" {
		t.Fatalf("cql = %q", r.Get("cql_filter"))
	}
	if r.Has("missing") {
		t.Fatal("Has(missing) = true")
	}
}
