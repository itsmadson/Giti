package rest

import (
	"io"
	"net/http"
)

func (a *api) v1GetSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := a.s.GetSettings(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(cfg)
}

func (a *api) v1SetSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if err := a.s.SetSettings(r.Context(), body); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
