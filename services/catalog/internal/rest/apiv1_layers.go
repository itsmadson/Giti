package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/store"
)

func (a *api) v1PatchFeatureType(w http.ResponseWriter, r *http.Request) {
	var p store.FeatureTypePatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if err := a.s.PatchFeatureType(r.Context(), r.PathValue("ws"), r.PathValue("name"), p); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layer.updated", map[string]string{"name": r.PathValue("name"), "workspace": r.PathValue("ws")})
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1PatchLayer(w http.ResponseWriter, r *http.Request) {
	var p store.LayerPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if err := a.s.PatchLayer(r.Context(), r.PathValue("ws"), r.PathValue("name"), p); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layer.updated", map[string]string{"name": r.PathValue("name"), "workspace": r.PathValue("ws")})
	w.WriteHeader(http.StatusNoContent)
}

// v1LayerBbox recomputes a layer's bounding box (mode=data|srs) in EPSG:4326.
func (a *api) v1LayerBbox(w http.ResponseWriter, r *http.Request) {
	d, err := a.s.GetLayerDetail(r.Context(), r.PathValue("ws"), r.PathValue("name"))
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"bbox": d.Bbox, "featureCount": d.FeatureCount})
}

func (a *api) v1SRS(w http.ResponseWriter, r *http.Request) {
	info, err := a.s.GetSRSInfo(r.Context(), r.PathValue("code"))
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, info)
}
