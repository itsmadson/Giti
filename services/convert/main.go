// Command convert ingests spatial files and publishes them as layers.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/giti/giti/libs/ogc-kit/health"
	"github.com/giti/giti/services/convert/internal/api"
)

func newHandler(catalogURL, dataDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/readyz", health.NewMux(map[string]health.Check{}))
	api.Mount(mux, catalogURL, dataDir)
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	addr := os.Getenv("GITI_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	catalogURL := os.Getenv("GITI_CATALOG_URL")
	if catalogURL == "" {
		catalogURL = "http://catalog:8080"
	}
	dataDir := os.Getenv("GITI_DATA_DIR")
	if dataDir == "" {
		dataDir = "/var/lib/giti/data"
	}
	slog.Info("convert listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(catalogURL, dataDir)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
