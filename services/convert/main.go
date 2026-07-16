// Command convert ingests spatial files and publishes them as layers.
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

func newHandler(catalogURL, dataDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/readyz", health.NewMux(map[string]health.Check{}))
	// api.Mount is wired in Task 6.
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	addr := os.Getenv("GEOSON_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	catalogURL := os.Getenv("GEOSON_CATALOG_URL")
	if catalogURL == "" {
		catalogURL = "http://catalog:8080"
	}
	dataDir := os.Getenv("GEOSON_DATA_DIR")
	if dataDir == "" {
		dataDir = "/var/lib/geoson/data"
	}
	slog.Info("convert listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(catalogURL, dataDir)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
