package rest

import (
	"io"
	"net/http"
	"strings"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

type ftXML struct {
	XMLName    struct{} `xml:"featureType"`
	Name       string   `xml:"name"`
	NativeName string   `xml:"nativeName"`
	Title      string   `xml:"title,omitempty"`
	SRS        string   `xml:"srs,omitempty"`
	Enabled    bool     `xml:"enabled"`
}
type ftJSON struct {
	FeatureType struct {
		Name       string `json:"name"`
		NativeName string `json:"nativeName"`
		Title      string `json:"title,omitempty"`
		SRS        string `json:"srs,omitempty"`
		Enabled    bool   `json:"enabled"`
	} `json:"featureType"`
}
type covXML struct {
	XMLName    struct{} `xml:"coverage"`
	Name       string   `xml:"name"`
	NativeName string   `xml:"nativeName"`
	Title      string   `xml:"title,omitempty"`
	SRS        string   `xml:"srs,omitempty"`
	Enabled    bool     `xml:"enabled"`
}
type layerXML struct {
	XMLName      struct{}       `xml:"layer"`
	Name         string         `xml:"name"`
	Type         string         `xml:"type"`
	DefaultStyle *layerStyleRef `xml:"defaultStyle,omitempty"`
}
type layerStyleRef struct {
	Name string `xml:"name"`
}
type styleXML struct {
	XMLName  struct{} `xml:"style"`
	Name     string   `xml:"name"`
	Format   string   `xml:"format,omitempty"`
	Filename string   `xml:"filename,omitempty"`
}
type lgXML struct {
	XMLName struct{} `xml:"layerGroup"`
	Name    string   `xml:"name"`
	Mode    string   `xml:"mode"`
	Layers  []string `xml:"layers>layer"`
}

func (a *api) layerRoutes(mux *http.ServeMux) {
	// featuretypes
	mux.HandleFunc("POST /rest/workspaces/{ws}/datastores/{ds}/featuretypes", a.createFeatureType)
	mux.HandleFunc("GET /rest/workspaces/{ws}/datastores/{ds}/featuretypes", a.listFeatureTypes)
	mux.HandleFunc("GET /rest/workspaces/{ws}/datastores/{ds}/featuretypes/{ft}", a.getFeatureType)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}/datastores/{ds}/featuretypes/{ft}", a.deleteFeatureType)
	// coverages
	mux.HandleFunc("POST /rest/workspaces/{ws}/coveragestores/{cs}/coverages", a.createCoverage)
	mux.HandleFunc("GET /rest/workspaces/{ws}/coveragestores/{cs}/coverages/{c}", a.getCoverage)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}/coveragestores/{cs}/coverages/{c}", a.deleteCoverage)
	// layers
	mux.HandleFunc("GET /rest/layers", a.listLayers)
	mux.HandleFunc("GET /rest/layers.json", a.listLayers)
	mux.HandleFunc("GET /rest/layers/{layer}", a.getLayer)
	mux.HandleFunc("PUT /rest/layers/{layer}", a.updateLayer)
	mux.HandleFunc("DELETE /rest/layers/{layer}", a.deleteLayer)
	// styles (global + workspace)
	mux.HandleFunc("POST /rest/styles", a.createStyle(""))
	mux.HandleFunc("GET /rest/styles", a.listStyles(""))
	mux.HandleFunc("GET /rest/styles.json", a.listStyles(""))
	mux.HandleFunc("GET /rest/styles/{s}", a.getStyle(""))
	mux.HandleFunc("PUT /rest/styles/{s}", a.updateStyle(""))
	mux.HandleFunc("DELETE /rest/styles/{s}", a.deleteStyle(""))
	mux.HandleFunc("POST /rest/workspaces/{ws}/styles", a.createStyleWS)
	mux.HandleFunc("GET /rest/workspaces/{ws}/styles/{s}", a.getStyleWS)
	// layergroups
	mux.HandleFunc("POST /rest/workspaces/{ws}/layergroups", a.createLayerGroup)
	mux.HandleFunc("GET /rest/workspaces/{ws}/layergroups/{lg}", a.getLayerGroup)
	mux.HandleFunc("DELETE /rest/workspaces/{ws}/layergroups/{lg}", a.deleteLayerGroup)
	mux.HandleFunc("POST /rest/layergroups", a.createLayerGroup) // ws="" global
	mux.HandleFunc("GET /rest/layergroups/{lg}", a.getLayerGroup)
	mux.HandleFunc("DELETE /rest/layergroups/{lg}", a.deleteLayerGroup)
}

