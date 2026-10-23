package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bvdwalt/spotter/internal/config"
	"github.com/bvdwalt/spotter/internal/model"
	"github.com/bvdwalt/spotter/internal/server"
	"github.com/bvdwalt/spotter/internal/source/adsb"
	"github.com/bvdwalt/spotter/internal/source/ais"
	"github.com/bvdwalt/spotter/internal/track"
)

var version = "dev"

func run(ctx context.Context, cfg *config.Config) error {
	store := track.New(track.Options{
		HomeLat: cfg.HomeLat, HomeLon: cfg.HomeLon,
		AircraftExpiry: cfg.AircraftExpiry, VesselExpiry: cfg.VesselExpiry,
		TrackWindow: cfg.TrackWindow, TrackMinInterval: cfg.TrackMinInterval,
	})

	adsbSource := adsb.New(cfg.ADSBBaseURL, cfg.HomeLat, cfg.HomeLon, cfg.RadiusNM, cfg.ADSBPollInterval)
	sources := []model.StatusReporter{adsbSource}

	updates := make(chan model.Contact, 256)
	go func() {
		if err := adsbSource.Run(ctx, updates); err != nil {
			slog.Error("adsb source stopped", "err", err)
		}
	}()
	if cfg.AISStreamAPIKey == "" {
		slog.Info("AISSTREAM_API_KEY not set, ships disabled")
	} else {
		aisSource := ais.New(cfg.AISStreamAPIKey, cfg.AISBBox)
		sources = append(sources, aisSource)
		slog.Info("ais enabled", "bbox", cfg.AISBBox)
		go func() {
			if err := aisSource.Run(ctx, updates); err != nil {
				slog.Error("ais source stopped", "err", err)
			}
		}()
	}
	go func() {
		sweep := time.NewTicker(5 * time.Second)
		defer sweep.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case c := <-updates:
				store.Upsert(c)
			case now := <-sweep.C:
				store.Sweep(now)
			}
		}
	}()

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: server.New(server.Deps{
			Store:   store,
			Sources: sources,
			Version: version,
			Config: server.ClientConfig{
				HomeLat: cfg.HomeLat, HomeLon: cfg.HomeLon, RadiusNM: cfg.RadiusNM,
				AircraftExpiryS: cfg.AircraftExpiry.Seconds(),
				VesselExpiryS:   cfg.VesselExpiry.Seconds(),
				TrackWindowS:    cfg.TrackWindow.Seconds(),
				PollIntervalS:   cfg.ADSBPollInterval.Seconds(),
			},
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server listening", "port", cfg.Port, "version", version, "adsb", adsbSource.Name())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}
