// Command wps is the Giti Web Processing Service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/giti/giti/libs/ogc-kit/health"
	"github.com/giti/giti/services/wps/internal/wps"
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
	if d.db != nil {
		jobs := wps.NewJobs(d.dir, d.nc, d.db)
		wps.Mount(mux, jobs)
		if d.nc != nil {
			go jobs.RunWorker(context.Background())
		}
	}
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	addr := os.Getenv("GITI_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	d := deps{dir: os.Getenv("GITI_WPS_RESULTS_DIR")}
	if d.dir == "" {
		d.dir = "/var/lib/giti/wps"
	}
	if dsn := os.Getenv("GITI_DATABASE_URL"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			slog.Error("postgres connect", "err", err)
			os.Exit(1)
		}
		d.db = pool
	}
	if url := os.Getenv("GITI_NATS_URL"); url != "" {
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