func (a *api) createFeatureType(w http.ResponseWriter, r *http.Request) {
	ws, ds := r.PathValue("ws"), r.PathValue("ds")
	var x ftXML
	var j ftJSON
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ft := model.FeatureType{Workspace: ws, Store: ds,
		Name: x.Name, NativeName: x.NativeName, Title: x.Title, SRS: x.SRS, Enabled: x.Enabled}
	if j.FeatureType.Name != "" {
		b := j.FeatureType
		ft.Name, ft.NativeName, ft.Title, ft.SRS, ft.Enabled = b.Name, b.NativeName, b.Title, b.SRS, b.Enabled
	}
	if ft.Name == "" {
		http.Error(w, "featureType name required", http.StatusBadRequest)
		return
	}
	if ft.NativeName == "" {
		ft.NativeName = ft.Name
	}
	if ft.SRS == "" {
		ft.SRS = "EPSG:4326"
	}
	if err := a.s.CreateFeatureType(r.Context(), ft); err != nil {
		httpErr(w, err)
		return
	}
	// GeoServer auto-publishes a layer for every new featuretype.
	if err := a.s.CreateLayer(r.Context(), model.Layer{Workspace: ws, Name: ft.Name,
		Type: "VECTOR", ResourceName: ft.Name, DefaultStyle: "generic", Enabled: true}); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.featuretype.created",
		map[string]string{"name": ft.Name, "workspace": ws})
	a.pub.Publish("catalog.layer.created",
		map[string]string{"name": ft.Name, "workspace": ws})
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(ft.Name))
}

