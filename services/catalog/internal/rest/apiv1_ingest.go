package rest

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/giti/giti/services/catalog/internal/model"
)

// v1Ingest accepts an uploaded GeoJSON file, loads it into a PostGIS table
// (host=self), and publishes a servable layer. This is the correct path for
// file data — wms/wfs read PostGIS, not files.
func (a *api) v1Ingest(w http.ResponseWriter, r *http.Request) {
	ws := r.URL.Query().Get("workspace")
	if ws == "" {
		ws = "default"
	}
	name := r.URL.Query().Get("name")

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".geojson" && ext != ".json" {
		http.Error(w, "only GeoJSON upload is supported for direct ingest; other formats need conversion first", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 512<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}

	// 1) load into PostGIS
	table, err := a.ingestName(r, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := a.s.IngestGeoJSON(r.Context(), table, data); err != nil {
		http.Error(w, "ingest failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 2) ensure workspace + a self PostGIS store
	_ = a.s.CreateWorkspace(r.Context(), model.Workspace{Name: ws})
	storeName := "uploads"
	_ = a.s.CreateStore(r.Context(), model.Store{
		Workspace: ws, Name: storeName, Kind: "datastore", Type: "PostGIS", Enabled: true,
		Connection: map[string]string{"host": "self", "schema": "public"},
	})

	// 3) featuretype + layer
	if err := a.s.CreateFeatureType(r.Context(), model.FeatureType{
		Workspace: ws, Store: storeName, Name: table, NativeName: table,
		SRS: "EPSG:4326", Enabled: true,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = a.s.CreateLayer(r.Context(), model.Layer{
		Workspace: ws, Name: table, Type: "VECTOR", ResourceName: table,
		DefaultStyle: "generic", Enabled: true,
	})
	a.pub.Publish("catalog.featuretype.created", map[string]string{"name": table, "workspace": ws})
	a.pub.Publish("catalog.layer.created", map[string]string{"name": table, "workspace": ws})

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"workspace": ws, "layer": table})
}

// ingestName returns a table name derived from the requested name; the store
// layer sanitizes it further.
func (a *api) ingestName(r *http.Request, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errBadName
	}
	return name, nil
}

var errBadName = &badName{}

type badName struct{}

func (*badName) Error() string { return "layer name required" }
