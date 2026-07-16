// Package rules implements GeoFence-style access rule evaluation.
package rules

import (
	"strings"

	"github.com/giti/giti/services/auth/internal/store"
)

type Subject struct {
	Username string
	Roles    []string
}

type Query struct {
	Service, Request, Workspace, Layer string
}

type Decision struct {
	Allow      bool     `json:"allow"`
	CQLRead    string   `json:"cqlRead,omitempty"`
	CQLWrite   string   `json:"cqlWrite,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}

func fieldMatch(ruleVal, queryVal string) bool {
	return ruleVal == "*" || ruleVal == "" || strings.EqualFold(ruleVal, queryVal)
}

func roleMatch(ruleRole string, roles []string) bool {
	if ruleRole == "*" || ruleRole == "" {
		return true
	}
	for _, r := range roles {
		if strings.EqualFold(ruleRole, r) {
			return true
		}
	}
	return false
}

func matches(r store.Rule, sub Subject, q Query) bool {
	return fieldMatch(r.Username, sub.Username) &&
		roleMatch(r.Rolename, sub.Roles) &&
		fieldMatch(r.Service, q.Service) &&
		fieldMatch(r.Request, q.Request) &&
		fieldMatch(r.Workspace, q.Workspace) &&
		fieldMatch(r.Layer, q.Layer)
}

func andCQL(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return "(" + a + ") AND (" + b + ")"
}

// Evaluate walks priority-ordered rules. LIMIT matches accumulate
// constraints; the first matching ALLOW/DENY decides. No decision rule
// matched -> defaultAllow (constraints still apply when allowed).
func Evaluate(rs []store.Rule, sub Subject, q Query, defaultAllow bool) Decision {
	var d Decision
	for _, r := range rs {
		if !matches(r, sub, q) {
			continue
		}
		switch r.Access {
		case "LIMIT":
			d.CQLRead = andCQL(d.CQLRead, r.CQLRead)
			d.CQLWrite = andCQL(d.CQLWrite, r.CQLWrite)
			d.Attributes = append(d.Attributes, r.Attributes...)
		case "ALLOW":
			d.Allow = true
			return d
		case "DENY":
			return Decision{Allow: false}
		}
	}
	d.Allow = defaultAllow
	if !d.Allow {
		return Decision{Allow: false}
	}
	return d
}
