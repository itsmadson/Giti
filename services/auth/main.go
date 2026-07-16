// Command auth is the Giti authentication and authorization service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/giti/giti/libs/ogc-kit/health"
	"github.com/giti/giti/services/auth/internal/api"
	"github.com/giti/giti/services/auth/internal/password"
	"github.com/giti/giti/services/auth/internal/store"
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
	if d.db != nil {
		api.Mount(mux, store.New(d.db), d.rdb, d.secret, d.defaultAllow)
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
	d := deps{
		secret:       []byte(os.Getenv("GITI_JWT_SECRET")),
		defaultAllow: os.Getenv("GITI_AUTH_DEFAULT") != "DENY",
	}
	if len(d.secret) == 0 {
		slog.Warn("GITI_JWT_SECRET not set; using insecure dev secret")
		d.secret = []byte("giti-dev-secret")
	}
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
		hash, err := password.Hash("geoserver")
		if err != nil {
			slog.Error("seed hash", "err", err)
			os.Exit(1)
		}
		if err := store.New(pool).SeedAdmin(ctx, hash); err != nil {
			slog.Error("seed admin", "err", err)
			os.Exit(1)
		}
		slog.Warn("default admin user active — change the password", "user", "admin")
		d.db = pool
	}
	if raddr := os.Getenv("GITI_REDIS_URL"); raddr != "" {
		d.rdb = redis.NewClient(&redis.Options{Addr: raddr})
	}
	slog.Info("auth listening", "addr", addr)
	if err := health.Serve(ctx, addr, newHandler(d)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
