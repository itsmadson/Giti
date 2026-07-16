// Command wps is the Geoson Web Processing Service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoson/geoson/libs/ogc-kit/health"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type deps struct {
	db  *pgxpool.Pool
	nc  *nats.Conn
	dir string
}

func newHandler(d deps) http.Handler {
	checks := map[string]health.Check{}
	if d.db != nil {
		checks["postgres"] = func(ctx context.Context) error { return d.db.Ping(ctx) }
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(checks))
	mux.Handle("/readyz", health.NewMux(checks))
	// wps.Mount is wired in Task 4 once handlers exist.
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	addr := os.Getenv("GEOSON_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	d := deps{dir: os.Getenv("GEOSON_WPS_RESULTS_DIR")}
	if d.dir == "" {
		d.dir = "/var/lib/geoson/wps"
	}
	if dsn := os.Getenv("GEOSON_DATABASE_URL"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			slog.Error("postgres connect", "err", err)
			os.Exit(1)
		}
		d.db = pool
	}
	if url := os.Getenv("GEOSON_NATS_URL"); url != "" {
		if nc, err := nats.Connect(url); err == nil {
			d.nc = nc
		}
	}
	slog.Info("wps listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(d)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
