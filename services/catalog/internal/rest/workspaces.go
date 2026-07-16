package rest

import (
	"errors"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/model"
	"github.com/giti/giti/services/catalog/internal/store"
)

// XML/JSON wire shapes for GeoServer compat.
type wsXML struct {
	XMLName  struct{} `xml:"workspace"`
	Name     string   `xml:"name"`
	Isolated bool     `xml:"isolated,omitempty"`
}
type wsListXML struct {
	XMLName struct{} `xml:"workspaces"`
	Items   []wsXML  `xml:"workspace"`
}
type wsJSON struct {
	Workspace struct {
		Name     string `json:"name"`
		Isolated bool   `json:"isolated,omitempty"`
	} `json:"workspace"`
}

func (a *api) workspaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /rest/workspaces", a.listWorkspaces)
	mux.HandleFunc("GET /rest/workspaces.json", a.listWorkspaces)
	mux.HandleFunc("POST /rest/workspaces", a.createWorkspace)
	mux.HandleFunc("POST /rest/workspaces.json", a.createWorkspace)
	mux.HandleFunc("GET /rest/workspaces/{ws}", a.getWorkspace)
	mux.HandleFunc("PUT /rest/workspaces/{ws}", a.updateWorkspace)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}", a.deleteWorkspace)
}

func (a *api) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListWorkspaces(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	xmlOut := wsListXML{}
	type wsRef struct {
		Name string `json:"name"`
	}
	jsonItems := []wsRef{}
	for _, ws := range list {
		xmlOut.Items = append(xmlOut.Items, wsXML{Name: ws.Name, Isolated: ws.Isolated})
		jsonItems = append(jsonItems, wsRef{Name: ws.Name})
	}
	writePayload(w, r, xmlOut,
		map[string]any{"workspaces": map[string]any{"workspace": jsonItems}})
}

func (a *api) readWorkspace(r *http.Request) (model.Workspace, error) {
	var x wsXML
	var j wsJSON
	if err := readPayload(r, &x, &j); err != nil {
		return model.Workspace{}, err
	}
	if j.Workspace.Name != "" {
		return model.Workspace{Name: j.Workspace.Name, Isolated: j.Workspace.Isolated}, nil
	}
	if x.Name == "" {
		return model.Workspace{}, errors.New("workspace name required")
	}
	return model.Workspace{Name: x.Name, Isolated: x.Isolated}, nil
}

func (a *api) createWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := a.readWorkspace(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.s.CreateWorkspace(r.Context(), ws); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.workspace.created", map[string]string{"name": ws.Name})
	w.Header().Set("Location", "/rest/workspaces/"+ws.Name)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(ws.Name))
}

func (a *api) getWorkspace(w http.ResponseWriter, r *http.Request) {
	name := trimFormat(r.PathValue("ws"))
	ws, err := a.s.GetWorkspace(r.Context(), name)
	if err != nil {
		httpErr(w, err)
		return
	}
	var j wsJSON
	j.Workspace.Name = ws.Name
	j.Workspace.Isolated = ws.Isolated
	writePayload(w, r, wsXML{Name: ws.Name, Isolated: ws.Isolated}, j)
}

func (a *api) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	name := trimFormat(r.PathValue("ws"))
	ws, err := a.readWorkspace(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.s.UpdateWorkspace(r.Context(), name, ws); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.workspace.updated", map[string]string{"name": ws.Name})
	w.WriteHeader(http.StatusOK)
}

func (a *api) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	name := trimFormat(r.PathValue("ws"))
	recurse := r.URL.Query().Get("recurse") == "true"
	if err := a.s.DeleteWorkspace(r.Context(), name, recurse); err != nil {
		if errors.Is(err, store.ErrConflict) {
			http.Error(w, "workspace not empty", http.StatusForbidden)
			return
		}
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.workspace.deleted", map[string]string{"name": name})
	w.WriteHeader(http.StatusOK)
}
