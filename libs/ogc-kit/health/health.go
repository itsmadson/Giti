// Package health implements the Giti project-wide health convention:
// GET /healthz  -> 200 "ok" (liveness, never checks dependencies)
// GET /readyz   -> 200/503 JSON {"status": ..., "checks": {...}} (readiness)
package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Check reports whether one dependency is usable.
type Check func(ctx context.Context) error

// NewMux returns a mux serving /healthz and /readyz over the given checks.
func NewMux(checks map[string]Check) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		status := "ready"
		code := http.StatusOK
		results := make(map[string]string, len(checks))
		for name, check := range checks {
			if err := check(ctx); err != nil {
				results[name] = err.Error()
				status = "unready"
				code = http.StatusServiceUnavailable
			} else {
				results[name] = "ok"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{"status": status, "checks": results})
	})
	return mux
}

// Serve runs an HTTP server until ctx is cancelled, then drains gracefully.
func Serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
