// Command auth is the Geoson authentication and authorization service.
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
	"github.com/redis/go-redis/v9"
)

type deps struct {
	db           *pgxpool.Pool
	rdb          *redis.Client
	secret       []byte
	defaultAllow bool
}

func newHandler(d deps) http.Handler {
	checks := map[string]health.Check{}
	if d.db != nil {
		checks["postgres"] = func(ctx context.Context) error { return d.db.Ping(ctx) }
	}
	if d.rdb != nil {
		checks["redis"] = func(ctx context.Context) error { return d.rdb.Ping(ctx).Err() }
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewMux(checks))
	mux.Handle("/readyz", health.NewMux(checks))
	// api.Mount is wired here once storage exists (Task 5).
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	addr := os.Getenv("GEOSON_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	d := deps{
		secret:       []byte(os.Getenv("GEOSON_JWT_SECRET")),
		defaultAllow: os.Getenv("GEOSON_AUTH_DEFAULT") != "DENY",
	}
	if len(d.secret) == 0 {
		slog.Warn("GEOSON_JWT_SECRET not set; using insecure dev secret")
		d.secret = []byte("geoson-dev-secret")
	}
	if dsn := os.Getenv("GEOSON_DATABASE_URL"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			slog.Error("postgres connect", "err", err)
			os.Exit(1)
		}
		d.db = pool
	}
	if raddr := os.Getenv("GEOSON_REDIS_URL"); raddr != "" {
		d.rdb = redis.NewClient(&redis.Options{Addr: raddr})
	}
	slog.Info("auth listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(d)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
