// Package api serves auth HTTP endpoints: login, authorization checks,
// and GeoServer-compatible security configuration REST.
package api

import (
	"context"
	"net/http"

	"github.com/geoson/geoson/services/auth/internal/store"
	"github.com/redis/go-redis/v9"
)

type api struct {
	s            *store.Store
	rdb          *redis.Client
	secret       []byte
	defaultAllow bool
}

func Mount(mux *http.ServeMux, s *store.Store, rdb *redis.Client, secret []byte, defaultAllow bool) {
	a := &api{s: s, rdb: rdb, secret: secret, defaultAllow: defaultAllow}
	a.checkRoutes(mux)
	a.securityRoutes(mux)
	a.geofenceRoutes(mux)
}

// bumpGen invalidates all cached authz decisions.
func (a *api) bumpGen(ctx context.Context) {
	if a.rdb != nil {
		a.rdb.Incr(ctx, "authz:gen")
	}
}
