package wfs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/wfs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTransactionInsertUpdateDelete(t *testing.T) {
	h := testHandler(t)
	rec := post(t, h, `<Transaction service="WFS" version="1.1.0" xmlns="http://www.opengis.net/wfs"
	  xmlns:giti="giti" xmlns:gml="http://www.opengis.net/gml">
	  <Insert>
	    <giti:wfstest--wfs_roads>
	      <giti:name>new rd</giti:name>
	      <giti:lanes>8</giti:lanes>
	      <giti:geom><gml:LineString><gml:coordinates>5,5 6,6</gml:coordinates></gml:LineString></giti:geom>
	    </giti:wfstest--wfs_roads>
	  </Insert></Transaction>`)
	if !strings.Contains(rec.Body.String(), "<wfs:totalInserted>1</wfs:totalInserted>") {
		t.Fatalf("insert = %s", rec.Body.String())
	}

	rec = post(t, h, `<Transaction service="WFS" version="1.1.0" xmlns="http://www.opengis.net/wfs" xmlns:ogc="http://www.opengis.net/ogc">
	  <Update typeName="wfstest:wfs_roads">
	    <Property><Name>lanes</Name><Value>9</Value></Property>
	    <ogc:Filter><ogc:PropertyIsEqualTo><ogc:PropertyName>name</ogc:PropertyName><ogc:Literal>new rd</ogc:Literal></ogc:PropertyIsEqualTo></ogc:Filter>
	  </Update></Transaction>`)
	if !strings.Contains(rec.Body.String(), "<wfs:totalUpdated>1</wfs:totalUpdated>") {
		t.Fatalf("update = %s", rec.Body.String())
	}

	rec = post(t, h, `<Transaction service="WFS" version="1.1.0" xmlns="http://www.opengis.net/wfs" xmlns:ogc="http://www.opengis.net/ogc">
	  <Delete typeName="wfstest:wfs_roads">
	    <ogc:Filter><ogc:PropertyIsEqualTo><ogc:PropertyName>name</ogc:PropertyName><ogc:Literal>new rd</ogc:Literal></ogc:PropertyIsEqualTo></ogc:Filter>
	  </Delete></Transaction>`)
	if !strings.Contains(rec.Body.String(), "<wfs:totalDeleted>1</wfs:totalDeleted>") {
		t.Fatalf("delete = %s", rec.Body.String())
	}
}

func TestGetFeatureById(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&request=GetFeature&typeNames=wfstest:wfs_roads&outputFormat=application/json&featureID=wfs_roads.1", nil)
	var fc struct {
		NumberReturned int `json:"numberReturned"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil || fc.NumberReturned != 1 {
		t.Fatalf("byid = %s", rec.Body.String())
	}
}
