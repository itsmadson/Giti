// Package api serves the convert HTTP endpoints (multipart import + SSE progress).
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/giti/giti/services/convert/internal/ingest"
)

// Mount registers the convert endpoints.
func Mount(mux *http.ServeMux, catalogURL, dataDir string) {
	h := &handler{catalogURL: catalogURL, dataDir: dataDir}
	mux.HandleFunc("POST /api/v1/convert/import", h.importFile)
	mux.HandleFunc("POST /api/v1/convert/upload", h.upload)
	mux.HandleFunc("POST /api/v1/convert/coverage", h.coverage)
	mux.HandleFunc("POST /api/v1/convert/cog", h.cog)
}

// sanitizeName keeps a safe base filename (no path traversal).
func sanitizeName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	var b strings.Builder
	for _, r := range name {
		if r == '.' || r == '_' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		out = "file"
	}
	return out
}

// upload stores a raw file on the shared data volume and returns its path, so
// the admin store wizard can register a file-backed store pointing at it.
func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()
	dir := filepath.Join(h.dataDir, "uploads")
	if err := os.MkdirAll(dir, 0o775); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := sanitizeName(header.Filename)
	// prefix with a timestamp to avoid collisions
	name = time.Now().UTC().Format("20060102T150405") + "_" + name
	full := filepath.Join(dir, name)
	dst, err := os.Create(full)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, io.LimitReader(file, 1<<30)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": full, "name": header.Filename})
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

// coverage stores an uploaded GeoTIFF on the shared volume and registers it as
// a coverage (coveragestore + coverage + RASTER layer) via the catalog.
func (h *handler) coverage(w http.ResponseWriter, r *http.Request) {
	ws := r.URL.Query().Get("workspace")
	if ws == "" {
		ws = "default"
	}
	srs := r.URL.Query().Get("srs")
	if srs == "" {
		srs = "EPSG:4326"
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()
	name := sanitizeName(strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename)))
	if q := r.URL.Query().Get("name"); q != "" {
		name = sanitizeName(q)
	}
	dir := filepath.Join(h.dataDir, "coverages")
	if err := os.MkdirAll(dir, 0o775); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	full := filepath.Join(dir, name+".tif")
	dst, err := os.Create(full)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, io.LimitReader(file, 2<<30)); err != nil {
		dst.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dst.Close()

	body, _ := json.Marshal(map[string]string{"workspace": ws, "name": name, "path": full, "srs": srs})
	resp, err := http.Post(h.catalogURL+"/api/v1/register-coverage", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "register: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		http.Error(w, "register failed: "+string(msg), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"workspace": ws, "layer": name})
}

// cog is a stub: full GeoTIFF→COG conversion lands in the raster driver pack (S12).
func (h *handler) cog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"pending","note":"COG conversion lands in the raster driver pack"}`))
}
