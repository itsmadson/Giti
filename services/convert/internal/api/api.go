// Package api serves the convert HTTP endpoints (multipart import + SSE progress).
package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/giti/giti/services/convert/internal/ingest"
)

// Mount registers the convert endpoints.
func Mount(mux *http.ServeMux, catalogURL, dataDir string) {
	h := &handler{catalogURL: catalogURL, dataDir: dataDir}
	mux.HandleFunc("POST /api/v1/convert/import", h.importFile)
	mux.HandleFunc("POST /api/v1/convert/cog", h.cog)
}

type handler struct {
	catalogURL, dataDir string
}

func (h *handler) importFile(w http.ResponseWriter, r *http.Request) {
	workspace := r.URL.Query().Get("workspace")
	if workspace == "" {
		workspace = "default"
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 512<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	send := func(v any) {
		buf, _ := json.Marshal(v)
		w.Write([]byte("data: "))
		w.Write(buf)
		w.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}

	res, err := ingest.Import(r.Context(), h.catalogURL, h.dataDir, workspace,
		header.Filename, data, func(step string) {
			send(map[string]string{"step": step})
		})
	if err != nil {
		send(map[string]any{"error": err.Error()})
		return
	}
	send(map[string]any{"done": true, "layer": res.Layer, "workspace": res.Workspace})
}

// cog is a stub: full GeoTIFF→COG conversion lands in the raster driver pack (S12).
func (h *handler) cog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"pending","note":"COG conversion lands in the raster driver pack"}`))
}
