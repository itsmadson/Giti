// Command catalog is the Giti configuration system of record.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/giti/giti/libs/ogc-kit/health"
	"github.com/giti/giti/services/catalog/internal/events"
	"github.com/giti/giti/services/catalog/internal/rest"
	"github.com/giti/giti/services/catalog/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type deps struct {
	db  *pgxpool.Pool
	nc  *nats.Conn
	pub rest.Publisher
}

type noopPub struct{}

func (noopPub) Publish(subject string, payload any) {}

func newHandler(d deps) http.Handler {
	checks := map[string]health.Check{}
	if d.db != nil {
		checks["postgres"] = func(ctx context.Context) error { return d.db.Ping(ctx) }
	}
	if d.nc != nil {
		checks["nats"] = func(ctx context.Context) error {
			if !d.nc.IsConnected() {
				return context.DeadlineExceeded
			}
			return nil
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(checks))
	mux.Handle("/readyz", health.NewMux(checks))
	if d.db != nil {
		pub := d.pub
		if pub == nil {
			pub = noopPub{}
		}
		rest.Mount(mux, store.New(d.db), pub)
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
	var d deps
	if dsn := os.Getenv("GITI_DATABASE_URL"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			slog.Error("postgres connect", "err", err)
			os.Exit(1)
		}
		if err := store.Migrate(ctx, pool); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
		d.db = pool
	}
	if url := os.Getenv("GITI_NATS_URL"); url != "" {
		nc, err := nats.Connect(url)
		if err != nil {
			slog.Error("nats connect", "err", err)
			os.Exit(1)
		}
		d.nc = nc
		d.pub = events.NewNATS(nc)
	}
	slog.Info("catalog listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(d)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
