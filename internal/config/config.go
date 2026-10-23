package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	HomeLat          float64
	HomeLon          float64
	RadiusNM         float64
	ADSBBaseURL      string
	ADSBPollInterval time.Duration
	AISStreamAPIKey  string
	AircraftExpiry   time.Duration
	VesselExpiry     time.Duration
	TrackWindow      time.Duration
	TrackMinInterval time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:            getEnv("PORT", "8080"),
		ADSBBaseURL:     getEnv("ADSB_BASE_URL", "https://api.adsb.lol/v2"),
		AISStreamAPIKey: os.Getenv("AISSTREAM_API_KEY"),
	}
	var err error
	if cfg.HomeLat, err = requireFloat("HOME_LAT"); err != nil {
		return nil, err
	}
	if cfg.HomeLon, err = requireFloat("HOME_LON"); err != nil {
		return nil, err
	}
	if cfg.RadiusNM, err = floatEnv("RADIUS_NM", 150); err != nil {
		return nil, err
	}
	if cfg.RadiusNM <= 0 || cfg.RadiusNM > 250 {
		return nil, fmt.Errorf("RADIUS_NM must be between 1 and 250, got %v", cfg.RadiusNM)
	}
	durations := []struct {
		dst *time.Duration
		key string
		def time.Duration
	}{
		{&cfg.ADSBPollInterval, "ADSB_POLL_INTERVAL", 5 * time.Second},
		{&cfg.AircraftExpiry, "AIRCRAFT_EXPIRY", 60 * time.Second},
		{&cfg.VesselExpiry, "VESSEL_EXPIRY", 15 * time.Minute},
		{&cfg.TrackWindow, "TRACK_WINDOW", 15 * time.Minute},
		{&cfg.TrackMinInterval, "TRACK_MIN_INTERVAL", 10 * time.Second},
	}
	for _, d := range durations {
		if *d.dst, err = durationEnv(d.key, d.def); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireFloat(key string) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return f, nil
}

func floatEnv(key string, def float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return f, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return d, nil
}
