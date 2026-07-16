package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/giti/giti/services/auth/internal/store"
)

type ruleJSON struct {
	ID         int64    `json:"id,omitempty"`
	Priority   int64    `json:"priority"`
	UserName   string   `json:"userName,omitempty"`
	RoleName   string   `json:"roleName,omitempty"`
	Service    string   `json:"service,omitempty"`
	Request    string   `json:"request,omitempty"`
	Workspace  string   `json:"workspace,omitempty"`
	Layer      string   `json:"layer,omitempty"`
	Access     string   `json:"access"`
	CQLRead    string   `json:"cqlFilterRead,omitempty"`
	CQLWrite   string   `json:"cqlFilterWrite,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}

func (a *api) geofenceRoutes(mux *http.ServeMux) {
	handleBoth(mux, "GET /rest/geofence/rules", a.listRules)
	handleBoth(mux, "POST /rest/geofence/rules", a.createRule)
	handleBoth(mux, "DELETE /rest/geofence/rules/id/{id}", a.deleteRule)
}

func (a *api) listRules(w http.ResponseWriter, r *http.Request) {
	rs, err := a.s.ListRules(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	out := make([]ruleJSON, 0, len(rs))
	for _, ru := range rs {
		out = append(out, ruleJSON{ID: ru.ID, Priority: ru.Priority, UserName: ru.Username,
			RoleName: ru.Rolename, Service: ru.Service, Request: ru.Request,
			Workspace: ru.Workspace, Layer: ru.Layer, Access: ru.Access,
			CQLRead: ru.CQLRead, CQLWrite: ru.CQLWrite, Attributes: ru.Attributes})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"count": len(out), "rules": out})
}

func (a *api) createRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rule ruleJSON `json:"rule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid rule body", http.StatusBadRequest)
		return
	}
	b := body.Rule
	if b.Access != "ALLOW" && b.Access != "DENY" && b.Access != "LIMIT" {
		http.Error(w, "access must be ALLOW, DENY or LIMIT", http.StatusBadRequest)
		return
	}
	id, err := a.s.CreateRule(r.Context(), store.Rule{Priority: b.Priority,
		Username: b.UserName, Rolename: b.RoleName, Service: b.Service, Request: b.Request,
		Workspace: b.Workspace, Layer: b.Layer, Access: b.Access,
		CQLRead: b.CQLRead, CQLWrite: b.CQLWrite, Attributes: b.Attributes})
	if err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (a *api) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := a.s.DeleteRule(r.Context(), id); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}
