package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/connect"
	"github.com/giti/giti/services/catalog/internal/model"
)

// apiV1Routes serves the clean JSON API consumed by the Giti frontend.
func (a *api) apiV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/workspaces", a.v1Workspaces)
	mux.HandleFunc("GET /api/v1/layers", a.v1Layers)
	mux.HandleFunc("GET /api/v1/layers/{ws}/{name}", a.v1LayerDetail)
	mux.HandleFunc("GET /api/v1/stores", a.v1Stores)
	mux.HandleFunc("GET /api/v1/store-types", a.v1StoreTypes)
	mux.HandleFunc("POST /api/v1/stores", a.v1CreateStore)
	mux.HandleFunc("PUT /api/v1/stores/{ws}/{store}", a.v1UpdateStore)
	mux.HandleFunc("DELETE /api/v1/stores/{ws}/{store}", a.v1DeleteStore)
	mux.HandleFunc("POST /api/v1/stores/{ws}/{store}/test", a.v1TestStore)
	mux.HandleFunc("POST /api/v1/stores/test", a.v1TestStore)
	mux.HandleFunc("GET /api/v1/stores/{ws}/{store}/tables", a.v1StoreTables)
	mux.HandleFunc("GET /api/v1/styles", a.v1Styles)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (a *api) v1Workspaces(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListWorkspaces(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	type ws struct {
		Name string `json:"name"`
	}
	out := []ws{}
	for _, item := range list {
		out = append(out, ws{Name: item.Name})
	}
	writeJSON(w, out)
}

func (a *api) v1Layers(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListLayers(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	type layer struct {
		Workspace    string `json:"workspace"`
		Name         string `json:"name"`
		Type         string `json:"type"`
		DefaultStyle string `json:"defaultStyle"`
	}
	out := []layer{}
	for _, l := range list {
		out = append(out, layer{Workspace: l.Workspace, Name: l.Name,
			Type: l.Type, DefaultStyle: l.DefaultStyle})
	}
	writeJSON(w, out)
}

func (a *api) v1LayerDetail(w http.ResponseWriter, r *http.Request) {
	d, err := a.s.GetLayerDetail(r.Context(), r.PathValue("ws"), r.PathValue("name"))
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, d)
}

func (a *api) v1Stores(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListAllStores(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	type store struct {
		Workspace string `json:"workspace"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		Kind      string `json:"kind"`
		Enabled   bool   `json:"enabled"`
	}
	out := []store{}
	for _, s := range list {
		out = append(out, store{Workspace: s.Workspace, Name: s.Name,
			Type: s.Type, Kind: s.Kind, Enabled: s.Enabled})
	}
	writeJSON(w, out)
}

// v1StoreTables introspects a store and returns publishable resources, marking
// which are already published as layers.
func (a *api) v1StoreTables(w http.ResponseWriter, r *http.Request) {
	ws, storeName := r.PathValue("ws"), r.PathValue("store")
	st, err := a.s.GetStore(r.Context(), ws, storeName, "datastore")
	if err != nil {
		httpErr(w, err)
		return
	}
	c, err := connect.ForType(st.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	st.Workspace = ws
	res, err := c.Introspect(r.Context(), st)
	if err != nil {
		http.Error(w, "introspect failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	published := map[string]bool{}
	if fts, err := a.s.ListFeatureTypes(r.Context(), ws, storeName); err == nil {
		for _, ft := range fts {
			published[ft.Name] = true
		}
	}
	type table struct {
		Name      string `json:"name"`
		GeomType  string `json:"geomType"`
		SRS       string `json:"srs"`
		Published bool   `json:"published"`
	}
	out := []table{}
	for _, ri := range res {
		out = append(out, table{Name: ri.Name, GeomType: ri.GeometryType,
			SRS: ri.SRS, Published: published[ri.Name]})
	}
	writeJSON(w, out)
}

func (a *api) v1Styles(w http.ResponseWriter, r *http.Request) {
	list, err := a.s.ListStyles(r.Context(), "")
	if err != nil {
		httpErr(w, err)
		return
	}
	type style struct {
		Name   string `json:"name"`
		Format string `json:"format"`
	}
	out := []style{}
	for _, s := range list {
		out = append(out, style{Name: s.Name, Format: s.Format})
	}
	writeJSON(w, out)
}

var _ = model.Store{}
