package process

import "testing"

func TestRegistryHasCoreProcesses(t *testing.T) {
	r := Registry()
	for _, id := range []string{"geoson:buffer", "geoson:centroid", "geoson:area",
		"geoson:length", "geoson:reproject", "geoson:intersection", "geoson:union",
		"geoson:simplify"} {
		if _, ok := r[id]; !ok {
			t.Fatalf("missing process %s", id)
		}
	}
	p, ok := Get("geoson:buffer")
	if !ok || len(p.Inputs) < 2 {
		t.Fatalf("buffer process = %+v", p)
	}
}
