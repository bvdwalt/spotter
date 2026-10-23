package track

import (
	"testing"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

func newTestStore() *Store {
	return New(Options{
		HomeLat: 0, HomeLon: 0,
		AircraftExpiry: time.Minute, VesselExpiry: 15 * time.Minute,
		TrackWindow: 15 * time.Minute, TrackMinInterval: 10 * time.Second,
	})
}

func TestUpsertTrackDownsampleAndWindow(t *testing.T) {
	s := newTestStore()
	base := time.Unix(1_700_000_000, 0)
	for i := 0; i < 200; i++ {
		s.Upsert(model.Contact{ID: "a:1", Kind: model.KindAircraft, Lat: 1, Lon: float64(i) / 100, Seen: base.Add(time.Duration(i) * 5 * time.Second)})
	}
	track, ok := s.Track("a:1")
	if !ok {
		t.Fatal("missing track")
	}
	// 200 samples at 5 s = 1000 s span; 10 s min interval over a 15 min window -> about 90 points.
	if len(track) < 85 || len(track) > 92 {
		t.Errorf("track len = %d", len(track))
	}
	last := base.Add(199 * 5 * time.Second)
	if last.Sub(track[0].Time) > 15*time.Minute {
		t.Errorf("oldest point outside window: %v", last.Sub(track[0].Time))
	}
}

func TestSweepPerKind(t *testing.T) {
	s := newTestStore()
	now := time.Now()
	s.Upsert(model.Contact{ID: "a:1", Kind: model.KindAircraft, Seen: now.Add(-2 * time.Minute)})
	s.Upsert(model.Contact{ID: "v:1", Kind: model.KindVessel, Seen: now.Add(-2 * time.Minute)})
	s.Upsert(model.Contact{ID: "a:2", Kind: model.KindAircraft, Seen: now})
	if got := s.Sweep(now); got != 1 {
		t.Errorf("removed %d, want 1", got)
	}
	if len(s.Snapshot()) != 2 {
		t.Errorf("snapshot = %+v", s.Snapshot())
	}
}

func TestDistanceAndStaticCarryOver(t *testing.T) {
	s := newTestStore()
	s.Upsert(model.Contact{ID: "v:1", Kind: model.KindVessel, Label: "SHIP", Lat: 0, Lon: 1, Seen: time.Now()})
	s.Upsert(model.Contact{ID: "v:1", Kind: model.KindVessel, Lat: 0, Lon: 1, Seen: time.Now()})
	c := s.Snapshot()[0]
	if c.Label != "SHIP" {
		t.Errorf("label lost: %+v", c)
	}
	if c.DistanceKm < 111 || c.DistanceKm > 112 {
		t.Errorf("distance = %v", c.DistanceKm)
	}
}
