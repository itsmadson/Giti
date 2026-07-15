package rest

import (
	"net/http"
	"strings"
	"testing"
)

func setupWsAndStore(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>topp</name></workspace>`)
	do(t, mux, "POST", "/rest/workspaces/topp/datastores", "application/xml",
		`<dataStore><name>files</name><type>Directory</type><enabled>true</enabled>
		 <connectionParameters/></dataStore>`)
}

func TestFeatureTypeAutoCreatesLayer(t *testing.T) {
	mux, _ := testMux(t)
	setupWsAndStore(t, mux)

	rec := do(t, mux, "POST", "/rest/workspaces/topp/datastores/files/featuretypes",
		"application/xml",
		`<featureType><name>roads</name><nativeName>roads</nativeName><srs>EPSG:3857</srs><enabled>true</enabled></featureType>`)
	if rec.Code != 201 {
		t.Fatalf("POST ft = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores/files/featuretypes/roads", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<srs>EPSG:3857</srs>") {
		t.Fatalf("GET ft = %d %s", rec.Code, rec.Body.String())
	}
	// auto-created layer with default style
	rec = do(t, mux, "GET", "/rest/layers/topp:roads", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "generic") {
		t.Fatalf("GET layer = %d %s", rec.Code, rec.Body.String())
	}
}

func TestStyleUploadSLD(t *testing.T) {
	mux, _ := testMux(t)
	sld := `<?xml version="1.0"?><StyledLayerDescriptor version="1.0.0"><NamedLayer><Name>x</Name></NamedLayer></StyledLayerDescriptor>`
	rec := do(t, mux, "POST", "/rest/styles?name=mystyle", "application/vnd.ogc.sld+xml", sld)
	if rec.Code != 201 {
		t.Fatalf("POST sld = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/styles/mystyle.sld", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "StyledLayerDescriptor") {
		t.Fatalf("GET sld = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/styles/mystyle.json", "", "")
	if !strings.Contains(rec.Body.String(), `"name":"mystyle"`) {
		t.Fatalf("GET style json = %s", rec.Body.String())
	}
	do(t, mux, "DELETE", "/rest/styles/mystyle", "", "")
}

func TestLayerGroupREST(t *testing.T) {
	mux, _ := testMux(t)
	setupWsAndStore(t, mux)
	do(t, mux, "POST", "/rest/workspaces/topp/datastores/files/featuretypes",
		"application/xml", `<featureType><name>roads</name><nativeName>roads</nativeName><enabled>true</enabled></featureType>`)
	rec := do(t, mux, "POST", "/rest/workspaces/topp/layergroups", "application/xml",
		`<layerGroup><name>base</name><mode>SINGLE</mode><layers><layer>roads</layer></layers></layerGroup>`)
	if rec.Code != 201 {
		t.Fatalf("POST lg = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/layergroups/base", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<layer>roads</layer>") {
		t.Fatalf("GET lg = %d %s", rec.Code, rec.Body.String())
	}
}
