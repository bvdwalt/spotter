package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
	"github.com/go-chi/chi/v5"
)

type handlers struct {
	Deps
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) contacts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"now":      time.Now(),
		"contacts": h.Store.Snapshot(),
	})
}

func (h *handlers) track(w http.ResponseWriter, r *http.Request) {
	// chi returns the raw path segment, so "a%3A1" must be unescaped before lookup.
	id, err := url.PathUnescape(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad contact id"})
		return
	}
	positions, ok := h.Store.Track(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown contact"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"positions": positions})
}

func (h *handlers) status(w http.ResponseWriter, r *http.Request) {
	sources := make([]model.SourceStatus, 0, len(h.Sources))
	for _, s := range h.Sources {
		sources = append(sources, s.Status())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":  h.Version,
		"now":      time.Now(),
		"sources":  sources,
		"contacts": h.Store.Counts(),
	})
}

func (h *handlers) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Config)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
