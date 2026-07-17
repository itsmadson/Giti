package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/connect"
	"github.com/giti/giti/services/catalog/internal/model"
)

func (a *api) v1StoreTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, connect.StoreTypes())
}

type storeReq struct {
	Workspace   string            `json:"workspace"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Kind        string            `json:"kind"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Connection  map[string]string `json:"connection"`
}

func (b storeReq) toModel() model.Store {
	kind := b.Kind
	if kind == "" {
		kind = "datastore"
	}
	conn := b.Connection
	if conn == nil {
		conn = map[string]string{}
	}
	return model.Store{Workspace: b.Workspace, Name: b.Name, Type: b.Type, Kind: kind,
		Description: b.Description, Enabled: b.Enabled, Connection: conn}
}

func (a *api) v1CreateStore(w http.ResponseWriter, r *http.Request) {
	var b storeReq
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Workspace == "" || b.Name == "" || b.Type == "" {
		http.Error(w, "workspace, name, type required", http.StatusBadRequest)
		return
	}
	st := b.toModel()
	if err := a.s.CreateStore(r.Context(), st); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.store.created", map[string]string{"name": st.Name, "workspace": st.Workspace})
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"name": st.Name})
}

func (a *api) v1UpdateStore(w http.ResponseWriter, r *http.Request) {
	ws, name := r.PathValue("ws"), r.PathValue("store")
	var b storeReq
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	b.Workspace, b.Name = ws, name
	if err := a.s.UpdateStore(r.Context(), ws, name, b.toModel()); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1DeleteStore(w http.ResponseWriter, r *http.Request) {
	ws, name := r.PathValue("ws"), r.PathValue("store")
	recurse := r.URL.Query().Get("recurse") == "true"
	if err := a.s.DeleteStore(r.Context(), ws, name, recurse); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// v1TestStore validates connectivity for a candidate store WITHOUT persisting.
func (a *api) v1TestStore(w http.ResponseWriter, r *http.Request) {
	var b storeReq
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	c, err := connect.ForType(b.Type)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := c.Validate(r.Context(), b.toModel()); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
