package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/giti/giti/libs/ogc-kit/ows"
)

type authDecision struct {
	Allow      bool     `json:"allow"`
	User       string   `json:"user"`
	Roles      []string `json:"roles"`
	CQLRead    string   `json:"cqlRead"`
	CQLWrite   string   `json:"cqlWrite"`
	Attributes []string `json:"attributes"`
}

var authClient = &http.Client{Timeout: 5 * time.Second}

// authzMiddleware asks the auth service to authorize each OWS request.
// authURL == "" disables enforcement (pass-through).
func authzMiddleware(authURL string, next http.Handler) http.Handler {
	if authURL == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := ows.ParseKVP(r.URL.Query())
		wsName, layer, _ := parsePath(r.URL.Path)

		checkReq, _ := http.NewRequestWithContext(r.Context(), "GET", authURL+"/check", nil)
		if h := r.Header.Get("Authorization"); h != "" {
			checkReq.Header.Set("Authorization", h)
		}
		checkReq.Header.Set("X-Giti-Service", req.Service)
		checkReq.Header.Set("X-Giti-Request", req.Request)
		checkReq.Header.Set("X-Giti-Workspace", wsName)
		checkReq.Header.Set("X-Giti-Layer", layer)

		resp, err := authClient.Do(checkReq)
		if err != nil {
			ows.WriteException(w, req.Service, req.Version, req.Get("EXCEPTIONS"),
				ows.ServiceError{Code: ows.CodeNoApplicableCode,
					Message: "Authorization service unavailable", Status: 503})
			return
		}
		defer resp.Body.Close()
		var d authDecision
		json.NewDecoder(resp.Body).Decode(&d)

		if resp.StatusCode == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="giti"`)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if !d.Allow {
			if d.User == "" { // anonymous: challenge for credentials
				w.Header().Set("WWW-Authenticate", `Basic realm="giti"`)
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			ows.WriteException(w, req.Service, req.Version, req.Get("EXCEPTIONS"),
				ows.ServiceError{Code: ows.CodeNoApplicableCode,
					Message: "Access denied", Status: 403})
			return
		}
		r.Header.Set("X-Giti-User", d.User)
		if len(d.Roles) > 0 {
			buf, _ := json.Marshal(d.Roles)
			r.Header.Set("X-Giti-Roles", string(buf))
		}
		if d.CQLRead != "" {
			r.Header.Set("X-Giti-CQL-Read", d.CQLRead)
		}
		if d.CQLWrite != "" {
			r.Header.Set("X-Giti-CQL-Write", d.CQLWrite)
		}
		next.ServeHTTP(w, r)
	})
}
