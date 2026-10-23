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
	if c.Label != "ZS-ABC" {
		t.Errorf("label fallback to reg, got %q", c.Label)
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
	s := New(srv.URL+"/v2/", -26.2, 28.05, 150, time.Second)
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
	s := New(srv.URL, 0, 0, 10, 5*time.Second)
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
	lol := New("https://api.adsb.lol/v2", -33.93, 18.45, 150, time.Second)
	if got := lol.pointURL(); got != "https://api.adsb.lol/v2/point/-33.9300/18.4500/150" {
		t.Errorf("adsb.lol url = %s", got)
	}
	fi := New("https://opendata.adsb.fi/api/v2", -33.93, 18.45, 150, time.Second)
	if got := fi.pointURL(); got != "https://opendata.adsb.fi/api/v2/lat/-33.9300/lon/18.4500/dist/150" {
		t.Errorf("adsb.fi url = %s", got)
	}
}
