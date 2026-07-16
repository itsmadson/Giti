package wfs

import (
	"strings"
	"testing"
)

func TestCapabilitiesVersions(t *testing.T) {
	h := testHandler(t)
	cases := []struct{ version, want1, want2 string }{
		{"2.0.0", `version="2.0.0"`, "wfstest:wfs_roads"},
		{"1.1.0", `version="1.1.0"`, "DefaultSRS"},
		{"1.0.0", `version="1.0.0"`, "<SRS>EPSG:4326</SRS>"},
	}
	for _, c := range cases {
		rec := get(t, h, "service=WFS&version="+c.version+"&request=GetCapabilities", nil)
		body := rec.Body.String()
		if !strings.Contains(body, c.want1) || !strings.Contains(body, c.want2) {
			t.Fatalf("%s: %s", c.version, clip(body, 1200))
		}
	}
}
