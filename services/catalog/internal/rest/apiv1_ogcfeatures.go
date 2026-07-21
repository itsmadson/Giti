package rest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/giti/giti/services/catalog/internal/store"
)

// OGC API - Features (subset) served under /api/v1/ogc/features.

func (a *api) ogcLanding(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"title":       "Giti OGC API - Features",
		"description": "Feature access for published Giti layers",
		"links": []map[string]string{
			{"rel": "data", "href": "/api/v1/ogc/features/collections"},
			{"rel": "conformance", "href": "/api/v1/ogc/features/conformance"},
		},
	})
}

func (a *api) ogcConformance(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"conformsTo": []string{
		"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
		"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
	}})
}

func (a *api) ogcCollections(w http.ResponseWriter, r *http.Request) {
	layers, err := a.s.ListLayers(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	type coll struct {
		ID    string              `json:"id"`
		Title string              `json:"title"`
		Links []map[string]string `json:"links"`
	}
	out := []coll{}
	for _, l := range layers {
		id := l.Workspace + ":" + l.Name
		out = append(out, coll{ID: id, Title: id, Links: []map[string]string{
			{"rel": "items", "href": "/api/v1/ogc/features/collections/" + id + "/items"},
		}})
	}
	writeJSON(w, map[string]any{"collections": out})
}

func (a *api) ogcItems(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ws, name, ok := strings.Cut(id, ":")
	if !ok {
		http.Error(w, "collection id must be workspace:layer", http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	bbox := store.ParseBbox(q.Get("bbox"))
	// filter: CQL2/CQL text via ?filter= (filter-lang cql2-text | cql-text)
	cql := q.Get("filter")
	if limit <= 0 {
		limit = 1000
	}
	gj, err := a.s.FeaturesGeoJSON(r.Context(), ws, name, limit, offset, bbox, cql)
	if err != nil {
		httpErr(w, err)
		return
	}
	// add OGC API-Features links (self/next/prev) + timeStamp
	var fc map[string]any
	if json.Unmarshal(gj, &fc) == nil {
		self := r.URL.String()
		links := []map[string]string{
			{"rel": "self", "type": "application/geo+json", "href": self},
		}
		ret, _ := fc["numberReturned"].(float64)
		if int(ret) >= limit {
			links = append(links, map[string]string{"rel": "next", "type": "application/geo+json",
				"href": withOffset(r.URL, offset+limit)})
		}
		if offset > 0 {
			prev := offset - limit
			if prev < 0 {
				prev = 0
			}
			links = append(links, map[string]string{"rel": "prev", "type": "application/geo+json",
				"href": withOffset(r.URL, prev)})
		}
		fc["links"] = links
		fc["timeStamp"] = time.Now().UTC().Format(time.RFC3339)
		if b, err := json.Marshal(fc); err == nil {
			gj = b
		}
	}
	w.Header().Set("Content-Type", "application/geo+json")
	w.Write(gj)
}

func withOffset(u *url.URL, offset int) string {
	q := u.Query()
	q.Set("offset", strconv.Itoa(offset))
	c := *u
	c.RawQuery = q.Encode()
	return c.String()
}
