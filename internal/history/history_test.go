package history

import (
	"context"
	"math"
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
	// A plane zig-zagging about 1 km at every 10 s sample, reporting every 2 s for a minute: one row per 10 s.
	for i := 0; i <= 30; i++ {
		s.Record(contact("a:1", model.KindAircraft, time.Duration(i)*2*time.Second, -33+float64((i/5)%2)*0.01, 18+float64(i)*0.01))
	}
	s.releaseHeld(base.Add(time.Hour), 0)
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
	// A moored ship reporting every 10 s for 70 minutes without moving: first fix plus one per 30 min.
	for i := 0; i <= 420; i++ {
		s.Record(contact("v:1", model.KindVessel, time.Duration(i)*10*time.Second, -33.9, 18.43))
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Track(context.Background(), "v:1", base.Add(-time.Hour))
	if len(got) != 3 { // 0, 30 min, 60 min
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

// distToPathM is the distance from a point to the nearest segment of a stored path.
func distToPathM(path []model.Position, lat, lon float64) float64 {
	best := math.MaxFloat64
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		cos := math.Cos(a.Lat * math.Pi / 180)
		bx, by := (b.Lon-a.Lon)*cos*111195, (b.Lat-a.Lat)*111195
		px, py := (lon-a.Lon)*cos*111195, (lat-a.Lat)*111195
		t := 0.0
		if l2 := bx*bx + by*by; l2 > 0 {
			t = math.Max(0, math.Min(1, (px*bx+py*by)/l2))
		}
		best = math.Min(best, math.Hypot(px-t*bx, py-t*by))
	}
	return best
}

func TestStraightFlightKeepsOnlyHeartbeats(t *testing.T) {
	s := newTestStore(t)
	// 10 minutes of level flight east at about 400 kt, sampled every 10 s.
	for i := 0; i <= 60; i++ {
		s.Record(contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, -33, 18+float64(i)*0.02))
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	if len(got) != 3 { // t = 0, then one heartbeat per 5 minutes
		t.Fatalf("a straight leg should store 3 rows, got %d", len(got))
	}
}

func TestTurnKeepsItsCorner(t *testing.T) {
	s := newTestStore(t)
	type pt struct{ lat, lon float64 }
	var flown []pt
	for i := 0; i < 30; i++ { // east
		flown = append(flown, pt{-33, 18 + float64(i)*0.02})
	}
	for i := 1; i <= 30; i++ { // then north
		flown = append(flown, pt{-33 + float64(i)*0.02, 18 + 29*0.02})
	}
	for i, p := range flown {
		s.Record(contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, p.lat, p.lon))
	}
	s.releaseHeld(base.Add(time.Hour), 0)
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	if len(got) > 5 {
		t.Errorf("an L-shaped flight needs a handful of rows, got %d", len(got))
	}
	for i, p := range flown {
		if d := distToPathM(got, p.lat, p.lon); d > 50.5 {
			t.Fatalf("sample %d is %.0f m off the stored path, tolerance is 50 m", i, d)
		}
	}
}

func TestLevelOffKeepsItsAltitudeStep(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 60; i++ {
		c := contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, -33, 18+float64(i)*0.02)
		alt := min(i*200, 6000) // climbs 200 ft per sample, then levels at 6000 ft
		c.AltFt = &alt
		s.Record(c)
	}
	s.releaseHeld(base.Add(time.Hour), 0)
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	found := false
	for _, p := range got {
		if p.AltFt != nil && *p.AltFt == 6000 && p.Time.Equal(base.Add(300*time.Second)) {
			found = true
		}
	}
	if !found {
		t.Errorf("the level-off at 6000 ft should be stored as a row, got %d rows", len(got))
	}
}

