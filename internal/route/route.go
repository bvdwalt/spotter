// Package route resolves airline callsigns to origin and destination airports via the adsb.im route database.
package route

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

const (
	positiveTTL = 6 * time.Hour
	negativeTTL = time.Hour
	maxBatch    = 50
)

type Query struct {
	Callsign string
	Lat, Lon float64
}

type cached struct {
	route   *model.Route // nil when the database has no route for the callsign
	expires time.Time
}

type Resolver struct {
	URL    string
	Client *http.Client

	mu      sync.Mutex
	cache   map[string]cached
	pausedT time.Time // no upstream calls before this after a 429
}

func New(url string) *Resolver {
	return &Resolver{URL: url, Client: &http.Client{Timeout: 15 * time.Second}, cache: make(map[string]cached)}
}

// Lookup returns cached routes for the callsigns and fetches the ones that are missing or expired.
// Callsigns the database does not know are cached as nil so they are not asked again for a while.
func (r *Resolver) Lookup(ctx context.Context, queries []Query) map[string]*model.Route {
	now := time.Now()
	out := make(map[string]*model.Route, len(queries))
	var missing []Query
	r.mu.Lock()
	paused := now.Before(r.pausedT)
	for _, q := range queries {
		if q.Callsign == "" {
			continue
		}
		if c, ok := r.cache[q.Callsign]; ok && now.Before(c.expires) {
			out[q.Callsign] = c.route
		} else if !paused {
			missing = append(missing, q)
		}
	}
	r.mu.Unlock()

	for len(missing) > 0 {
		batch := missing
		if len(batch) > maxBatch {
			batch = missing[:maxBatch]
		}
		missing = missing[len(batch):]
		found, err := r.fetch(ctx, batch)
		if err != nil {
			slog.Warn("route lookup failed", "err", err, "callsigns", len(batch))
			return out
		}
		r.mu.Lock()
		for _, q := range batch {
			rt := found[q.Callsign]
			ttl := negativeTTL
			if rt != nil {
				ttl = positiveTTL
			}
			r.cache[q.Callsign] = cached{route: rt, expires: now.Add(ttl)}
			out[q.Callsign] = rt
		}
		r.mu.Unlock()
	}
	return out
}

type requestPlane struct {
	Callsign string  `json:"callsign"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

type responseRoute struct {
	Callsign     string `json:"callsign"`
	AirportCodes string `json:"airport_codes"`
	Plausible    bool   `json:"plausible"`
	Airports     []struct {
		IATA     string  `json:"iata"`
		ICAO     string  `json:"icao"`
		Name     string  `json:"name"`
		Location string  `json:"location"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
	} `json:"_airports"`
}

func (r *Resolver) fetch(ctx context.Context, batch []Query) (map[string]*model.Route, error) {
	planes := make([]requestPlane, len(batch))
	for i, q := range batch {
		planes[i] = requestPlane{Callsign: q.Callsign, Lat: q.Lat, Lng: q.Lon}
	}
	body, err := json.Marshal(map[string]any{"planes": planes})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "spotter (github.com/bvdwalt/spotter)")
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests {
		r.mu.Lock()
		r.pausedT = time.Now().Add(5 * time.Minute)
		r.mu.Unlock()
		return nil, fmt.Errorf("rate limited (429), pausing route lookups for 5m")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return Parse(json.NewDecoder(resp.Body))
}

// Parse reads a routeset response into routes keyed by callsign; unknown routes are absent.
func Parse(dec *json.Decoder) (map[string]*model.Route, error) {
	var rows []responseRoute
	if err := dec.Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := make(map[string]*model.Route, len(rows))
	for _, row := range rows {
		// Multi-leg routes list more airports; the first and last are what matters here.
		if row.AirportCodes == "unknown" || len(row.Airports) < 2 {
			continue
		}
		from, to := row.Airports[0], row.Airports[len(row.Airports)-1]
		out[row.Callsign] = &model.Route{
			From:      model.Airport{IATA: from.IATA, ICAO: from.ICAO, Name: from.Name, Lat: from.Lat, Lon: from.Lon},
			To:        model.Airport{IATA: to.IATA, ICAO: to.ICAO, Name: to.Name, Lat: to.Lat, Lon: to.Lon},
			Plausible: row.Plausible,
		}
	}
	return out, nil
}
