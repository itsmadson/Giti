// Package ows implements shared OGC Web Service request handling:
// KVP/XML parsing, version negotiation, and exception rendering.
package ows

import (
	"net/url"
	"strings"
)

type Request struct {
	Service string
	Version string
	Request string
	params  map[string][]string
}

// ParseKVP parses OGC KVP params. Keys are case-insensitive per OGC 06-121r9.
func ParseKVP(q url.Values) Request {
	params := make(map[string][]string, len(q))
	for k, v := range q {
		params[strings.ToUpper(k)] = v
	}
	r := Request{params: params}
	r.Service = strings.ToUpper(first(params, "SERVICE"))
	r.Version = first(params, "VERSION")
	r.Request = first(params, "REQUEST")
	return r
}

func first(m map[string][]string, k string) string {
	if v := m[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (r Request) Get(key string) string { return first(r.params, strings.ToUpper(key)) }
func (r Request) Has(key string) bool {
	_, ok := r.params[strings.ToUpper(key)]
	return ok
}
