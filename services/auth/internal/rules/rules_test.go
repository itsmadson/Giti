package rules

import (
	"testing"

	"github.com/geoson/geoson/services/auth/internal/store"
)

func r(pri int64, user, role, svc, ws, access, cql string) store.Rule {
	return store.Rule{Priority: pri, Username: user, Rolename: role, Service: svc,
		Request: "*", Workspace: ws, Layer: "*", Access: access, CQLRead: cql}
}

func TestFirstMatchDecides(t *testing.T) {
	rs := []store.Rule{
		r(10, "*", "*", "WMS", "secret", "DENY", ""),
		r(20, "*", "*", "*", "*", "ALLOW", ""),
	}
	d := Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "secret"}, true)
	if d.Allow {
		t.Fatal("secret workspace must be denied")
	}
	d = Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "open"}, true)
	if !d.Allow {
		t.Fatal("open workspace must be allowed")
	}
}

func TestRoleMatch(t *testing.T) {
	rs := []store.Rule{
		r(10, "*", "ROLE_EDITOR", "WFS", "*", "ALLOW", ""),
		r(20, "*", "*", "WFS", "*", "DENY", ""),
	}
	editor := Subject{Username: "alice", Roles: []string{"ROLE_EDITOR"}}
	if d := Evaluate(rs, editor, Query{Service: "WFS"}, true); !d.Allow {
		t.Fatal("editor must be allowed")
	}
	if d := Evaluate(rs, Subject{Username: "bob"}, Query{Service: "WFS"}, true); d.Allow {
		t.Fatal("bob must be denied")
	}
}

func TestLimitAccumulates(t *testing.T) {
	rs := []store.Rule{
		{Priority: 5, Username: "*", Rolename: "*", Service: "*", Request: "*",
			Workspace: "topp", Layer: "roads", Access: "LIMIT",
			CQLRead: "state='CA'", Attributes: []string{"id", "name"}},
		r(10, "*", "*", "*", "topp", "ALLOW", ""),
	}
	d := Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "topp", Layer: "roads"}, false)
	if !d.Allow || d.CQLRead != "state='CA'" || len(d.Attributes) != 2 {
		t.Fatalf("decision = %+v", d)
	}
	// two LIMIT rules AND together
	rs = append([]store.Rule{{Priority: 1, Username: "*", Rolename: "*", Service: "*",
		Request: "*", Workspace: "topp", Layer: "roads", Access: "LIMIT",
		CQLRead: "public=true"}}, rs...)
	d = Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "topp", Layer: "roads"}, false)
	if d.CQLRead != "(public=true) AND (state='CA')" {
		t.Fatalf("cql = %q", d.CQLRead)
	}
}

func TestDefaultWhenNoMatch(t *testing.T) {
	if d := Evaluate(nil, Subject{}, Query{Service: "WMS"}, true); !d.Allow {
		t.Fatal("default allow")
	}
	if d := Evaluate(nil, Subject{}, Query{Service: "WMS"}, false); d.Allow {
		t.Fatal("default deny")
	}
}

func TestCaseInsensitiveMatch(t *testing.T) {
	rs := []store.Rule{r(10, "*", "*", "wms", "TOPP", "DENY", "")}
	if d := Evaluate(rs, Subject{}, Query{Service: "WMS", Workspace: "topp"}, true); d.Allow {
		t.Fatal("case-insensitive match failed")
	}
}
