package api

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"strings"

	"github.com/geoson/geoson/services/auth/internal/password"
	"github.com/geoson/geoson/services/auth/internal/store"
)

// handleBoth registers a pattern under /rest/... and /geoserver/rest/...
func handleBoth(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
	method, path, _ := strings.Cut(pattern, " ")
	mux.HandleFunc(pattern, h)
	mux.HandleFunc(method+" /geoserver"+path, h)
}

func httpErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, store.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func wantsJSON(r *http.Request) bool {
	return strings.HasSuffix(r.URL.Path, ".json") ||
		strings.Contains(r.Header.Get("Accept"), "application/json")
}

func (a *api) securityRoutes(mux *http.ServeMux) {
	handleBoth(mux, "GET /rest/security/usergroup/users", a.listUsers)
	handleBoth(mux, "GET /rest/security/usergroup/users.json", a.listUsers)
	handleBoth(mux, "POST /rest/security/usergroup/users", a.createUser)
	handleBoth(mux, "DELETE /rest/security/usergroup/user/{u}", a.deleteUser)
	handleBoth(mux, "GET /rest/security/roles", a.listRoles)
	handleBoth(mux, "GET /rest/security/roles.json", a.listRoles)
	handleBoth(mux, "POST /rest/security/roles/role/{r}", a.createRole)
	handleBoth(mux, "DELETE /rest/security/roles/role/{r}", a.deleteRole)
	handleBoth(mux, "POST /rest/security/roles/role/{r}/user/{u}", a.associateRole)
}

type userJSON struct {
	UserName string `json:"userName" xml:"userName"`
	Enabled  bool   `json:"enabled" xml:"enabled"`
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.s.ListUsers(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	out := make([]userJSON, 0, len(users))
	for _, u := range users {
		out = append(out, userJSON{UserName: u.Name, Enabled: u.Enabled})
	}
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"users": out})
		return
	}
	type usersXML struct {
		XMLName struct{}   `xml:"users"`
		Items   []userJSON `xml:"user"`
	}
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(usersXML{Items: out})
}

func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		User struct {
			UserName string `json:"userName"`
			Password string `json:"password"`
			Enabled  bool   `json:"enabled"`
		} `json:"user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.User.UserName == "" {
		http.Error(w, "invalid user body", http.StatusBadRequest)
		return
	}
	hash, err := password.Hash(body.User.Password)
	if err != nil {
		httpErr(w, err)
		return
	}
	if err := a.s.CreateUser(r.Context(), store.User{
		Name: body.User.UserName, Enabled: body.User.Enabled, PasswordHash: hash}); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusCreated)
}

func (a *api) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteUser(r.Context(), r.PathValue("u")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}

func (a *api) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := a.s.ListRoles(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	if roles == nil {
		roles = []string{}
	}
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"roles": roles})
		return
	}
	type rolesXML struct {
		XMLName struct{} `xml:"roles"`
		Items   []string `xml:"role"`
	}
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(rolesXML{Items: roles})
}

func (a *api) createRole(w http.ResponseWriter, r *http.Request) {
	if err := a.s.CreateRole(r.Context(), r.PathValue("r")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusCreated)
}

func (a *api) deleteRole(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteRole(r.Context(), r.PathValue("r")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}

func (a *api) associateRole(w http.ResponseWriter, r *http.Request) {
	if err := a.s.AssignRoleUser(r.Context(), r.PathValue("r"), r.PathValue("u")); err != nil {
		httpErr(w, err)
		return
	}
	a.bumpGen(r.Context())
	w.WriteHeader(http.StatusOK)
}
