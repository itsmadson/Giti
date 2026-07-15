package rest

import (
	"strings"
	"testing"
)

func TestDatastoreRESTLifecycle(t *testing.T) {
	mux, _ := testMux(t)
	do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>topp</name></workspace>`)

	rec := do(t, mux, "POST", "/rest/workspaces/topp/datastores", "application/xml",
		`<dataStore><name>pg</name><type>Directory</type><enabled>true</enabled>
		 <connectionParameters><entry key="host">postgres</entry>
		 <entry key="port">5432</entry></connectionParameters></dataStore>`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores/pg", "", "")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `<entry key="host">postgres</entry>`) {
		t.Fatalf("GET xml = %d %s", rec.Code, body)
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores/pg.json", "", "")
	body = rec.Body.String()
	if !strings.Contains(body, `"@key":"host"`) || !strings.Contains(body, `"$":"postgres"`) {
		t.Fatalf("GET json = %s", body)
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/datastores.json", "", "")
	if !strings.Contains(rec.Body.String(), `"dataStores"`) {
		t.Fatalf("list json = %s", rec.Body.String())
	}
	rec = do(t, mux, "GET", "/rest/workspaces/topp/coveragestores/pg", "", "")
	if rec.Code != 404 {
		t.Fatalf("cross-kind GET = %d", rec.Code)
	}
	rec = do(t, mux, "DELETE", "/rest/workspaces/topp/datastores/pg", "", "")
	if rec.Code != 200 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
}

func TestDatastoreCreateValidatesConnection(t *testing.T) {
	mux, _ := testMux(t)
	do(t, mux, "POST", "/rest/workspaces", "application/xml",
		`<workspace><name>vt</name></workspace>`)
	rec := do(t, mux, "POST", "/rest/workspaces/vt/datastores", "application/xml",
		`<dataStore><name>bad</name><type>PostGIS</type><enabled>true</enabled>
		 <connectionParameters><entry key="host">127.0.0.1</entry><entry key="port">1</entry>
		 <entry key="database">x</entry><entry key="user">x</entry><entry key="passwd">x</entry>
		 </connectionParameters></dataStore>`)
	if rec.Code != 400 {
		t.Fatalf("bad store POST = %d %s", rec.Code, rec.Body.String())
	}
}
