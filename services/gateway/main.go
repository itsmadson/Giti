// Command gateway is the Giti OWS front door. Sprint 1: health endpoints only.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/giti/giti/libs/ogc-kit/health"
)

func newHandler() http.Handler {
	return newHandlerWith(newBackends(os.Getenv))
}

func newHandlerWith(b backends) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/readyz", health.NewMux(map[string]health.Check{}))
	mux.Handle("/metrics", metricsHandler())
	limit, _ := strconv.ParseFloat(os.Getenv("GITI_RATE_LIMIT"), 64)
	burst, _ := strconv.Atoi(os.Getenv("GITI_RATE_BURST"))
	if burst == 0 {
		burst = int(limit * 2)
	}
	mux.Handle("/giti/",
		rateLimitMiddleware(limit, burst,
			metricsMiddleware(
				authzMiddleware(os.Getenv("GITI_AUTH_URL"), newDispatcher(b)))))
	return mux
}

func main() {
	addr := os.Getenv("GITI_HTTP_ADDR")
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
