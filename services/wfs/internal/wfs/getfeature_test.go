package wfs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geoson/geoson/services/wfs/internal/meta"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	dsn := os.Getenv("GEOSON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GEOSON_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	seedFixture(t, pool)
	mux := http.NewServeMux()
	Mount(mux, meta.New(pool))
	return mux
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	must := func(sql string) {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis`)
	pool.Exec(ctx, `DELETE FROM layers WHERE workspace='wfstest'`)
	pool.Exec(ctx, `DELETE FROM resources WHERE workspace='wfstest'`)
	pool.Exec(ctx, `DELETE FROM stores WHERE workspace='wfstest'`)
	pool.Exec(ctx, `DELETE FROM workspaces WHERE name='wfstest'`)
	must(`INSERT INTO workspaces(name) VALUES('wfstest')`)
	must(`INSERT INTO stores(workspace,name,kind,type,enabled,connection)
		VALUES('wfstest','local','datastore','PostGIS',true,'{"host":"self"}'::jsonb)`)
	must(`DROP TABLE IF EXISTS wfs_roads`)
	must(`CREATE TABLE wfs_roads (
		id serial PRIMARY KEY, name text, lanes int, geom geometry(LineString,4326))`)
	must(`INSERT INTO wfs_roads(name,lanes,geom) VALUES
		('main st', 4, ST_GeomFromText('LINESTRING(0 0, 1 1)', 4326)),
		('back rd', 2, ST_GeomFromText('LINESTRING(2 2, 3 3)', 4326)),
		('ring way', 6, ST_GeomFromText('LINESTRING(10 10, 11 11)', 4326))`)
	must(`INSERT INTO resources(workspace,store,name,kind,native_name,srs,enabled)
		VALUES('wfstest','local','wfs_roads','featuretype','wfs_roads','EPSG:4326',true)`)
	must(`INSERT INTO layers(workspace,name,type,resource_name,default_style,enabled)
		VALUES('wfstest','wfs_roads','VECTOR','wfs_roads','generic',true)`)
}

func get(t *testing.T, h http.Handler, q string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/wfs?"+q, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGetFeatureGeoJSON(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&version=2.0.0&request=GetFeature&typeNames=wfstest:wfs_roads&outputFormat=application/json", nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry   json.RawMessage `json:"geometry"`
			Properties map[string]any  `json:"properties"`
			ID         string          `json:"id"`
		} `json:"features"`
		NumberMatched int `json:"numberMatched"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatalf("%v: %s", err, rec.Body.String())
	}
	if fc.Type != "FeatureCollection" || len(fc.Features) != 3 || fc.NumberMatched != 3 {
		t.Fatalf("fc = %+v", fc)
	}
	if fc.Features[0].Properties["name"] == nil {
		t.Fatalf("properties missing: %+v", fc.Features[0])
	}
}

func TestGetFeatureCQLAndPaging(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&request=GetFeature&typeNames=wfstest:wfs_roads&outputFormat=application/json&CQL_FILTER=lanes%3E2&sortBy=lanes&startIndex=0&count=1", nil)
	var fc struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
		} `json:"features"`
		NumberMatched  int `json:"numberMatched"`
		NumberReturned int `json:"numberReturned"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatalf("%v: %s", err, rec.Body.String())
	}
	if fc.NumberMatched != 2 || fc.NumberReturned != 1 {
		t.Fatalf("counts = %+v", fc)
	}
	if fc.Features[0].Properties["lanes"] != float64(4) {
		t.Fatalf("sort/filter wrong: %+v", fc.Features[0].Properties)
	}
}

func TestGetFeatureBBox(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&request=GetFeature&typeNames=wfstest:wfs_roads&outputFormat=application/json&bbox=-1,-1,5,5", nil)
	var fc struct {
		NumberReturned int `json:"numberReturned"`
	}
	json.Unmarshal(rec.Body.Bytes(), &fc)
	if fc.NumberReturned != 2 {
		t.Fatalf("bbox returned = %d body=%s", fc.NumberReturned, rec.Body.String())
	}
}

func TestGetFeatureHits(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&version=2.0.0&request=GetFeature&typeNames=wfstest:wfs_roads&resultType=hits", nil)
	if !strings.Contains(rec.Body.String(), `numberMatched="3"`) {
		t.Fatalf("hits = %s", rec.Body.String())
	}
}

func TestGetFeatureAuthCQLHeader(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&request=GetFeature&typeNames=wfstest:wfs_roads&outputFormat=application/json",
		map[string]string{"X-Geoson-CQL-Read": "lanes = 2"})
	var fc struct {
		NumberReturned int `json:"numberReturned"`
	}
	json.Unmarshal(rec.Body.Bytes(), &fc)
	if fc.NumberReturned != 1 {
		t.Fatalf("auth cql = %d", fc.NumberReturned)
	}
}

func TestGetFeaturePropertyName(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&request=GetFeature&typeNames=wfstest:wfs_roads&outputFormat=application/json&propertyName=name", nil)
	var fc struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	json.Unmarshal(rec.Body.Bytes(), &fc)
	if len(fc.Features[0].Properties) != 1 {
		t.Fatalf("props = %+v", fc.Features[0].Properties)
	}
}

func TestGetFeatureUnknownLayer(t *testing.T) {
	h := testHandler(t)
	rec := get(t, h, "service=WFS&version=2.0.0&request=GetFeature&typeNames=wfstest:ghost", nil)
	if !strings.Contains(rec.Body.String(), "ExceptionReport") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
