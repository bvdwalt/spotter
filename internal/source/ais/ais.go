// Package ais streams vessel positions from AISStream.io over a websocket.
package ais

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	endpoint   = "wss://stream.aisstream.io/v0/stream"
	sourceName = "aisstream"
	maxBackoff = 60 * time.Second
)

// BBox is a bounding box as AISStream expects it: two [lat, lon] corners.
type BBox [2][2]float64

type Source struct {
	APIKey string
	BBox   BBox

	mu     sync.Mutex
	status model.SourceStatus
	static map[string]staticInfo // keyed by MMSI, survives between position reports
}

type staticInfo struct {
	name, shipType, destination string
	eta                         *time.Time
}

func New(apiKey string, bbox BBox) *Source {
	return &Source{
		APIKey: apiKey,
		BBox:   bbox,
		status: model.SourceStatus{Name: sourceName, Enabled: true},
		static: make(map[string]staticInfo),
	}
}

func (s *Source) Name() string { return sourceName }

func (s *Source) Status() model.SourceStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run keeps one subscription open until ctx is done, reconnecting with jittered backoff.
func (s *Source) Run(ctx context.Context, out chan<- model.Contact) error {
	wait := time.Second
	for {
		err := s.stream(ctx, out)
		if ctx.Err() != nil {
			return nil
		}
		s.recordError(err)
		slog.Warn("ais stream ended", "err", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait + rand.N(wait/2)):
		}
		wait = min(wait*2, maxBackoff)
	}
}

func (s *Source) stream(ctx context.Context, out chan<- model.Contact) error {
	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort close on the way out
	conn.SetReadLimit(1 << 20)

	sub := map[string]any{
		"APIKey":             s.APIKey,
		"BoundingBoxes":      []BBox{s.BBox},
		"FilterMessageTypes": []string{"PositionReport", "ShipStaticData"},
	}
	if err := wsjson.Write(ctx, conn, sub); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	s.recordConnect()

	for {
		var raw json.RawMessage
		if err := wsjson.Read(ctx, conn, &raw); err != nil {
			return fmt.Errorf("read: %w", err)
		}
		c, err := s.handle(raw, time.Now())
		if err != nil {
			return err
		}
		if c == nil {
			continue
		}
		select {
		case out <- *c:
		case <-ctx.Done():
			return nil
		}
	}
}

type envelope struct {
	MessageType string `json:"MessageType"`
	MetaData    struct {
		MMSI      json.Number `json:"MMSI"`
		ShipName  string      `json:"ShipName"`
		Latitude  float64     `json:"latitude"`
		Longitude float64     `json:"longitude"`
		TimeUTC   string      `json:"time_utc"`
	} `json:"MetaData"`
	Message struct {
		PositionReport *struct {
			Cog                float64 `json:"Cog"`
			Sog                float64 `json:"Sog"`
			TrueHeading        int     `json:"TrueHeading"`
			NavigationalStatus int     `json:"NavigationalStatus"`
		} `json:"PositionReport"`
		ShipStaticData *struct {
			Name        string `json:"Name"`
			Type        int    `json:"Type"`
			Destination string `json:"Destination"`
			Eta         struct {
				Month, Day, Hour, Minute int
			} `json:"Eta"`
		} `json:"ShipStaticData"`
	} `json:"Message"`
	Error string `json:"error"`
}

// handle parses one message. Static data updates the cache and returns nil; position reports return a contact.
func (s *Source) handle(raw json.RawMessage, now time.Time) (*model.Contact, error) {
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if e.Error != "" {
		return nil, errors.New("server: " + e.Error)
	}
	s.recordMessage()
	mmsi := e.MetaData.MMSI.String()
	if mmsi == "" || mmsi == "0" {
		return nil, nil
	}
	name := strings.TrimSpace(e.MetaData.ShipName)

	switch e.MessageType {
	case "ShipStaticData":
		if d := e.Message.ShipStaticData; d != nil {
			info := staticInfo{name: name, shipType: ShipType(d.Type), destination: strings.TrimSpace(d.Destination)}
			info.eta = parseETA(d.Eta.Month, d.Eta.Day, d.Eta.Hour, d.Eta.Minute, now)
			if n := strings.TrimSpace(d.Name); n != "" {
				info.name = n
			}
			s.mu.Lock()
			s.static[mmsi] = info
			s.mu.Unlock()
		}
		return nil, nil
	case "PositionReport":
		p := e.Message.PositionReport
		if p == nil {
			return nil, nil
		}
		s.mu.Lock()
		info := s.static[mmsi]
		s.mu.Unlock()
		c := &model.Contact{
			ID:          "v:" + mmsi,
			Kind:        model.KindVessel,
			Label:       firstNonEmpty(info.name, name, mmsi),
			Lat:         e.MetaData.Latitude,
			Lon:         e.MetaData.Longitude,
			Type:        info.shipType,
			Destination: info.destination,
			ETA:         info.eta,
			Source:      sourceName,
			Seen:        parseTime(e.MetaData.TimeUTC, now),
		}
		sog := p.Sog
		c.SpeedKt = &sog
		if p.TrueHeading >= 0 && p.TrueHeading < 360 {
			h := float64(p.TrueHeading)
			c.HeadingDeg = &h
		} else if p.Cog >= 0 && p.Cog < 360 {
			cog := p.Cog
			c.HeadingDeg = &cog
		}
		nav := p.NavigationalStatus
		c.NavStatus = &nav
		return c, nil
	}
	return nil, nil
}

// parseETA turns the AIS month/day/hour/minute ETA into a time in UTC. AIS sends 0/0/24/60 for "unavailable".
// The year is not transmitted; an ETA more than a month in the past is taken to mean next year.
func parseETA(month, day, hour, minute int, now time.Time) *time.Time {
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59 {
		return nil
	}
	t := time.Date(now.UTC().Year(), time.Month(month), day, hour, minute, 0, 0, time.UTC)
	if t.Before(now.Add(-30 * 24 * time.Hour)) {
		t = t.AddDate(1, 0, 0)
	}
	return &t
}

// parseTime handles AISStream's "2006-01-02 15:04:05.999999999 -0700 UTC" timestamps.
func parseTime(s string, fallback time.Time) time.Time {
	if t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", s); err == nil {
		return t
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ShipType maps an AIS ship type code to a short description.
func ShipType(code int) string {
	switch {
	case code == 0:
		return ""
	case code >= 20 && code <= 29:
		return "WIG"
	case code == 30:
		return "Fishing"
	case code == 31 || code == 32:
		return "Towing"
	case code == 33:
		return "Dredger"
	case code == 34:
		return "Diving"
	case code == 35:
		return "Military"
	case code == 36:
		return "Sailing"
	case code == 37:
		return "Pleasure"
	case code >= 40 && code <= 49:
		return "High speed"
	case code == 50:
		return "Pilot"
	case code == 51:
		return "SAR"
	case code == 52:
		return "Tug"
	case code == 53:
		return "Port tender"
	case code == 55:
		return "Law enforcement"
	case code >= 60 && code <= 69:
		return "Passenger"
	case code >= 70 && code <= 79:
		return "Cargo"
	case code >= 80 && code <= 89:
		return "Tanker"
	default:
		return "Type " + strconv.Itoa(code)
	}
}

func (s *Source) recordConnect() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Reconnects++
	s.status.LastSuccess = &now
}

func (s *Source) recordMessage() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Messages++
	s.status.LastSuccess = &now
}

func (s *Source) recordError(err error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Errors++
	s.status.LastError = err.Error()
	s.status.LastErrorAt = &now
}
