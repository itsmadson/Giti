// Command gateway is the Geoson OWS front door. Sprint 1: health endpoints only.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoson/geoson/libs/ogc-kit/health"
)

func newHandler() http.Handler {
	// No dependencies yet; checks are added as sprints wire in Postgres/Redis/NATS.
	return health.NewMux(map[string]health.Check{})
}

func main() {
	addr := os.Getenv("GEOSON_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	slog.Info("gateway listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler()); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
