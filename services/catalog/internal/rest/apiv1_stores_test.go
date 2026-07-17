package rest

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStoreTypesEndpoint(t *testing.T) {
	a := &api{}
	rec := httptest.NewRecorder()
	a.v1StoreTypes(rec, httptest.NewRequest("GET", "/api/v1/store-types", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "PostGIS") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
