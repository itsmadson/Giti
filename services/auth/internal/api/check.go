package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/geoson/geoson/services/auth/internal/password"
	"github.com/geoson/geoson/services/auth/internal/rules"
	"github.com/geoson/geoson/services/auth/internal/token"
)

const tokenTTL = 8 * time.Hour

func (a *api) checkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("GET /check", a.check)
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var body struct{ Username, Password string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	u, err := a.s.GetUser(r.Context(), body.Username)
	if err != nil || !u.Enabled || !password.Verify(body.Password, u.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	roles, err := a.s.RolesOf(r.Context(), u.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tok, err := token.Issue(a.secret, u.Name, roles, tokenTTL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token": tok, "expiresIn": int(tokenTTL.Seconds()),
	})
}

// authenticate resolves the Authorization header to a subject.
// ok=false means credentials were presented but are invalid.
func (a *api) authenticate(r *http.Request) (sub rules.Subject, ok bool) {
	h := r.Header.Get("Authorization")
	switch {
	case strings.HasPrefix(h, "Basic "):
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "Basic "))
		if err != nil {
			return sub, false
		}
		name, pw, found := strings.Cut(string(raw), ":")
		if !found {
			return sub, false
		}
		u, err := a.s.GetUser(r.Context(), name)
		if err != nil || !u.Enabled || !password.Verify(pw, u.PasswordHash) {
			return sub, false
		}
		roles, _ := a.s.RolesOf(r.Context(), name)
		return rules.Subject{Username: name, Roles: roles}, true
	case strings.HasPrefix(h, "Bearer "):
		name, roles, err := token.Verify(a.secret, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return sub, false
		}
		return rules.Subject{Username: name, Roles: roles}, true
	default:
		return rules.Subject{}, true // anonymous
	}
}

func (a *api) check(w http.ResponseWriter, r *http.Request) {
	sub, ok := a.authenticate(r)
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"allow": false})
		return
	}
	q := rules.Query{
		Service:   r.Header.Get("X-Geoson-Service"),
		Request:   r.Header.Get("X-Geoson-Request"),
		Workspace: r.Header.Get("X-Geoson-Workspace"),
		Layer:     r.Header.Get("X-Geoson-Layer"),
	}

	var cacheKey string
	if a.rdb != nil {
		gen, _ := a.rdb.Get(r.Context(), "authz:gen").Result()
		cacheKey = fmt.Sprintf("authz:%s:%s:%s:%s:%s:%s",
			gen, sub.Username, q.Service, q.Request, q.Workspace, q.Layer)
		if cached, err := a.rdb.Get(r.Context(), cacheKey).Result(); err == nil {
			w.Write([]byte(cached))
			return
		}
	}

	rs, err := a.s.ListRules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d := rules.Evaluate(rs, sub, q, a.defaultAllow)
	out := map[string]any{
		"allow": d.Allow, "user": sub.Username, "roles": sub.Roles,
	}
	if d.CQLRead != "" {
		out["cqlRead"] = d.CQLRead
	}
	if d.CQLWrite != "" {
		out["cqlWrite"] = d.CQLWrite
	}
	if len(d.Attributes) > 0 {
		out["attributes"] = d.Attributes
	}
	buf, _ := json.Marshal(out)
	if a.rdb != nil {
		a.rdb.Set(r.Context(), cacheKey, buf, 60*time.Second)
	}
	w.Write(buf)
}
