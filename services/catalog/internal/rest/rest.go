// Package rest implements the GeoServer-compatible /rest configuration API.
package rest

import (
	"errors"
	"net/http"

	"github.com/geoson/geoson/services/catalog/internal/store"
)

type Publisher interface {
	Publish(subject string, payload any)
}

type api struct {
	s   *store.Store
	pub Publisher
}

// Mount registers all /rest handlers under both /rest/ and /geoserver/rest/.
func Mount(mux *http.ServeMux, s *store.Store, pub Publisher) {
	a := &api{s: s, pub: pub}
	inner := http.NewServeMux()
	a.workspaceRoutes(inner)
	mux.Handle("/rest/", inner)
	mux.Handle("/geoserver/rest/", http.StripPrefix("/geoserver", inner))
}

// httpErr maps repository errors to GeoServer-compatible status codes.
func httpErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, store.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
