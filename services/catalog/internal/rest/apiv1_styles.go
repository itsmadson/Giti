package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/model"
	"github.com/giti/giti/services/catalog/internal/style"
)

func (a *api) v1ValidateStyle(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Format  string `json:"format"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	writeJSON(w, style.Validate(b.Format, b.Content))
}

func (a *api) v1GenerateStyle(w http.ResponseWriter, r *http.Request) {
	var b struct {
		GeomType string `json:"geomType"`
		Color    string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"sld": style.GenerateDefault(b.GeomType, b.Color)})
}

func (a *api) v1GetStyle(w http.ResponseWriter, r *http.Request) {
	st, err := a.s.GetStyle(r.Context(), "", r.PathValue("name"))
	if err != nil {
		httpErr(w, err)
		return
	}
	out := map[string]any{"name": st.Name, "format": st.Format, "content": st.Body}
	if len(st.Model) > 0 && string(st.Model) != "null" {
		out["model"] = json.RawMessage(st.Model)
	}
	writeJSON(w, out)
}

func (a *api) v1CreateStyle(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name    string          `json:"name"`
		Format  string          `json:"format"`
		Content string          `json:"content"`
		Model   json.RawMessage `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if b.Format == "" {
		b.Format = "sld"
	}
	err := a.s.CreateStyle(r.Context(), model.Style{Name: b.Name, Format: b.Format, Body: b.Content, Model: b.Model})
	if err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"name": b.Name})
}

func (a *api) v1UpdateStyle(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Format  string          `json:"format"`
		Content string          `json:"content"`
		Model   json.RawMessage `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	name := r.PathValue("name")
	if b.Format == "" {
		b.Format = "sld"
	}
	if err := a.s.UpdateStyle(r.Context(), "", name, model.Style{Name: name, Format: b.Format, Body: b.Content, Model: b.Model}); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.style.updated", map[string]string{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) v1DeleteStyle(w http.ResponseWriter, r *http.Request) {
	if err := a.s.DeleteStyle(r.Context(), "", r.PathValue("name")); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
