package model

import "time"

type Kind string

const (
	KindAircraft Kind = "aircraft"
	KindVessel   Kind = "vessel"
)

type Position struct {
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	AltFt      *int      `json:"alt_ft,omitempty"`
	SpeedKt    *float64  `json:"speed_kt,omitempty"`
	HeadingDeg *float64  `json:"heading_deg,omitempty"`
	Time       time.Time `json:"time"`
}

type Airport struct {
	IATA string  `json:"iata"`
	ICAO string  `json:"icao"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

// Route is the scheduled origin and destination looked up by callsign; aircraft only.
type Route struct {
	From      Airport `json:"from"`
	To        Airport `json:"to"`
	Plausible bool    `json:"plausible"`
}

type Contact struct {
	ID          string     `json:"id"` // "a:<hex>" or "v:<mmsi>"
	Kind        Kind       `json:"kind"`
	Label       string     `json:"label"`
	Lat         float64    `json:"lat"`
	Lon         float64    `json:"lon"`
	AltFt       *int       `json:"alt_ft,omitempty"`
	SpeedKt     *float64   `json:"speed_kt,omitempty"`
	HeadingDeg  *float64   `json:"heading_deg,omitempty"`
	VSFpm       *int       `json:"vs_fpm,omitempty"`
	OnGround    bool       `json:"on_ground"`
	Type        string     `json:"type,omitempty"`
	Reg         string     `json:"reg,omitempty"`
	Squawk      string     `json:"squawk,omitempty"`
	Emergency   string     `json:"emergency,omitempty"`
	Category    string     `json:"category,omitempty"`
	NavStatus   *int       `json:"nav_status,omitempty"`
	Destination string     `json:"destination,omitempty"`
	ETA         *time.Time `json:"eta,omitempty"` // vessel only, as reported by the crew
	Military    bool       `json:"military"`
	Route       *Route     `json:"route,omitempty"`
	Source      string     `json:"source"`
	Seen        time.Time  `json:"seen"`
	DistanceKm  float64    `json:"distance_km"`
}

func (c Contact) Position() Position {
	return Position{Lat: c.Lat, Lon: c.Lon, AltFt: c.AltFt, SpeedKt: c.SpeedKt, HeadingDeg: c.HeadingDeg, Time: c.Seen}
}

type SourceStatus struct {
	Name        string     `json:"name"`
	Enabled     bool       `json:"enabled"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	LastErrorAt *time.Time `json:"last_error_at,omitempty"`
	Polls       int64      `json:"polls,omitempty"`
	Messages    int64      `json:"messages,omitempty"`
	Reconnects  int64      `json:"reconnects,omitempty"`
	Errors      int64      `json:"errors"`
	LastCount   int        `json:"last_count,omitempty"`
}

type StatusReporter interface {
	Status() SourceStatus
}
