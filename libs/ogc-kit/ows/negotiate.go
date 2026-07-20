package ows

import "strings"

var Versions = map[string][]string{
	"WMS":  {"1.3.0", "1.1.1"},
	"WFS":  {"2.0.0", "1.1.0", "1.0.0"},
	"WMTS": {"1.0.0"},
	"WPS":  {"1.0.0"},
	"CSW":  {"2.0.2"},
}

// cmpVer compares dotted numeric versions: -1, 0, 1.
func cmpVer(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			for _, ch := range as[i] {
				av = av*10 + int(ch-'0')
			}
		}
		if i < len(bs) {
			for _, ch := range bs[i] {
				bv = bv*10 + int(ch-'0')
			}
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Negotiate returns the version to use per OGC rules: exact match; else
// highest supported below requested; else lowest supported. Empty requested
// -> highest supported. Unknown service -> "".
func Negotiate(service, requested string) string {
	supported, ok := Versions[strings.ToUpper(service)]
	if !ok {
		return ""
	}
	if requested == "" {
		return supported[0]
	}
	for _, v := range supported { // newest-first
		if cmpVer(v, requested) <= 0 {
			return v
		}
	}
	return supported[len(supported)-1]
}
