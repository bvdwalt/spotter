package route

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestParseFixture(t *testing.T) {
	f, err := os.Open("testdata/routeset.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	routes, err := Parse(json.NewDecoder(f))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := routes["LNK351Q"]; ok {
		t.Error("unknown route should be absent")
	}
	r := routes["SFR806"]
	if r == nil || r.From.IATA != "JNB" || r.To.ICAO != "FACT" || !r.Plausible || r.To.Lat == 0 {
		t.Errorf("SFR806 = %+v", r)
	}
}

func TestLookupCachesPositiveAndNegative(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.ServeFile(w, r, "testdata/routeset.json")
	}))
	defer srv.Close()
	res := New(srv.URL)
	qs := []Query{{Callsign: "SFR806"}, {Callsign: "LNK351Q"}}
	first := res.Lookup(context.Background(), qs)
	second := res.Lookup(context.Background(), qs)
	if calls != 1 {
		t.Errorf("want one upstream call, got %d", calls)
	}
	if first["SFR806"] == nil || second["SFR806"] == nil {
		t.Error("known route missing")
	}
	if rt, ok := second["LNK351Q"]; !ok || rt != nil {
		t.Error("unknown callsign should be cached as nil")
	}
}

func TestLookupUpstreamErrorLeavesCacheEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	res := New(srv.URL)
	out := res.Lookup(context.Background(), []Query{{Callsign: "SFR806"}})
	if len(out) != 0 || len(res.cache) != 0 {
		t.Errorf("error should cache nothing: %v %v", out, res.cache)
	}
}

func TestLookupPausesAfter429(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	res := New(srv.URL)
	res.Lookup(context.Background(), []Query{{Callsign: "SFR806"}})
	res.Lookup(context.Background(), []Query{{Callsign: "SFR806"}})
	if calls != 1 {
		t.Errorf("second lookup should be paused, got %d calls", calls)
	}
}
