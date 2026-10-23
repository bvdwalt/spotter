package alert

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

type recorder struct{ sent []Alert }

func (r *recorder) Send(_ context.Context, a Alert) error { r.sent = append(r.sent, a); return nil }

func TestRulesAndCooldown(t *testing.T) {
	rec := &recorder{}
	e := New(Options{WatchList: []string{"ABC123", " 601972000 "}, Cooldown: time.Hour}, rec)
	now := time.Unix(1_700_000_000, 0)
	alt := 35000
	cases := []struct {
		c    model.Contact
		want []string
	}{
		{model.Contact{ID: "a:1", Label: "X", Squawk: "7700", Seen: now, AltFt: &alt}, []string{"emergency"}},
		{model.Contact{ID: "a:1", Label: "X", Squawk: "7700", Seen: now.Add(time.Minute)}, nil}, // cooldown
		{model.Contact{ID: "a:1", Label: "X", Squawk: "7700", Seen: now.Add(2 * time.Hour)}, []string{"emergency"}},
		{model.Contact{ID: "a:2", Label: "MIL", Military: true, Emergency: "7600", Seen: now}, []string{"emergency", "military"}},
		{model.Contact{ID: "a:abc123", Label: "W", Seen: now}, []string{"watchlist"}},
		{model.Contact{ID: "v:601972000", Label: "SHIP", Kind: model.KindVessel, Seen: now}, []string{"watchlist"}},
		{model.Contact{ID: "a:3", Label: "plain", Squawk: "1200", Seen: now}, nil},
	}
	for i, tc := range cases {
		got := e.Observe(context.Background(), tc.c)
		var rules []string
		for _, a := range got {
			rules = append(rules, a.Rule)
		}
		if len(rules) != len(tc.want) {
			t.Errorf("case %d: rules = %v, want %v", i, rules, tc.want)
			continue
		}
		for j := range rules {
			if rules[j] != tc.want[j] {
				t.Errorf("case %d: rules = %v, want %v", i, rules, tc.want)
			}
		}
	}
	if len(rec.sent) != 6 || len(e.Recent()) != 6 {
		t.Errorf("sent %d, recent %d, want 6", len(rec.sent), len(e.Recent()))
	}
	if rec.sent[0].Message != "General emergency squawk. 35000 ft, 0 km from home" {
		t.Errorf("message = %q", rec.sent[0].Message)
	}
	if got := rec.sent[4].Message; got != "0 km from home" {
		t.Errorf("empty type and reg should not leave a leading space, got %q", got)
	}
}

func TestNtfySink(t *testing.T) {
	var gotTitle, gotPrio, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTitle = r.Header.Get("Title")
		gotPrio = r.Header.Get("Priority")
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, 64)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
	}))
	defer srv.Close()
	s := NewNtfy(srv.URL, "tok")
	err := s.Send(context.Background(), Alert{Rule: "emergency", Title: "T", Message: "M"})
	if err != nil {
		t.Fatal(err)
	}
	if gotTitle != "T" || gotPrio != "high" || gotAuth != "Bearer tok" || gotBody != "M" {
		t.Errorf("got %q %q %q %q", gotTitle, gotPrio, gotAuth, gotBody)
	}
}

func TestClearAndDismiss(t *testing.T) {
	e := New(Options{Cooldown: time.Hour}, nil)
	now := time.Unix(1_700_000_000, 0)
	e.Observe(context.Background(), model.Contact{ID: "a:1", Squawk: "7700", Seen: now})
	e.Observe(context.Background(), model.Contact{ID: "a:2", Military: true, Seen: now})
	if !e.Dismiss("emergency", "a:1", now) || e.Dismiss("emergency", "a:1", now) {
		t.Error("dismiss should succeed once")
	}
	if len(e.Recent()) != 1 {
		t.Errorf("recent = %d, want 1", len(e.Recent()))
	}
	e.Clear()
	if len(e.Recent()) != 0 {
		t.Error("clear should empty recent")
	}
	if got := e.Observe(context.Background(), model.Contact{ID: "a:1", Squawk: "7700", Seen: now.Add(time.Minute)}); len(got) != 0 {
		t.Error("cooldown should survive clear")
	}
}

func TestWatchAndUnwatch(t *testing.T) {
	e := New(Options{Cooldown: time.Hour}, nil)
	now := time.Unix(1_700_000_000, 0)
	c := model.Contact{ID: "v:123", Kind: model.KindVessel, Label: "S", Seen: now}
	if got := e.Observe(context.Background(), c); len(got) != 0 {
		t.Fatal("unwatched contact should not alert")
	}
	e.Watch("v:123")
	if got := e.Observe(context.Background(), c); len(got) != 1 || got[0].Rule != "watchlist" {
		t.Fatalf("watched contact should alert: %v", got)
	}
	if !e.Watching("v:123") || e.WatchList()[0] != "123" {
		t.Errorf("watch state: %v", e.WatchList())
	}
	e.Unwatch("v:123")
	e.Watch("123")
	if got := e.Observe(context.Background(), c); len(got) != 1 {
		t.Error("re-watching should alert again despite cooldown")
	}
}
