// Package adsb polls a readsb-style aggregator (adsb.fi, adsb.lol) for aircraft within one or more circles.
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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

const (
	maxBackoff = 60 * time.Second
	// circleSpacing keeps consecutive circle queries under adsb.fi's one request per second.
	circleSpacing   = 1100 * time.Millisecond
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
	Circles  []model.Circle
	Interval time.Duration
	Spacing  time.Duration // pause between circle queries within one poll
	Client   *http.Client

	mu     sync.Mutex
	status model.SourceStatus
}

func New(baseURL string, circles []model.Circle, interval time.Duration) *Source {
	name := baseURL
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		name = u.Host
	}
	return &Source{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Circles:  circles,
		Interval: interval,
		Spacing:  circleSpacing,
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

// Poll queries every circle in turn and merges the results by aircraft, keeping the freshest position.
// A local aircraft.json is fetched once and filtered to the circles instead.
func (s *Source) Poll(ctx context.Context) ([]model.Contact, error) {
	if s.isLocalFile() {
		contacts, err := s.fetch(ctx, s.BaseURL)
		if err != nil {
			return nil, err
		}
		return s.withinCircles(contacts), nil
	}
	byID := map[string]model.Contact{}
	for i, c := range s.Circles {
		if i > 0 && s.Spacing > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.Spacing):
			}
		}
		contacts, err := s.fetch(ctx, s.pointURL(c))
		if err != nil {
			return nil, err
		}
		for _, ct := range contacts {
			if prev, ok := byID[ct.ID]; !ok || ct.Seen.After(prev.Seen) {
				byID[ct.ID] = ct
			}
		}
	}
	out := make([]model.Contact, 0, len(byID))
	for _, ct := range byID {
		out = append(out, ct)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Source) fetch(ctx context.Context, u string) ([]model.Contact, error) {
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

// isLocalFile reports whether BaseURL points straight at a readsb/tar1090 aircraft.json,
// which returns everything the receiver hears rather than a point query.
func (s *Source) isLocalFile() bool {
	return strings.HasSuffix(s.BaseURL, ".json")
}

func (s *Source) withinCircles(contacts []model.Contact) []model.Contact {
	out := contacts[:0]
	for _, ct := range contacts {
		for _, c := range s.Circles {
			if haversineNM(c.Lat, c.Lon, ct.Lat, ct.Lon) <= c.RadiusNM {
				out = append(out, ct)
				break
			}
		}
	}
	return out
}

func haversineNM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthNM = 3440.065
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthNM * math.Asin(math.Sqrt(a))
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

// pointURL builds the query for one circle. adsb.fi uses a different path from the readsb-style aggregators.
func (s *Source) pointURL(c model.Circle) string {
	radius := int(math.Round(c.RadiusNM))
	if strings.Contains(s.BaseURL, "adsb.fi") {
		return fmt.Sprintf("%s/lat/%s/lon/%s/dist/%d", s.BaseURL, fmtCoord(c.Lat), fmtCoord(c.Lon), radius)
	}
	return fmt.Sprintf("%s/point/%s/%s/%d", s.BaseURL, fmtCoord(c.Lat), fmtCoord(c.Lon), radius)
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
