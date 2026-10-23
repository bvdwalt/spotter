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
