package ows

import "testing"

func TestNegotiate(t *testing.T) {
	cases := []struct{ svc, req, want string }{
		{"WMS", "1.3.0", "1.3.0"},
		{"WMS", "1.1.1", "1.1.1"},
		{"WMS", "", "1.3.0"},      // none -> highest
		{"WMS", "9.9.9", "1.3.0"}, // above all -> highest below
		{"WMS", "1.2.0", "1.1.1"}, // between -> highest below
		{"WMS", "0.9.0", "1.1.1"}, // below all -> lowest
		{"WFS", "1.1.0", "1.1.0"},
		{"WFS", "2.0.0", "2.0.0"},
		{"NOPE", "1.0.0", ""},
	}
	for _, c := range cases {
		if got := Negotiate(c.svc, c.req); got != c.want {
			t.Errorf("Negotiate(%s,%s) = %s, want %s", c.svc, c.req, got, c.want)
		}
	}
}
