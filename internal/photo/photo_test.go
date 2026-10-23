package photo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseFixture(t *testing.T) {
	f, err := os.Open("testdata/hex.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	p, err := Parse(json.NewDecoder(f))
	if err != nil || p == nil {
		t.Fatalf("parse: %v %v", p, err)
	}
	if p.Photographer != "Gerrit Griem" || !strings.HasPrefix(p.Large, "https://") || p.Link == "" {
		t.Errorf("photo = %+v", p)
	}
}

func TestParseEmptyAndError(t *testing.T) {
	if p, err := Parse(json.NewDecoder(strings.NewReader(`{"photos":[]}`))); err != nil || p != nil {
		t.Errorf("empty: %v %v", p, err)
	}
	if _, err := Parse(json.NewDecoder(strings.NewReader(`{"error":"bad UA"}`))); err == nil {
		t.Error("error body should fail")
	}
}

func TestLookupCachesAndSendsUserAgent(t *testing.T) {
	calls := 0
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		ua = r.Header.Get("User-Agent")
		if strings.HasSuffix(r.URL.Path, "/nopic") {
			_, _ = w.Write([]byte(`{"photos":[]}`))
			return
		}
		http.ServeFile(w, r, "testdata/hex.json")
	}))
	defer srv.Close()
	c := New(srv.URL, "test")
	for i := 0; i < 2; i++ {
		if p, err := c.Lookup(context.Background(), "00B22C"); err != nil || p == nil {
			t.Fatalf("lookup %d: %v %v", i, p, err)
		}
		if p, err := c.Lookup(context.Background(), "nopic"); err != nil || p != nil {
			t.Fatalf("miss %d: %v %v", i, p, err)
		}
	}
	if calls != 2 {
		t.Errorf("want 2 upstream calls (one per hex), got %d", calls)
	}
	if !strings.Contains(ua, "+https://github.com/bvdwalt/spotter") {
		t.Errorf("user agent must carry a contact URL, got %q", ua)
	}
}
