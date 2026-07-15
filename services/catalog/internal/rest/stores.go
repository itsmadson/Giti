package rest

import (
	"net/http"

	"github.com/geoson/geoson/services/catalog/internal/connect"
	"github.com/geoson/geoson/services/catalog/internal/model"
)

// GeoServer connectionParameters wire formats.
type cpEntryXML struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}
type storeXML struct {
	XMLName     struct{}     `xml:"dataStore"`
	Name        string       `xml:"name"`
	Type        string       `xml:"type,omitempty"`
	Enabled     bool         `xml:"enabled"`
	Description string       `xml:"description,omitempty"`
	Params      []cpEntryXML `xml:"connectionParameters>entry"`
}
type covStoreXML struct {
	XMLName     struct{} `xml:"coverageStore"`
	Name        string   `xml:"name"`
	Type        string   `xml:"type,omitempty"`
	Enabled     bool     `xml:"enabled"`
	Description string   `xml:"description,omitempty"`
	URL         string   `xml:"url,omitempty"`
}
type cpEntryJSON struct {
	Key   string `json:"@key"`
	Value string `json:"$"`
}
type storeBodyJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Params      *struct {
		Entry []cpEntryJSON `json:"entry"`
	} `json:"connectionParameters,omitempty"`
}
type dsJSON struct {
	DataStore storeBodyJSON `json:"dataStore"`
}
type csJSON struct {
	CoverageStore storeBodyJSON `json:"coverageStore"`
}

func connToEntries(conn map[string]string) []cpEntryXML {
	out := make([]cpEntryXML, 0, len(conn))
	for k, v := range conn {
		out = append(out, cpEntryXML{Key: k, Value: v})
	}
	return out
}

func (a *api) storeRoutes(mux *http.ServeMux) {
	for _, kind := range []string{"datastores", "coveragestores"} {
		k := map[string]string{"datastores": "datastore", "coveragestores": "coveragestore"}[kind]
		mux.HandleFunc("GET /rest/workspaces/{ws}/"+kind, a.listStoresH(k))
		mux.HandleFunc("GET /rest/workspaces/{ws}/"+kind+".json", a.listStoresH(k))
		mux.HandleFunc("POST /rest/workspaces/{ws}/"+kind, a.createStoreH(k))
		mux.HandleFunc("POST /rest/workspaces/{ws}/"+kind+".json", a.createStoreH(k))
		mux.HandleFunc("GET /rest/workspaces/{ws}/"+kind+"/{name}", a.getStoreH(k))
		mux.HandleFunc("PUT /rest/workspaces/{ws}/"+kind+"/{name}", a.updateStoreH(k))
		mux.HandleFunc("DELETE /rest/workspaces/{ws}/"+kind+"/{name}", a.deleteStoreH(k))
	}
}

func (a *api) readStore(r *http.Request, kind string) (model.Store, error) {
	st := model.Store{Kind: kind, Enabled: true}
	if kind == "datastore" {
		var x storeXML
		var j dsJSON
		if err := readPayload(r, &x, &j); err != nil {
			return st, err
		}
		b := j.DataStore
		if b.Name == "" { // XML path
			st.Name, st.Type, st.Enabled, st.Description = x.Name, x.Type, x.Enabled, x.Description
			st.Connection = map[string]string{}
			for _, e := range x.Params {
				st.Connection[e.Key] = e.Value
			}
			return st, nil
		}
		st.Name, st.Type, st.Enabled, st.Description = b.Name, b.Type, b.Enabled, b.Description
		st.Connection = map[string]string{}
		if b.Params != nil {
			for _, e := range b.Params.Entry {
				st.Connection[e.Key] = e.Value
			}
		}
		return st, nil
	}
	var x covStoreXML
	var j csJSON
	if err := readPayload(r, &x, &j); err != nil {
		return st, err
	}
	b := j.CoverageStore
	if b.Name == "" {
		st.Name, st.Type, st.Enabled, st.Description = x.Name, x.Type, x.Enabled, x.Description
		st.Connection = map[string]string{"url": x.URL}
		return st, nil
	}
	st.Name, st.Type, st.Enabled, st.Description = b.Name, b.Type, b.Enabled, b.Description
	st.Connection = map[string]string{"url": b.URL}
	return st, nil
}

