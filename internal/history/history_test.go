package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bvdwalt/spotter/internal/db"
	"github.com/bvdwalt/spotter/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	d, err := db.Connect(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return New(d, DefaultOptions(10*time.Second, 72*time.Hour))
}

var base = time.Unix(1_700_000_000, 0)

func contact(id string, kind model.Kind, at time.Duration, lat, lon float64) model.Contact {
	alt := 35000
	return model.Contact{ID: id, Kind: kind, Lat: lat, Lon: lon, AltFt: &alt, Seen: base.Add(at)}
}

func TestRecordRateLimitsMovingContact(t *testing.T) {
	s := newTestStore(t)
	// A plane moving about 1 km between 2 s updates for a minute: one row per 10 s.
	for i := 0; i <= 30; i++ {
		s.Record(contact("a:1", model.KindAircraft, time.Duration(i)*2*time.Second, -33, 18+float64(i)*0.01))
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 7 { // t = 0, 10, 20, 30, 40, 50, 60 s
		t.Fatalf("want 7 rows, got %d", len(got))
	}
	if got[0].AltFt == nil || *got[0].AltFt != 35000 || !got[0].Time.Equal(base) {
		t.Errorf("first row = %+v", got[0])
	}
	if !got[6].Time.Equal(base.Add(time.Minute)) {
		t.Errorf("rows should be oldest first, last = %v", got[6].Time)
	}
}

func TestRecordStationaryHeartbeat(t *testing.T) {
	s := newTestStore(t)
	// A moored ship reporting every 10 s for 12 minutes without moving: first fix plus one per 5 min.
	for i := 0; i <= 72; i++ {
		s.Record(contact("v:1", model.KindVessel, time.Duration(i)*10*time.Second, -33.9, 18.43))
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Track(context.Background(), "v:1", base.Add(-time.Hour))
	if len(got) != 3 { // 0, 5 min, 10 min
		t.Fatalf("want 3 heartbeat rows, got %d", len(got))
	}
}

func TestTrackWindowAndPrune(t *testing.T) {
	s := newTestStore(t)
	s.Record(contact("a:1", model.KindAircraft, 0, -33, 18))
	s.Record(contact("a:1", model.KindAircraft, 2*time.Hour, -32, 19))
	s.Record(contact("a:2", model.KindAircraft, 2*time.Hour, -30, 20))
	ctx := context.Background()
	if err := s.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	recent, _ := s.Track(ctx, "a:1", base.Add(time.Hour))
	if len(recent) != 1 {
		t.Errorf("window should drop the older fix, got %d", len(recent))
	}
	n, err := s.Prune(ctx, base.Add(73*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("prune removed %d rows (%v), want 1", n, err)
	}
	all, _ := s.Track(ctx, "a:1", base.Add(-time.Hour))
	if len(all) != 1 {
		t.Errorf("after prune want 1 row for a:1, got %d", len(all))
	}
	if _, ok := s.last["a:1"]; !ok {
		t.Error("a contact newer than retention should stay in the rate-limit map")
	}
}

func TestFlushFailureKeepsBuffer(t *testing.T) {
	s := newTestStore(t)
	s.Record(contact("a:1", model.KindAircraft, 0, -33, 18))
	_ = s.db.Close()
	if err := s.Flush(context.Background()); err == nil {
		t.Fatal("flush on a closed database should fail")
	}
	if len(s.buf) != 1 {
		t.Errorf("failed rows should stay buffered, have %d", len(s.buf))
	}
}

func TestRunFlushesOnShutdown(t *testing.T) {
	s := newTestStore(t)
	s.opts.FlushEvery = time.Hour // only the final flush can write
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	s.Record(contact("a:1", model.KindAircraft, time.Since(base), -33, 18))
	cancel()
	<-done
	got, _ := s.Track(context.Background(), "a:1", base)
	if len(got) != 1 {
		t.Errorf("shutdown should flush the buffer, got %d rows", len(got))
	}
}