func TestJitteringMooredShipStoresOnlyHeartbeats(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i <= 420; i++ { // 70 minutes, position wobbles by about 10 m
		lat := -33.9 + float64(i%2)*0.0001
		s.Record(contact("v:1", model.KindVessel, time.Duration(i)*10*time.Second, lat, 18.43))
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Track(context.Background(), "v:1", base.Add(-time.Hour))
	if len(got) != 3 {
		t.Fatalf("want 3 heartbeat rows, got %d", len(got))
	}
}

func TestMovingContactKeepsShortHeartbeat(t *testing.T) {
	s := newTestStore(t)
	// Level flight for 11 minutes keeps the 5 minute heartbeat; only a still contact waits 30 minutes.
	for i := 0; i <= 66; i++ {
		s.Record(contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, -33, 18+float64(i)*0.02))
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	if len(got) != 3 { // 0, 5 min, 10 min
		t.Fatalf("want 3 rows, got %d", len(got))
	}
}

func TestIdleContactReleasesItsLastPosition(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i <= 9; i++ { // 90 s of straight flight, then the feed goes quiet
		s.Record(contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, -33, 18+float64(i)*0.02))
	}
	s.releaseHeld(base.Add(91*time.Second), 2*time.Minute)
	_ = s.Flush(context.Background())
	if got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour)); len(got) != 1 {
		t.Fatalf("a recently seen contact keeps its held point, got %d rows", len(got))
	}
	s.releaseHeld(base.Add(5*time.Minute), 2*time.Minute)
	_ = s.Flush(context.Background())
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	if len(got) != 2 || !got[1].Time.Equal(base.Add(90*time.Second)) {
		t.Fatalf("an idle contact should store its newest position, got %+v", got)
	}
}

func TestNewCallsignMidLineIsStored(t *testing.T) {
	s := newTestStore(t)
	// Straight level flight; the callsign changes at 60 s, which the straight-line filter alone would drop.
	for i := 0; i <= 12; i++ {
		c := contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, -33, 18+float64(i)*0.02)
		c.Callsign = "SFR177"
		if i >= 6 {
			c.Callsign = "SFR251"
		}
		s.Record(c)
	}
	s.releaseHeld(base.Add(time.Hour), 0)
	_ = s.Flush(context.Background())
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	var first *model.Position
	for i := range got {
		if got[i].Callsign == "SFR251" {
			first = &got[i]
			break
		}
	}
	if first == nil || !first.Time.Equal(base.Add(60*time.Second)) {
		t.Fatalf("the first sample under the new callsign should be stored, got %+v", got)
	}
	if got[0].Callsign != "SFR177" {
		t.Errorf("callsign should round-trip, got %q", got[0].Callsign)
	}
}

func TestMissingCallsignDoesNotForceAWrite(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i <= 12; i++ {
		c := contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, -33, 18+float64(i)*0.02)
		if i%3 != 0 {
			c.Callsign = "SFR177"
		}
		s.Record(c)
	}
	s.releaseHeld(base.Add(time.Hour), 0)
	_ = s.Flush(context.Background())
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	if len(got) != 2 { // first fix and the released last one
		t.Fatalf("callsign dropouts should not add rows, got %d", len(got))
	}
}

func TestTouchdownAndTakeoffAreStored(t *testing.T) {
	s := newTestStore(t)
	// Rolls down the runway in a straight line: airborne, on the ground, then airborne again.
	for i := 0; i <= 12; i++ {
		c := contact("a:1", model.KindAircraft, time.Duration(i)*10*time.Second, -33, 18+float64(i)*0.01)
		c.AltFt = nil
		c.OnGround = i >= 4 && i < 8
		s.Record(c)
	}
	s.releaseHeld(base.Add(time.Hour), 0)
	_ = s.Flush(context.Background())
	got, _ := s.Track(context.Background(), "a:1", base.Add(-time.Hour))
	want := map[time.Duration]bool{40 * time.Second: true, 80 * time.Second: false}
	for at, ground := range want {
		found := false
		for _, p := range got {
			if p.Time.Equal(base.Add(at)) && p.OnGround == ground {
				found = true
			}
		}
		if !found {
			t.Errorf("want a row at %v with on_ground=%v, got %+v", at, ground, got)
		}
	}
}