func (a *api) createStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws := r.PathValue("ws")
		st, err := a.readStore(r, kind)
		if err != nil || st.Name == "" {
			http.Error(w, "invalid store body", http.StatusBadRequest)
			return
		}
		st.Workspace = ws
		if c, cerr := connect.ForType(st.Type); cerr == nil {
			if verr := c.Validate(r.Context(), st); verr != nil {
				http.Error(w, "store validation failed: "+verr.Error(), http.StatusBadRequest)
				return
			}
		}
		if err := a.s.CreateStore(r.Context(), st); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.store.created",
			map[string]string{"name": st.Name, "workspace": ws})
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(st.Name))
	}
}

func (a *api) getStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, name := r.PathValue("ws"), trimFormat(r.PathValue("name"))
		st, err := a.s.GetStore(r.Context(), ws, name, kind)
		if err != nil {
			httpErr(w, err)
			return
		}
		writeStore(w, r, st)
	}
}

func writeStore(w http.ResponseWriter, r *http.Request, st model.Store) {
	body := storeBodyJSON{Name: st.Name, Type: st.Type, Enabled: st.Enabled, Description: st.Description}
	entries := []cpEntryJSON{}
	for k, v := range st.Connection {
		entries = append(entries, cpEntryJSON{Key: k, Value: v})
	}
	body.Params = &struct {
		Entry []cpEntryJSON `json:"entry"`
	}{Entry: entries}
	if st.Kind == "coveragestore" {
		body.URL = st.Connection["url"]
		writePayload(w, r, covStoreXML{Name: st.Name, Type: st.Type, Enabled: st.Enabled,
			Description: st.Description, URL: st.Connection["url"]}, csJSON{CoverageStore: body})
		return
	}
	writePayload(w, r, storeXML{Name: st.Name, Type: st.Type, Enabled: st.Enabled,
		Description: st.Description, Params: connToEntries(st.Connection)}, dsJSON{DataStore: body})
}

func (a *api) listStoresH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws := trimFormat(r.PathValue("ws"))
		list, err := a.s.ListStores(r.Context(), ws, kind)
		if err != nil {
			httpErr(w, err)
			return
		}
		type ref struct {
			Name string `json:"name" xml:"name"`
		}
		refs := []ref{}
		for _, st := range list {
			refs = append(refs, ref{Name: st.Name})
		}
		if kind == "coveragestore" {
			type listXML struct {
				XMLName struct{} `xml:"coverageStores"`
				Items   []ref    `xml:"coverageStore"`
			}
			writePayload(w, r, listXML{Items: refs},
				map[string]any{"coverageStores": map[string]any{"coverageStore": refs}})
			return
		}
		type listXML struct {
			XMLName struct{} `xml:"dataStores"`
			Items   []ref    `xml:"dataStore"`
		}
		writePayload(w, r, listXML{Items: refs},
			map[string]any{"dataStores": map[string]any{"dataStore": refs}})
	}
}

func (a *api) updateStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, name := r.PathValue("ws"), trimFormat(r.PathValue("name"))
		st, err := a.readStore(r, kind)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.s.UpdateStore(r.Context(), ws, name, st); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.store.updated",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}

func (a *api) deleteStoreH(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, name := r.PathValue("ws"), trimFormat(r.PathValue("name"))
		recurse := r.URL.Query().Get("recurse") == "true"
		if err := a.s.DeleteStore(r.Context(), ws, name, recurse); err != nil {
			httpErr(w, err)
			return
		}
		a.pub.Publish("catalog.store.deleted",
			map[string]string{"name": name, "workspace": ws})
		w.WriteHeader(http.StatusOK)
	}
}
