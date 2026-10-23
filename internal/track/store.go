// Package track holds the in-memory set of live contacts and their recent positions.
package track

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

type Options struct {
	HomeLat, HomeLon float64
	AircraftExpiry   time.Duration
	VesselExpiry     time.Duration
	TrackWindow      time.Duration
	TrackMinInterval time.Duration
}

type entry struct {
	contact model.Contact
	track   []model.Position
}

type Store struct {
	opts Options
	mu   sync.RWMutex
	m    map[string]*entry
}

func New(opts Options) *Store {
	return &Store{opts: opts, m: make(map[string]*entry)}
}

// Upsert replaces the contact and appends its position to the track if enough time has passed.
// It returns the stored contact, with distance and carried-over static fields filled in.
func (s *Store) Upsert(c model.Contact) model.Contact {
	c.DistanceKm = haversineKm(s.opts.HomeLat, s.opts.HomeLon, c.Lat, c.Lon)
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[c.ID]
	if !ok {
		e = &entry{}
		s.m[c.ID] = e
	}
	// Static fields from a previous sighting survive a source that only sends positions.
	if c.Label == "" {
		c.Label = e.contact.Label
	}
	if c.Type == "" {
		c.Type = e.contact.Type
	}
	if c.Destination == "" {
		c.Destination = e.contact.Destination
	}
	e.contact = c
	if n := len(e.track); n == 0 || c.Seen.Sub(e.track[n-1].Time) >= s.opts.TrackMinInterval {
		e.track = append(e.track, c.Position())
	}
	cutoff := c.Seen.Add(-s.opts.TrackWindow)
	i := 0
	for i < len(e.track) && e.track[i].Time.Before(cutoff) {
		i++
	}
	e.track = e.track[i:]
	return c
}

// Sweep drops contacts whose last sighting is older than their kind's expiry.
func (s *Store) Sweep(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, e := range s.m {
		expiry := s.opts.AircraftExpiry
		if e.contact.Kind == model.KindVessel {
			expiry = s.opts.VesselExpiry
		}
		if now.Sub(e.contact.Seen) > expiry {
			delete(s.m, id)
			removed++
		}
	}
	return removed
}

func (s *Store) Snapshot() []model.Contact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Contact, 0, len(s.m))
	for _, e := range s.m {
		out = append(out, e.contact)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Track(id string) ([]model.Position, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.m[id]
	if !ok {
		return nil, false
	}
	out := make([]model.Position, len(e.track))
	copy(out, e.track)
	return out, true
}

func (s *Store) Counts() map[model.Kind]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := map[model.Kind]int{}
	for _, e := range s.m {
		counts[e.contact.Kind]++
	}
	return counts
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}
