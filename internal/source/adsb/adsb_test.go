package adsb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

func TestParseFixture(t *testing.T) {
	f, err := os.Open("testdata/point.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	contacts, err := Parse(json.NewDecoder(f), "test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) == 0 {
		t.Fatal("expected contacts from fixture")
	}
	c := contacts[0]
	if c.Kind != model.KindAircraft || !strings.HasPrefix(c.ID, "a:") {
		t.Errorf("bad kind/id: %+v", c)
	}
	if c.Callsign != "SAA375" {
		t.Errorf("callsign should be trimmed, got %q", c.Callsign)
	}
	if c.Label == "" || c.AltFt == nil || c.HeadingDeg == nil {
		t.Errorf("missing fields: %+v", c)
	}
	if c.Seen.IsZero() || time.Since(c.Seen) < 0 {
		t.Errorf("bad seen: %v", c.Seen)
	}
}

func TestParseGroundAndSkips(t *testing.T) {
	body := `{"now": 1700000000.5, "ac": [
	  {"hex":"abc123","flight":"  ","r":"ZS-ABC","alt_baro":"ground","lat":1,"lon":2,"seen_pos":2,"dbFlags":1,"emergency":"7700"},
	  {"hex":"nopos","flight":"X"}
	]}`
	contacts, err := Parse(json.NewDecoder(strings.NewReader(body)), "test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 {
		t.Fatalf("want 1 contact, got %d", len(contacts))
	}
	c := contacts[0]
	if !c.OnGround || c.AltFt != nil {
		t.Errorf("expected on ground: %+v", c)
	}
	if c.Label != "ZS-ABC" || c.Callsign != "" {
		t.Errorf("label fallback to reg without a callsign, got %q %q", c.Label, c.Callsign)
	}
	if !c.Military || c.Emergency != "7700" {
		t.Errorf("flags: %+v", c)
	}
	want := time.Unix(1700000000, 500_000_000).Add(-2 * time.Second)
	if !c.Seen.Equal(want) {
		t.Errorf("seen = %v, want %v", c.Seen, want)
	}
}

func TestParseAggregatorError(t *testing.T) {
	_, err := Parse(json.NewDecoder(strings.NewReader(`{"error":"go away"}`)), "test", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPollBuildsURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		http.ServeFile(w, r, "testdata/point.json")
	}))
	defer srv.Close()
	s := New(srv.URL+"/v2/", []model.Circle{{Lat: -26.2, Lon: 28.05, RadiusNM: 150}}, time.Second)
	contacts, err := s.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/point/-26.2000/28.0500/150" {
		t.Errorf("path = %s", gotPath)
	}
	if len(contacts) == 0 {
		t.Error("no contacts")
	}
}

func TestPollRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	s := New(srv.URL, []model.Circle{{RadiusNM: 10}}, 5*time.Second)
	_, err := s.Poll(context.Background())
	var rl RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter != 120*time.Second {
		t.Fatalf("want RateLimitedError with 120s, got %v", err)
	}
}

func TestRetryAfterBounds(t *testing.T) {
	h := http.Header{}
	if got := retryAfter(h, 30*time.Second); got != 30*time.Second {
		t.Errorf("fallback: %v", got)
	}
	h.Set("X-Ratelimit-Reset", "1")
	if got := retryAfter(h, 30*time.Second); got != 30*time.Second {
		t.Errorf("below fallback should clamp up: %v", got)
	}
	h.Set("Retry-After", "99999")
	if got := retryAfter(h, 30*time.Second); got != maxRetryAfter {
		t.Errorf("huge value should clamp to max: %v", got)
	}
}

func TestPointURLShapes(t *testing.T) {
	c := model.Circle{Lat: -33.93, Lon: 18.45, RadiusNM: 150}
	lol := New("https://api.adsb.lol/v2", []model.Circle{c}, time.Second)
	if got := lol.pointURL(c); got != "https://api.adsb.lol/v2/point/-33.9300/18.4500/150" {
		t.Errorf("adsb.lol url = %s", got)
	}
	fi := New("https://opendata.adsb.fi/api/v2", []model.Circle{c}, time.Second)
	if got := fi.pointURL(c); got != "https://opendata.adsb.fi/api/v2/lat/-33.9300/lon/18.4500/dist/150" {
		t.Errorf("adsb.fi url = %s", got)
	}
}

