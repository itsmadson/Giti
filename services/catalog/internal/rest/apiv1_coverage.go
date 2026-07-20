package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/model"
)

// v1RegisterCoverage registers an already-uploaded GeoTIFF as a coveragestore +
// coverage + RASTER layer. The file lives on the shared data volume (written by
// the convert service); wms reads it to serve WMS raster / WCS.
func (a *api) v1RegisterCoverage(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Workspace string `json:"workspace"`
		Name      string `json:"name"`
		Path      string `json:"path"`
		SRS       string `json:"srs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Workspace == "" || b.Name == "" || b.Path == "" {
		http.Error(w, "workspace, name, path required", http.StatusBadRequest)
		return
	}
	if b.SRS == "" {
		b.SRS = "EPSG:4326"
	}
	store := b.Name + "_cov"
	_ = a.s.CreateWorkspace(r.Context(), model.Workspace{Name: b.Workspace})
	_ = a.s.CreateStore(r.Context(), model.Store{
		Workspace: b.Workspace, Name: store, Kind: "coveragestore", Type: "GeoTIFF", Enabled: true,
		Connection: map[string]string{"url": "file://" + b.Path},
	})
	if err := a.s.CreateCoverage(r.Context(), model.Coverage{
		Workspace: b.Workspace, Store: store, Name: b.Name, NativeName: b.Name,
		SRS: b.SRS, Enabled: true,
	}); err != nil {
		httpErr(w, err)
		return
	}
	_ = a.s.CreateLayer(r.Context(), model.Layer{
		Workspace: b.Workspace, Name: b.Name, Type: "RASTER", ResourceName: b.Name, Enabled: true,
	})
	a.pub.Publish("catalog.layer.created", map[string]string{"name": b.Name, "workspace": b.Workspace})
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"workspace": b.Workspace, "layer": b.Name})
}
