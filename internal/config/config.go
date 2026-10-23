package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
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
	AISBBox          [2][2]float64 // [[latSW, lonSW], [latNE, lonNE]]
	NtfyURL          string
	NtfyToken        string
	WatchList        []string
	AlertCooldown    time.Duration
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
		NtfyURL:         os.Getenv("NTFY_URL"),
		NtfyToken:       os.Getenv("NTFY_TOKEN"),
	}
	if v := os.Getenv("WATCH_LIST"); v != "" {
		cfg.WatchList = strings.Split(v, ",")
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
		{&cfg.AlertCooldown, "ALERT_COOLDOWN", time.Hour},
	}
	for _, d := range durations {
		if *d.dst, err = durationEnv(d.key, d.def); err != nil {
			return nil, err
		}
	}
	if cfg.AISBBox, err = bboxEnv("AIS_BBOX", cfg.HomeLat, cfg.HomeLon, cfg.RadiusNM); err != nil {
		return nil, err
	}
	return cfg, nil
}

// bboxEnv parses "latSW,lonSW,latNE,lonNE" or derives a box spanning the radius around home.
func bboxEnv(key string, lat, lon, radiusNM float64) ([2][2]float64, error) {
	var box [2][2]float64
	v := os.Getenv(key)
	if v == "" {
		dLat := radiusNM / 60
		dLon := dLat / math.Max(math.Cos(lat*math.Pi/180), 0.01)
		return [2][2]float64{{lat - dLat, lon - dLon}, {lat + dLat, lon + dLon}}, nil
	}
	parts := strings.Split(v, ",")
	if len(parts) != 4 {
		return box, fmt.Errorf("%s must be latSW,lonSW,latNE,lonNE", key)
	}
	var vals [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return box, fmt.Errorf("%s: %w", key, err)
		}
		vals[i] = f
	}
	if vals[0] >= vals[2] || vals[1] >= vals[3] {
		return box, fmt.Errorf("%s: south-west corner must be below and left of north-east", key)
	}
	return [2][2]float64{{vals[0], vals[1]}, {vals[2], vals[3]}}, nil
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
