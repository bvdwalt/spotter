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

type Contact struct {
	ID          string    `json:"id"` // "a:<hex>" or "v:<mmsi>"
	Kind        Kind      `json:"kind"`
	Label       string    `json:"label"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	AltFt       *int      `json:"alt_ft,omitempty"`
	SpeedKt     *float64  `json:"speed_kt,omitempty"`
	HeadingDeg  *float64  `json:"heading_deg,omitempty"`
	VSFpm       *int      `json:"vs_fpm,omitempty"`
	OnGround    bool      `json:"on_ground"`
	Type        string    `json:"type,omitempty"`
	Reg         string    `json:"reg,omitempty"`
	Squawk      string    `json:"squawk,omitempty"`
	Emergency   string    `json:"emergency,omitempty"`
	Category    string    `json:"category,omitempty"`
	NavStatus   *int      `json:"nav_status,omitempty"`
	Destination string    `json:"destination,omitempty"`
	Military    bool      `json:"military"`
	Source      string    `json:"source"`
	Seen        time.Time `json:"seen"`
	DistanceKm  float64   `json:"distance_km"`
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
	Polls       int64      `json:"polls"`
	Errors      int64      `json:"errors"`
	LastCount   int        `json:"last_count"`
}

type StatusReporter interface {
	Status() SourceStatus
}
