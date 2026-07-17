package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/store"
)

func (a *api) v1ListGroups(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListGroupsFull(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, list)
}

func (a *api) v1GetGroup(w http.ResponseWriter, r *http.Request) {
	g, err := a.s.GetGroupFull(r.Context(), r.PathValue("ws"), r.PathValue("name"))
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, g)
}

func (a *api) v1SaveGroup(w http.ResponseWriter, r *http.Request) {
	var g store.GroupFull
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil || g.Workspace == "" || g.Name == "" {
		http.Error(w, "workspace and name required", http.StatusBadRequest)
		return
	}
	if err := a.s.SaveGroup(r.Context(), g); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layergroup.saved", map[string]string{"name": g.Name, "workspace": g.Workspace})
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1DeleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteLayerGroup(r.Context(), r.PathValue("ws"), r.PathValue("name")); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1GroupBounds(w http.ResponseWriter, r *http.Request) {
	b, err := a.s.ComputeGroupBounds(r.Context(), r.PathValue("ws"), r.PathValue("name"))
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"bounds": b})
}
