package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
	"github.com/bvdwalt/spotter/internal/track"
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
	positions, live := h.Store.Track(id)
	if h.History != nil {
		stored, err := h.History.Track(r.Context(), id, time.Now().Add(-h.TrackHistory))
		if err != nil {
			slog.Warn("history track read failed", "id", id, "err", err)
		} else if len(stored) > 0 {
			// Stored rows lag the live track by up to one flush, so add the newer in-memory points.
			last := stored[len(stored)-1].Time
			for _, p := range positions {
				if p.Time.After(last) {
					stored = append(stored, p)
				}
			}
			positions, live = stored, true
		}
	}
	if !live {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown contact"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"legs": track.Legs(positions)})
}

// photo returns the first planespotters photo for an aircraft; vessels and unknown hexes get 404.
func (h *handlers) photo(w http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(chi.URLParam(r, "id"))
	if err != nil || !strings.HasPrefix(id, "a:") || h.Photos == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no photo"})
		return
	}
	p, err := h.Photos.Lookup(r.Context(), strings.TrimPrefix(id, "a:"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no photo"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, map[string]any{"photo": p})
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

func (h *handlers) alerts(w http.ResponseWriter, r *http.Request) {
	recent := h.Alerts.Recent()
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": recent})
}

func (h *handlers) clearAlerts(w http.ResponseWriter, r *http.Request) {
	h.Alerts.Clear()
	w.WriteHeader(http.StatusNoContent)
}

// dismissAlert removes one alert; the ?at= query carries the alert's RFC 3339 time.
func (h *handlers) dismissAlert(w http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad contact id"})
		return
	}
	at, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("at"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad or missing at"})
		return
	}
	if !h.Alerts.Dismiss(chi.URLParam(r, "rule"), id, at) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown alert"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) watchList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"watch": h.Alerts.WatchList()})
}

func (h *handlers) watch(w http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(chi.URLParam(r, "id"))
	if err != nil || id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad contact id"})
		return
	}
	h.Alerts.Watch(id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) unwatch(w http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(chi.URLParam(r, "id"))
	if err != nil || id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad contact id"})
		return
	}
	h.Alerts.Unwatch(id)
	w.WriteHeader(http.StatusNoContent)
}

// metrics writes Prometheus text format by hand; the counters are few enough that a client library is not worth it.
func (h *handlers) metrics(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	write := func(name, help, kind string, rows ...string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
		for _, row := range rows {
			b.WriteString(row + "\n")
		}
	}
	var contacts []string
	for kind, n := range h.Store.Counts() {
		contacts = append(contacts, fmt.Sprintf(`spotter_contacts{kind=%q} %d`, kind, n))
	}
	sort.Strings(contacts)
	write("spotter_contacts", "Live contacts by kind.", "gauge", contacts...)

	var polls, errs, msgs, reconnects, up []string
	for _, src := range h.Sources {
		st := src.Status()
		label := fmt.Sprintf(`{source=%q}`, st.Name)
		polls = append(polls, fmt.Sprintf("spotter_source_polls_total%s %d", label, st.Polls))
		errs = append(errs, fmt.Sprintf("spotter_source_errors_total%s %d", label, st.Errors))
		msgs = append(msgs, fmt.Sprintf("spotter_source_messages_total%s %d", label, st.Messages))
		reconnects = append(reconnects, fmt.Sprintf("spotter_source_reconnects_total%s %d", label, st.Reconnects))
		ok := 0
		if st.LastSuccess != nil && (st.LastErrorAt == nil || st.LastSuccess.After(*st.LastErrorAt)) {
			ok = 1
		}
		up = append(up, fmt.Sprintf("spotter_source_up%s %d", label, ok))
	}
	write("spotter_source_up", "1 when the source's last result was a success.", "gauge", up...)
	write("spotter_source_polls_total", "HTTP polls made by a source.", "counter", polls...)
	write("spotter_source_messages_total", "Stream messages received by a source.", "counter", msgs...)
	write("spotter_source_reconnects_total", "Stream reconnects by a source.", "counter", reconnects...)
	write("spotter_source_errors_total", "Failed polls or dropped streams by a source.", "counter", errs...)
	write("spotter_alerts_recent", "Alerts currently in the recent list.", "gauge", fmt.Sprintf("spotter_alerts_recent %d", len(h.Alerts.Recent())))
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func (h *handlers) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Config)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