func TestLocalAircraftJSONFiltersByRadius(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data/aircraft.json" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "testdata/point.json") // fixture aircraft are all around Johannesburg
	}))
	defer srv.Close()
	near := New(srv.URL+"/data/aircraft.json", []model.Circle{{Lat: -26.2, Lon: 28.05, RadiusNM: 150}}, time.Second)
	got, err := near.Poll(context.Background())
	if err != nil || len(got) == 0 {
		t.Fatalf("near: %d %v", len(got), err)
	}
	far := New(srv.URL+"/data/aircraft.json", []model.Circle{{Lat: -33.9, Lon: 18.4, RadiusNM: 50}}, time.Second)
	got, err = far.Poll(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("far: want 0 contacts, got %d %v", len(got), err)
	}
	// An aircraft only needs to be inside one of several circles.
	either := New(srv.URL+"/data/aircraft.json", []model.Circle{
		{Lat: -33.9, Lon: 18.4, RadiusNM: 50},
		{Lat: -26.2, Lon: 28.05, RadiusNM: 150},
	}, time.Second)
	got, err = either.Poll(context.Background())
	if err != nil || len(got) == 0 {
		t.Fatalf("either: want the Johannesburg contacts, got %d %v", len(got), err)
	}
	if near.Name() == "" {
		t.Error("name should derive from the host")
	}
}

func TestPollMergesCircles(t *testing.T) {
	// Circle A sees 111111 and an older fix of 222222; circle B sees a newer fix of 222222 and 333333.
	bodies := map[string]string{
		"/point/-32.0800/21.1600/200": `{"now": 1700000010, "ac": [
		  {"hex":"111111","flight":"A1","lat":-32,"lon":21,"seen_pos":0},
		  {"hex":"222222","flight":"OLD","lat":-30,"lon":23,"seen_pos":5}]}`,
		"/point/-28.1600/25.9700/200": `{"now": 1700000010, "ac": [
		  {"hex":"222222","flight":"NEW","lat":-29,"lon":24,"seen_pos":1},
		  {"hex":"333333","flight":"C3","lat":-28,"lon":26,"seen_pos":0}]}`,
	}
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	s := New(srv.URL, []model.Circle{
		{Lat: -32.08, Lon: 21.16, RadiusNM: 200},
		{Lat: -28.16, Lon: 25.97, RadiusNM: 200},
	}, time.Second)
	s.Spacing = 0
	got, err := s.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Errorf("want one request per circle, got %v", paths)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 merged contacts, got %d: %+v", len(got), got)
	}
	labels := map[string]string{}
	for _, c := range got {
		labels[c.ID] = c.Label
	}
	if labels["a:222222"] != "NEW" {
		t.Errorf("overlap should keep the freshest fix, got %q", labels["a:222222"])
	}
	if got[0].ID != "a:111111" || got[2].ID != "a:333333" {
		t.Errorf("results should be sorted by ID: %v %v", got[0].ID, got[2].ID)
	}
}

func TestPollRateLimitedOnSecondCircle(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.Header().Set("Retry-After", "45")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		http.ServeFile(w, r, "testdata/point.json")
	}))
	defer srv.Close()
	s := New(srv.URL, []model.Circle{{RadiusNM: 10}, {RadiusNM: 10}, {RadiusNM: 10}}, 5*time.Second)
	s.Spacing = 0
	_, err := s.Poll(context.Background())
	var rl RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter != 45*time.Second {
		t.Fatalf("want RateLimitedError with 45s, got %v", err)
	}
	if calls != 2 {
		t.Errorf("polling should stop at the rate-limited circle, made %d calls", calls)
	}
}

func TestPollSpacesCircleRequests(t *testing.T) {
	var times []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		times = append(times, time.Now())
		_, _ = w.Write([]byte(`{"ac": []}`))
	}))
	defer srv.Close()
	s := New(srv.URL, []model.Circle{{RadiusNM: 10}, {RadiusNM: 10}}, time.Second)
	s.Spacing = 150 * time.Millisecond
	if _, err := s.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(times) != 2 || times[1].Sub(times[0]) < 150*time.Millisecond {
		t.Errorf("second request should wait for the spacing, gap %v", times[1].Sub(times[0]))
	}
}
