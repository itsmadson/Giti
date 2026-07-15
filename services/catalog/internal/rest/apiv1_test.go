package rest

import (
	"strings"
	"testing"
)

func TestAPIV1Layers(t *testing.T) {
	mux, _ := testMux(t)
	setupWsAndStore(t, mux)
	do(t, mux, "POST", "/rest/workspaces/topp/datastores/files/featuretypes",
		"application/xml", `<featureType><name>roads</name><enabled>true</enabled></featureType>`)

	rec := do(t, mux, "GET", "/api/v1/layers", "", "")
	if rec.Code != 200 {
		t.Fatalf("api layers = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"workspace":"topp"`) || !strings.Contains(body, `"defaultStyle":"generic"`) {
		t.Fatalf("api layers body = %s", body)
	}
	rec = do(t, mux, "GET", "/api/v1/workspaces", "", "")
	if !strings.Contains(rec.Body.String(), `"name":"topp"`) {
		t.Fatalf("api workspaces = %s", rec.Body.String())
	}
}
