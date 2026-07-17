package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/store"
)

func (a *api) v1Gridsets(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListGridsets(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, list)
}

func (a *api) v1SaveGridset(w http.ResponseWriter, r *http.Request) {
	var g store.Gridset
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil || g.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if err := a.s.SaveGridset(r.Context(), g); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.gridset.saved", map[string]string{"name": g.Name})
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1DeleteGridset(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteGridset(r.Context(), r.PathValue("name")); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1BlobStores(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListBlobStores(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, list)
}

func (a *api) v1SaveBlobStore(w http.ResponseWriter, r *http.Request) {
	var b store.BlobStore
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if err := a.s.SaveBlobStore(r.Context(), b); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1DeleteBlobStore(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteBlobStore(r.Context(), r.PathValue("name")); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1GetQuota(w http.ResponseWriter, r *http.Request) {
	q, err := a.s.GetQuota(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, q)
}

func (a *api) v1SetQuota(w http.ResponseWriter, r *http.Request) {
	var q store.DiskQuota
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if err := a.s.SetQuota(r.Context(), q); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1GetLayerCache(w http.ResponseWriter, r *http.Request) {
	c, err := a.s.GetLayerCache(r.Context(), r.PathValue("ws"), r.PathValue("layer"))
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, c)
}

func (a *api) v1SaveLayerCache(w http.ResponseWriter, r *http.Request) {
	var c store.LayerCache
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	c.Workspace = r.PathValue("ws")
	c.Layer = r.PathValue("layer")
	if err := a.s.SaveLayerCache(r.Context(), c); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
