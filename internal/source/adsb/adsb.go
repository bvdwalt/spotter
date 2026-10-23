// Package adsb polls a readsb-style aggregator (adsb.lol, airplanes.live) for aircraft near a point.
package adsb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

const (
	maxBackoff      = 60 * time.Second
	maxRetryAfter   = 10 * time.Minute
	defaultRetry429 = 30 * time.Second
)

// RateLimitedError carries the server's requested pause after an HTTP 429.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e RateLimitedError) Error() string {
	return "rate limited (429), retrying in " + e.RetryAfter.Round(time.Second).String()
}

type Source struct {
	BaseURL  string
	Lat, Lon float64
	RadiusNM float64
	Interval time.Duration
	Client   *http.Client

	mu     sync.Mutex
	status model.SourceStatus
}

func New(baseURL string, lat, lon, radiusNM float64, interval time.Duration) *Source {
	name := baseURL
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		name = u.Host
	}
	return &Source{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Lat:      lat,
		Lon:      lon,
		RadiusNM: radiusNM,
		Interval: interval,
		Client:   &http.Client{Timeout: 15 * time.Second},
		status:   model.SourceStatus{Name: name, Enabled: true},
	}
}

func (s *Source) Name() string { return s.status.Name }

func (s *Source) Status() model.SourceStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run polls until ctx is done, backing off on errors up to maxBackoff.
func (s *Source) Run(ctx context.Context, out chan<- model.Contact) error {
	wait := s.Interval
	for {
		contacts, err := s.Poll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.recordError(err)
			var rl RateLimitedError
			if errors.As(err, &rl) {
				wait = rl.RetryAfter
			} else {
				wait = min(wait*2, maxBackoff)
			}
			slog.Warn("adsb poll failed", "source", s.Name(), "err", err, "retry_in", wait)
		} else {
			s.recordSuccess(len(contacts))
			for _, c := range contacts {
				select {
				case out <- c:
				case <-ctx.Done():
					return nil
				}
			}
			wait = s.Interval
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

func (s *Source) Poll(ctx context.Context) ([]model.Contact, error) {
	u := fmt.Sprintf("%s/point/%s/%s/%d", s.BaseURL, fmtCoord(s.Lat), fmtCoord(s.Lon), int(math.Round(s.RadiusNM)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "spotter (github.com/bvdwalt/spotter)")
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, RateLimitedError{RetryAfter: retryAfter(resp.Header, max(s.Interval, defaultRetry429))}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return Parse(json.NewDecoder(resp.Body), s.Name(), time.Now())
}

// retryAfter reads Retry-After (seconds) or x-ratelimit-reset, bounded to something sane.
func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	for _, key := range []string{"Retry-After", "X-Ratelimit-Reset"} {
		if secs, err := strconv.ParseFloat(strings.TrimSpace(h.Get(key)), 64); err == nil && secs > 0 {
			return min(max(time.Duration(secs*float64(time.Second)), fallback), maxRetryAfter)
		}
	}
	return fallback
}

func fmtCoord(f float64) string { return fmt.Sprintf("%.4f", f) }

func (s *Source) recordSuccess(n int) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Polls++
	s.status.LastSuccess = &now
	s.status.LastCount = n
}

func (s *Source) recordError(err error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Polls++
	s.status.Errors++
	s.status.LastError = err.Error()
	s.status.LastErrorAt = &now
}

type response struct {
	Now      float64    `json:"now"`
	AC       []aircraft `json:"ac"`
	Aircraft []aircraft `json:"aircraft"` // adsb.fi uses this key
	Error    string     `json:"error"`
}

type aircraft struct {
	Hex       string          `json:"hex"`
	Flight    string          `json:"flight"`
	Reg       string          `json:"r"`
	Type      string          `json:"t"`
	Lat       *float64        `json:"lat"`
	Lon       *float64        `json:"lon"`
	AltBaro   json.RawMessage `json:"alt_baro"` // number, or the string "ground"
	GS        *float64        `json:"gs"`
	Track     *float64        `json:"track"`
	BaroRate  *float64        `json:"baro_rate"`
	Squawk    string          `json:"squawk"`
	Emergency string          `json:"emergency"`
	Category  string          `json:"category"`
	SeenPos   *float64        `json:"seen_pos"`
	Seen      *float64        `json:"seen"`
	DBFlags   int             `json:"dbFlags"`
}

// Parse converts an aggregator response into contacts. Aircraft without a position are skipped.
func Parse(dec *json.Decoder, source string, fallbackNow time.Time) ([]model.Contact, error) {
	var r response
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if r.Error != "" {
		return nil, fmt.Errorf("aggregator error: %s", r.Error)
	}
	now := fallbackNow
	if r.Now > 0 {
		if r.Now > 1e12 { // milliseconds
			now = time.UnixMilli(int64(r.Now))
		} else {
			now = time.Unix(0, int64(r.Now*1e9))
		}
	}
	list := r.AC
	if len(list) == 0 {
		list = r.Aircraft
	}
	out := make([]model.Contact, 0, len(list))
	for _, a := range list {
		if a.Lat == nil || a.Lon == nil || a.Hex == "" {
			continue
		}
		c := model.Contact{
			ID:         "a:" + a.Hex,
			Kind:       model.KindAircraft,
			Lat:        *a.Lat,
			Lon:        *a.Lon,
			SpeedKt:    a.GS,
			HeadingDeg: a.Track,
			Type:       a.Type,
			Reg:        a.Reg,
			Squawk:     a.Squawk,
			Category:   a.Category,
			Military:   a.DBFlags&1 == 1,
			Source:     source,
			Seen:       now,
		}
		if a.Emergency != "" && a.Emergency != "none" {
			c.Emergency = a.Emergency
		}
		if a.SeenPos != nil {
			c.Seen = now.Add(-time.Duration(*a.SeenPos * float64(time.Second)))
		}
		switch label := strings.TrimSpace(a.Flight); {
		case label != "":
			c.Label = label
		case a.Reg != "":
			c.Label = a.Reg
		default:
			c.Label = strings.ToUpper(a.Hex)
		}
		if string(a.AltBaro) == `"ground"` {
			c.OnGround = true
		} else if len(a.AltBaro) > 0 {
			var alt float64
			if err := json.Unmarshal(a.AltBaro, &alt); err == nil {
				v := int(math.Round(alt))
				c.AltFt = &v
			}
		}
		if a.BaroRate != nil {
			v := int(math.Round(*a.BaroRate))
			c.VSFpm = &v
		}
		out = append(out, c)
	}
	return out, nil
}