func (a *api) getFeatureType(w http.ResponseWriter, r *http.Request) {
	ws, ds, name := r.PathValue("ws"), r.PathValue("ds"), trimFormat(r.PathValue("ft"))
	ft, err := a.s.GetFeatureType(r.Context(), ws, ds, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	var j ftJSON
	j.FeatureType.Name, j.FeatureType.NativeName = ft.Name, ft.NativeName
	j.FeatureType.Title, j.FeatureType.SRS, j.FeatureType.Enabled = ft.Title, ft.SRS, ft.Enabled
	writePayload(w, r, ftXML{Name: ft.Name, NativeName: ft.NativeName,
		Title: ft.Title, SRS: ft.SRS, Enabled: ft.Enabled}, j)
}

func (a *api) listFeatureTypes(w http.ResponseWriter, r *http.Request) {
	ws, ds := r.PathValue("ws"), trimFormat(r.PathValue("ds"))
	fts, err := a.s.ListFeatureTypes(r.Context(), ws, ds)
	if err != nil {
		httpErr(w, err)
		return
	}
	type ref struct {
		Name string `json:"name" xml:"name"`
	}
	refs := []ref{}
	for _, ft := range fts {
		refs = append(refs, ref{Name: ft.Name})
	}
	type listXML struct {
		XMLName struct{} `xml:"featureTypes"`
		Items   []ref    `xml:"featureType"`
	}
	writePayload(w, r, listXML{Items: refs},
		map[string]any{"featureTypes": map[string]any{"featureType": refs}})
}

func (a *api) deleteFeatureType(w http.ResponseWriter, r *http.Request) {
	ws, ds, name := r.PathValue("ws"), r.PathValue("ds"), trimFormat(r.PathValue("ft"))
	if r.URL.Query().Get("recurse") == "true" {
		a.s.DeleteLayer(r.Context(), ws, name)
	}
	if err := a.s.DeleteFeatureType(r.Context(), ws, ds, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.featuretype.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

func (a *api) createCoverage(w http.ResponseWriter, r *http.Request) {
	ws, cs := r.PathValue("ws"), r.PathValue("cs")
	var x covXML
	var j struct {
		Coverage struct {
			Name       string `json:"name"`
			NativeName string `json:"nativeName"`
			Title      string `json:"title"`
			SRS        string `json:"srs"`
			Enabled    bool   `json:"enabled"`
		} `json:"coverage"`
	}
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c := model.Coverage{Workspace: ws, Store: cs,
		Name: x.Name, NativeName: x.NativeName, Title: x.Title, SRS: x.SRS, Enabled: x.Enabled}
	if j.Coverage.Name != "" {
		b := j.Coverage
		c.Name, c.NativeName, c.Title, c.SRS, c.Enabled = b.Name, b.NativeName, b.Title, b.SRS, b.Enabled
	}
	if c.Name == "" {
		http.Error(w, "coverage name required", http.StatusBadRequest)
		return
	}
	if c.NativeName == "" {
		c.NativeName = c.Name
	}
	if c.SRS == "" {
		c.SRS = "EPSG:4326"
	}
	if err := a.s.CreateCoverage(r.Context(), c); err != nil {
		httpErr(w, err)
		return
	}
	if err := a.s.CreateLayer(r.Context(), model.Layer{Workspace: ws, Name: c.Name,
		Type: "RASTER", ResourceName: c.Name, DefaultStyle: "raster", Enabled: true}); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.coverage.created",
		map[string]string{"name": c.Name, "workspace": ws})
	a.pub.Publish("catalog.layer.created",
		map[string]string{"name": c.Name, "workspace": ws})
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(c.Name))
}

func (a *api) getCoverage(w http.ResponseWriter, r *http.Request) {
	ws, cs, name := r.PathValue("ws"), r.PathValue("cs"), trimFormat(r.PathValue("c"))
	c, err := a.s.GetCoverage(r.Context(), ws, cs, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	writePayload(w, r, covXML{Name: c.Name, NativeName: c.NativeName,
		Title: c.Title, SRS: c.SRS, Enabled: c.Enabled},
		map[string]any{"coverage": map[string]any{"name": c.Name, "nativeName": c.NativeName,
			"title": c.Title, "srs": c.SRS, "enabled": c.Enabled}})
}

func (a *api) deleteCoverage(w http.ResponseWriter, r *http.Request) {
	ws, cs, name := r.PathValue("ws"), r.PathValue("cs"), trimFormat(r.PathValue("c"))
	if r.URL.Query().Get("recurse") == "true" {
		a.s.DeleteLayer(r.Context(), ws, name)
	}
	if err := a.s.DeleteCoverage(r.Context(), ws, cs, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.coverage.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

// splitLayer parses "ws:name" (GeoServer qualified layer) or bare "name".
func splitLayer(qualified string) (ws, name string) {
	qualified = trimFormat(qualified)
	if i := strings.IndexByte(qualified, ':'); i >= 0 {
		return qualified[:i], qualified[i+1:]
	}
	return "", qualified
}

func (a *api) getLayer(w http.ResponseWriter, r *http.Request) {
	ws, name := splitLayer(r.PathValue("layer"))
	l, err := a.s.GetLayer(r.Context(), ws, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	writePayload(w, r,
		layerXML{Name: l.Name, Type: l.Type, DefaultStyle: &layerStyleRef{Name: l.DefaultStyle}},
		map[string]any{"layer": map[string]any{"name": l.Name, "type": l.Type,
			"defaultStyle": map[string]any{"name": l.DefaultStyle}}})
}

func (a *api) listLayers(w http.ResponseWriter, r *http.Request) {
	ls, err := a.s.ListLayers(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	type ref struct {
		Name string `json:"name" xml:"name"`
	}
	refs := []ref{}
	for _, l := range ls {
		n := l.Name
		if l.Workspace != "" {
			n = l.Workspace + ":" + l.Name
		}
		refs = append(refs, ref{Name: n})
	}
	type listXML struct {
		XMLName struct{} `xml:"layers"`
		Items   []ref    `xml:"layer"`
	}
	writePayload(w, r, listXML{Items: refs},
		map[string]any{"layers": map[string]any{"layer": refs}})
}

func (a *api) updateLayer(w http.ResponseWriter, r *http.Request) {
	ws, name := splitLayer(r.PathValue("layer"))
	var x layerXML
	var j struct {
		Layer struct {
			DefaultStyle struct {
				Name string `json:"name"`
			} `json:"defaultStyle"`
			Enabled bool `json:"enabled"`
		} `json:"layer"`
	}
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cur, err := a.s.GetLayer(r.Context(), ws, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	if x.DefaultStyle != nil && x.DefaultStyle.Name != "" {
		cur.DefaultStyle = x.DefaultStyle.Name
	}
	if j.Layer.DefaultStyle.Name != "" {
		cur.DefaultStyle = j.Layer.DefaultStyle.Name
	}
	if err := a.s.UpdateLayer(r.Context(), ws, name, cur); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layer.updated",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

func (a *api) deleteLayer(w http.ResponseWriter, r *http.Request) {
	ws, name := splitLayer(r.PathValue("layer"))
	if err := a.s.DeleteLayer(r.Context(), ws, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layer.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}

func isSLDUpload(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "sld") {
		return true
	}
	return strings.Contains(ct, "application/xml") && r.URL.Query().Get("name") != ""
}

func (a *api) createStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isSLDUpload(r) {
			name := r.URL.Query().Get("name")
			if name == "" {
				http.Error(w, "name query param required for SLD upload", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			st := model.Style{Workspace: ws, Name: name, Format: "sld",
				Filename: name + ".sld", Body: string(body)}
			if err := a.s.CreateStyle(r.Context(), st); err != nil {
				httpErr(w, err)
				return
			}
			a.pub.Publish("catalog.style.created",
				map[string]string{"name": name, "workspace": ws})
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(name))
			return
		}
		var x styleXML
		var j struct {
			Style struct {
				Name     string `json:"name"`
				Format   string `json:"format"`
				Filename string `json:"filename"`
			} `json:"style"`
		}
		if err := readPayload(r, &x, &j); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st := model.Style{Workspace: ws, Name: x.Name, Format: x.Format, Filename: x.Filename}
		if j.Style.Name != "" {
			st.Name, st.Format, st.Filename = j.Style.Name, j.Style.Format, j.Style.Filename
		}
		if st.Format == "" {
			st.Format = "sld"
		}
		if st.Name == "" {
			http.Error(w, "style name required", http.StatusBadRequest)
			return
		}
		if err := a.s.CreateStyle(r.Context(), st); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.style.created",
			map[string]string{"name": st.Name, "workspace": ws})
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(st.Name))
	}
}

func (a *api) createStyleWS(w http.ResponseWriter, r *http.Request) {
	a.createStyle(r.PathValue("ws"))(w, r)
}

func (a *api) getStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.PathValue("s")
		if strings.HasSuffix(raw, ".sld") {
			st, err := a.s.GetStyle(r.Context(), ws, strings.TrimSuffix(raw, ".sld"))
			if err != nil {
				httpErr(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.ogc.sld+xml")
			w.Write([]byte(st.Body))
			return
		}
		st, err := a.s.GetStyle(r.Context(), ws, trimFormat(raw))
		if err != nil {
			httpErr(w, err)
			return
		}
		writePayload(w, r,
			styleXML{Name: st.Name, Format: st.Format, Filename: st.Filename},
			map[string]any{"style": map[string]any{"name": st.Name,
				"format": st.Format, "filename": st.Filename}})
	}
}

func (a *api) getStyleWS(w http.ResponseWriter, r *http.Request) {
	a.getStyle(r.PathValue("ws"))(w, r)
}

func (a *api) listStyles(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sts, err := a.s.ListStyles(r.Context(), ws)
		if err != nil {
			httpErr(w, err)
			return
		}
		type ref struct {
			Name string `json:"name" xml:"name"`
		}
		refs := []ref{}
		for _, st := range sts {
			refs = append(refs, ref{Name: st.Name})
		}
		type listXML struct {
			XMLName struct{} `xml:"styles"`
			Items   []ref    `xml:"style"`
		}
		writePayload(w, r, listXML{Items: refs},
			map[string]any{"styles": map[string]any{"style": refs}})
	}
}

func (a *api) updateStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := trimFormat(strings.TrimSuffix(r.PathValue("s"), ".sld"))
		cur, err := a.s.GetStyle(r.Context(), ws, name)
		if err != nil {
			httpErr(w, err)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cur.Body = string(body)
		if err := a.s.UpdateStyle(r.Context(), ws, name, cur); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.style.updated",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}

func (a *api) deleteStyle(ws string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := trimFormat(r.PathValue("s"))
		if err := a.s.DeleteStyle(r.Context(), ws, name); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.style.deleted",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}

func (a *api) createLayerGroup(w http.ResponseWriter, r *http.Request) {
	ws := r.PathValue("ws") // empty for global route
	var x lgXML
	var j struct {
		LayerGroup struct {
			Name   string `json:"name"`
			Mode   string `json:"mode"`
			Layers struct {
				Layer []string `json:"layer"`
			} `json:"layers"`
		} `json:"layerGroup"`
	}
	if err := readPayload(r, &x, &j); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lg := model.LayerGroup{Workspace: ws, Name: x.Name, Mode: x.Mode, Layers: x.Layers}
	if j.LayerGroup.Name != "" {
		lg.Name, lg.Mode, lg.Layers = j.LayerGroup.Name, j.LayerGroup.Mode, j.LayerGroup.Layers.Layer
	}
	if lg.Mode == "" {
		lg.Mode = "SINGLE"
	}
	if lg.Name == "" {
		http.Error(w, "layerGroup name required", http.StatusBadRequest)
		return
	}
	if err := a.s.CreateLayerGroup(r.Context(), lg); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layergroup.created",
		map[string]string{"name": lg.Name, "workspace": ws})
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(lg.Name))
}

func (a *api) getLayerGroup(w http.ResponseWriter, r *http.Request) {
	ws := r.PathValue("ws")
	name := trimFormat(r.PathValue("lg"))
	lg, err := a.s.GetLayerGroup(r.Context(), ws, name)
	if err != nil {
		httpErr(w, err)
		return
	}
	writePayload(w, r, lgXML{Name: lg.Name, Mode: lg.Mode, Layers: lg.Layers},
		map[string]any{"layerGroup": map[string]any{"name": lg.Name, "mode": lg.Mode,
			"layers": map[string]any{"layer": lg.Layers}}})
}

func (a *api) deleteLayerGroup(w http.ResponseWriter, r *http.Request) {
	ws := r.PathValue("ws")
	name := trimFormat(r.PathValue("lg"))
	if err := a.s.DeleteLayerGroup(r.Context(), ws, name); err != nil {
		httpErr(w, err)
		return
	}
	a.pub.Publish("catalog.layergroup.deleted",
		map[string]string{"name": name, "workspace": ws})
	w.WriteHeader(http.StatusOK)
}
