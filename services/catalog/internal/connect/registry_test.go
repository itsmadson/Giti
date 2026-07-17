package connect

import "testing"

func TestStoreTypesIncludesPostGIS(t *testing.T) {
	types := StoreTypes()
	var pg *StoreTypeMeta
	for i := range types {
		if types[i].Type == "PostGIS" {
			pg = &types[i]
		}
	}
	if pg == nil {
		t.Fatal("PostGIS not in StoreTypes()")
	}
	if pg.Category != "Vector" || pg.Kind != "datastore" {
		t.Fatalf("bad meta: %+v", pg)
	}
	keys := map[string]bool{}
	for _, p := range pg.Params {
		keys[p.Key] = true
	}
	for _, want := range []string{"host", "port", "database", "user", "passwd", "schema"} {
		if !keys[want] {
			t.Errorf("PostGIS meta missing param %q", want)
		}
	}
}
