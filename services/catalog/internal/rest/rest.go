// Package rest implements the GeoServer-compatible /rest configuration API.
package rest

import (
	"errors"
	"net/http"

	"github.com/giti/giti/services/catalog/internal/store"
)

type Publisher interface {
	Publish(subject string, payload any)
}

type api struct {
	s   *store.Store
	pub Publisher
}

// Mount registers all /rest handlers under both /rest/ and /giti/rest/.
func Mount(mux *http.ServeMux, s *store.Store, pub Publisher) {
	a := &api{s: s, pub: pub}
	inner := http.NewServeMux()
	a.workspaceRoutes(inner)
	a.storeRoutes(inner)
	a.layerRoutes(inner)
	a.apiV1Routes(inner)
	mux.Handle("/rest/", inner)
	mux.Handle("/api/v1/", inner)
	mux.Handle("/giti/rest/", http.StripPrefix("/giti", inner))
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
