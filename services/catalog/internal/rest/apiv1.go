package rest

import (
	"encoding/json"
	"net/http"
)

// apiV1Routes serves the clean JSON API consumed by the Giti frontend.
func (a *api) apiV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/workspaces", func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /api/v1/layers", func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
}
