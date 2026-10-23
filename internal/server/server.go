package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bvdwalt/spotter/internal/alert"
	"github.com/bvdwalt/spotter/internal/model"
	"github.com/bvdwalt/spotter/internal/track"
	"github.com/bvdwalt/spotter/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type ClientConfig struct {
	HomeLat         float64 `json:"home_lat"`
	HomeLon         float64 `json:"home_lon"`
	RadiusNM        float64 `json:"radius_nm"`
	AircraftExpiryS float64 `json:"aircraft_expiry_s"`
	VesselExpiryS   float64 `json:"vessel_expiry_s"`
	TrackWindowS    float64 `json:"track_window_s"`
	PollIntervalS   float64 `json:"poll_interval_s"`
}

type Deps struct {
	Store   *track.Store
	Alerts  *alert.Engine
	Sources []model.StatusReporter
	Config  ClientConfig
	Version string
}

func New(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)

	h := &handlers{d}
	r.Get("/health", healthHandler)
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.NoCache)
		r.Get("/contacts", h.contacts)
		r.Get("/contacts/{id}/track", h.track)
		r.Get("/status", h.status)
		r.Get("/alerts", h.alerts)
		r.Get("/config", h.config)
	})
	r.Handle("/*", spaHandler())
	return r
}

func spaHandler() http.Handler {
	sub, err := fs.Sub(web.FS, "dist")
	if err != nil {
		panic("failed to sub static files: " + err.Error())
	}
	fsHandler := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fs.FS.Open requires paths without a leading slash.
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "."
		}

		if f, err := sub.Open(name); err == nil {
			if err := f.Close(); err != nil {
				slog.Warn("failed to close static file probe", "name", name, "err", err)
			}
			// Cache hashed assets indefinitely; everything else no-cache.
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fsHandler.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routes.
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fsHandler.ServeHTTP(w, r)
	})
}
