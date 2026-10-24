package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

const maxADSBPoints = 5

type Config struct {
	Port             string
	HomeLat          float64
	HomeLon          float64
	RadiusNM         float64
	ADSBBaseURL      string
	ADSBPoints       []model.Circle // query circles; defaults to one around home with RadiusNM
	ADSBPollInterval time.Duration
	AISStreamAPIKey  string
	AISBBox          [2][2]float64 // [[latSW, lonSW], [latNE, lonNE]]
	RouteAPIURL      string        // empty disables route lookups
	PhotoAPIURL      string        // empty disables aircraft photos
	BasemapPMTiles   string        // path to a PMTiles file served at /basemap.pmtiles; empty uses OpenFreeMap
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
		ADSBBaseURL:     getEnv("ADSB_BASE_URL", "https://opendata.adsb.fi/api/v2"),
		AISStreamAPIKey: os.Getenv("AISSTREAM_API_KEY"),
		RouteAPIURL:     getEnv("ROUTE_API_URL", "https://adsb.im/api/0/routeset"),
		PhotoAPIURL:     getEnv("PHOTO_API_URL", "https://api.planespotters.net/pub/photos/hex"),
		BasemapPMTiles:  os.Getenv("BASEMAP_PMTILES"),
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
	if cfg.ADSBPoints, err = pointsEnv("ADSB_POINTS", model.Circle{Lat: cfg.HomeLat, Lon: cfg.HomeLon, RadiusNM: cfg.RadiusNM}); err != nil {
		return nil, err
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
	if cfg.BasemapPMTiles != "" {
		if _, err := os.Stat(cfg.BasemapPMTiles); err != nil {
			return nil, fmt.Errorf("BASEMAP_PMTILES: %w", err)
		}
	}
	if cfg.AISBBox, err = bboxEnv("AIS_BBOX", cfg.HomeLat, cfg.HomeLon, cfg.RadiusNM); err != nil {
		return nil, err
	}
	return cfg, nil
}

// pointsEnv parses "lat,lon,nm;lat,lon,nm" into query circles, or returns the single default circle when unset.
func pointsEnv(key string, def model.Circle) ([]model.Circle, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return []model.Circle{def}, nil
	}
	var out []model.Circle
	for _, part := range strings.Split(v, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, ",")
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s: %q must be lat,lon,nm", key, part)
		}
		var n [3]float64
		for i, f := range fields {
			x, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
			if err != nil {
				return nil, fmt.Errorf("%s: %q: %w", key, part, err)
			}
			n[i] = x
		}
		c := model.Circle{Lat: n[0], Lon: n[1], RadiusNM: n[2]}
		switch {
		case c.Lat < -90 || c.Lat > 90 || c.Lon < -180 || c.Lon > 180:
			return nil, fmt.Errorf("%s: %q is not a valid position", key, part)
		case c.RadiusNM <= 0 || c.RadiusNM > 250:
			return nil, fmt.Errorf("%s: %q radius must be between 1 and 250 nm", key, part)
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return []model.Circle{def}, nil
	}
	if len(out) > maxADSBPoints {
		return nil, fmt.Errorf("%s: at most %d circles, got %d", key, maxADSBPoints, len(out))
	}
	return out, nil
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
