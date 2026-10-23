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

func TestSuffixFallback(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Planes []requestPlane }
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, p := range body.Planes {
			asked = append(asked, p.Callsign)
		}
		_, _ = w.Write([]byte(`[{"callsign":"LNK351Q","airport_codes":"unknown","_airports":[]},
		  {"callsign":"LNK351","airport_codes":"FIMP-FACT","plausible":true,"_airports":[{"iata":"MRU","icao":"FIMP"},{"iata":"CPT","icao":"FACT"}]}]`))
	}))
	defer srv.Close()
	res := New(srv.URL)
	out := res.Lookup(context.Background(), []Query{{Callsign: "LNK351Q"}})
	if len(asked) != 2 || asked[1] != "LNK351" {
		t.Errorf("should also ask for the bare callsign, asked %v", asked)
	}
	if rt := out["LNK351Q"]; rt == nil || rt.From.IATA != "MRU" {
		t.Errorf("fallback route = %+v", rt)
	}
	if bareCallsign("SAA372") != "SAA372" || bareCallsign("ZSFHH") != "ZSFHH" || bareCallsign("UBT795A") != "UBT795" {
		t.Error("bareCallsign rules")
	}
}
