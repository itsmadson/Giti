package process

import "testing"

func TestRegistryHasCoreProcesses(t *testing.T) {
	r := Registry()
	for _, id := range []string{"giti:buffer", "giti:centroid", "giti:area",
		"giti:length", "giti:reproject", "giti:intersection", "giti:union",
		"giti:simplify"} {
		if _, ok := r[id]; !ok {
			t.Fatalf("missing process %s", id)
		}
	}
	p, ok := Get("giti:buffer")
	if !ok || len(p.Inputs) < 2 {
		t.Fatalf("buffer process = %+v", p)
	}
}
