package wps

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testMux(t *testing.T) *http.ServeMux {
	t.Helper()
	db := testDB(t)
	mux := http.NewServeMux()
	Mount(mux, NewJobs(t.TempDir(), nil, db))
	return mux
}

func get(t *testing.T, mux *http.ServeMux, q string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/wps?"+q, nil))
	return rec
}

func TestCapabilities(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=GetCapabilities")
	if !strings.Contains(rec.Body.String(), "giti:buffer") ||
		!strings.Contains(rec.Body.String(), "Capabilities") {
		t.Fatalf("caps = %s", rec.Body.String())
	}
}

func TestDescribeProcess(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=DescribeProcess&identifier=giti:buffer")
	body := rec.Body.String()
	if !strings.Contains(body, "giti:buffer") || !strings.Contains(body, "distance") {
		t.Fatalf("describe = %s", body)
	}
}

func TestExecuteSyncBuffer(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=Execute&identifier=giti:buffer"+
		"&DataInputs=geom%3DPOINT(0%200)%3Bdistance%3D1")
	body := rec.Body.String()
	if !strings.Contains(body, "ExecuteResponse") || !strings.Contains(body, "POLYGON") {
		t.Fatalf("execute = %s", body)
	}
}

func TestExecuteAsyncAndPoll(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=Execute&identifier=giti:centroid"+
		"&DataInputs=geom%3DPOLYGON((0%200%2C0%202%2C2%202%2C2%200%2C0%200))&storeExecuteResponse=true")
	body := rec.Body.String()
	if !strings.Contains(body, "statusLocation") {
		t.Fatalf("async execute = %s", body)
	}
	i := strings.Index(body, "/wps/status/")
	id := body[i+len("/wps/status/"):]
	id = strings.FieldsFunc(id, func(r rune) bool { return r == '"' || r == '<' || r == ' ' })[0]
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/wps/status/"+id, nil))
	if !strings.Contains(rec.Body.String(), "accepted") &&
		!strings.Contains(rec.Body.String(), "succeeded") {
		t.Fatalf("status = %s", rec.Body.String())
	}
}

func TestExecuteUnknownProcess(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "service=WPS&version=1.0.0&request=Execute&identifier=giti:ghost&DataInputs=x%3D1")
	if !strings.Contains(rec.Body.String(), "ExceptionReport") {
		t.Fatalf("unknown = %s", rec.Body.String())
	}
}
