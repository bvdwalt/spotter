package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bvdwalt/spotter/internal/alert"
	"github.com/bvdwalt/spotter/internal/model"
	"github.com/bvdwalt/spotter/internal/track"
)

type fakeSource struct{}

func (fakeSource) Status() model.SourceStatus { return model.SourceStatus{Name: "fake", Enabled: true} }

func newTestServer() http.Handler {
	store := track.New(track.Options{AircraftExpiry: time.Minute, VesselExpiry: time.Minute, TrackWindow: time.Minute, TrackMinInterval: time.Second})
	store.Upsert(model.Contact{ID: "a:1", Kind: model.KindAircraft, Label: "TEST1", Lat: 1, Lon: 1, Seen: time.Now()})
	alerts := alert.New(alert.Options{Cooldown: time.Hour}, nil)
	alerts.Observe(context.Background(), model.Contact{ID: "a:9", Label: "MAYDAY", Squawk: "7700", Seen: time.Now()})
	return New(Deps{Store: store, Alerts: alerts, Sources: []model.StatusReporter{fakeSource{}}, Version: "test"})
}

func get(t *testing.T, h http.Handler, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if rec.Body.Len() > 0 && rec.Header().Get("Content-Type") == "application/json" {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: bad json: %v", path, err)
		}
	}
	return rec.Code, body
}

func TestContactsAndTrack(t *testing.T) {
	h := newTestServer()
	code, body := get(t, h, "/api/contacts")
	if code != 200 || len(body["contacts"].([]any)) != 1 {
		t.Fatalf("contacts: %d %v", code, body)
	}
	for _, path := range []string{"/api/contacts/a:1/track", "/api/contacts/a%3A1/track"} {
		if code, _ := get(t, h, path); code != 200 {
			t.Errorf("track %s: %d", path, code)
		}
	}
	if code, _ := get(t, h, "/api/contacts/nope/track"); code != 404 {
		t.Errorf("unknown track: %d", code)
	}
}

func TestAlerts(t *testing.T) {
	h := newTestServer()
	code, body := get(t, h, "/api/alerts")
	if code != 200 || len(body["alerts"].([]any)) != 1 {
		t.Fatalf("alerts: %d %v", code, body)
	}
	a := body["alerts"].([]any)[0].(map[string]any)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/alerts/emergency/a%3A9?at="+url.QueryEscape(a["time"].(string)), nil))
	if rec.Code != 204 {
		t.Fatalf("dismiss: %d %s", rec.Code, rec.Body.String())
	}
	if code, body := get(t, h, "/api/alerts"); code != 200 || len(body["alerts"].([]any)) != 0 {
		t.Fatalf("after dismiss: %d %v", code, body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/alerts", nil))
	if rec.Code != 204 {
		t.Fatalf("clear: %d", rec.Code)
	}
}

func TestStatusAndHealth(t *testing.T) {
	h := newTestServer()
	code, body := get(t, h, "/api/status")
	if code != 200 || body["version"] != "test" || len(body["sources"].([]any)) != 1 {
		t.Fatalf("status: %d %v", code, body)
	}
	if code, _ := get(t, h, "/health"); code != 200 {
		t.Errorf("health: %d", code)
	}
}

func TestWatchEndpoints(t *testing.T) {
	h := newTestServer()
	do := func(method, path string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec.Code
	}
	if code := do(http.MethodPut, "/api/watch/v%3A123"); code != 204 {
		t.Fatalf("watch: %d", code)
	}
	if _, body := get(t, h, "/api/watch"); len(body["watch"].([]any)) != 1 || body["watch"].([]any)[0] != "123" {
		t.Fatalf("watch list: %v", body)
	}
	if code := do(http.MethodDelete, "/api/watch/v%3A123"); code != 204 {
		t.Fatalf("unwatch: %d", code)
	}
	if _, body := get(t, h, "/api/watch"); len(body["watch"].([]any)) != 0 {
		t.Fatalf("after unwatch: %v", body)
	}
}

func TestPhotoDisabled(t *testing.T) {
	if code, _ := get(t, newTestServer(), "/api/contacts/a:1/photo"); code != 404 {
		t.Errorf("photo without client: %d", code)
	}
}

func TestBasemapRange(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.pmtiles")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("0123456789")
	_ = f.Close()
	h := New(Deps{Store: track.New(track.Options{}), Alerts: alert.New(alert.Options{}, nil), PMTiles: f.Name()})
	req := httptest.NewRequest(http.MethodGet, "/basemap.pmtiles", nil)
	req.Header.Set("Range", "bytes=2-4")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "234" {
		t.Errorf("range: %d %q", rec.Code, rec.Body.String())
	}
	if code, _ := get(t, newTestServer(), "/basemap.pmtiles"); code == http.StatusPartialContent {
		t.Error("without PMTiles the route should fall through to the SPA")
	}
}

func TestMetrics(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`spotter_contacts{kind="aircraft"} 1`,
		`spotter_source_up{source="fake"} 0`,
		"# TYPE spotter_source_polls_total counter",
		"spotter_alerts_recent 1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q in:\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content type %q", ct)
	}
}

type fakeHistory struct {
	rows map[string][]model.Position
	err  error
}

func (f fakeHistory) Track(_ context.Context, id string, _ time.Time) ([]model.Position, error) {
	return f.rows[id], f.err
}

// trackLen returns the status and the number of positions across all legs.
func trackLen(t *testing.T, h http.Handler, path string) (int, int) {
	t.Helper()
	code, body := get(t, h, path)
	if code != 200 {
		return code, 0
	}
	n := 0
	for _, l := range body["legs"].([]any) {
		n += len(l.(map[string]any)["positions"].([]any))
	}
	return code, n
}

func TestTrackFromHistory(t *testing.T) {
	now := time.Now()
	store := track.New(track.Options{AircraftExpiry: time.Minute, VesselExpiry: time.Minute, TrackWindow: time.Hour, TrackMinInterval: time.Second})
	store.Upsert(model.Contact{ID: "a:1", Kind: model.KindAircraft, Seen: now.Add(-2 * time.Second)})
	store.Upsert(model.Contact{ID: "a:1", Kind: model.KindAircraft, Seen: now})
	hist := fakeHistory{rows: map[string][]model.Position{
		"a:1":    {{Time: now.Add(-2 * time.Hour)}, {Time: now.Add(-time.Hour)}, {Time: now.Add(-2 * time.Second)}},
		"a:gone": {{Time: now.Add(-3 * time.Hour)}},
	}}
	h := New(Deps{Store: store, Alerts: alert.New(alert.Options{}, nil), History: hist, TrackHistory: 6 * time.Hour})

	// Three stored rows plus the one live point newer than the last stored row.
	if code, n := trackLen(t, h, "/api/contacts/a:1/track"); code != 200 || n != 4 {
		t.Errorf("a:1: code %d, %d positions, want 200 and 4", code, n)
	}
	// The hour between stored rows is a gap, and the live point continues the newest leg.
	if _, body := get(t, h, "/api/contacts/a:1/track"); len(body["legs"].([]any)) != 3 {
		t.Errorf("a:1: want 3 legs, got %v", body["legs"])
	}
	// A contact that has expired from memory still has its stored path.
	if code, n := trackLen(t, h, "/api/contacts/a:gone/track"); code != 200 || n != 1 {
		t.Errorf("a:gone: code %d, %d positions, want 200 and 1", code, n)
	}
	if code, _ := trackLen(t, h, "/api/contacts/a:none/track"); code != 404 {
		t.Errorf("unknown contact: code %d, want 404", code)
	}

	// A failing database falls back to the in-memory track.
	broken := New(Deps{Store: store, Alerts: alert.New(alert.Options{}, nil), History: fakeHistory{err: context.DeadlineExceeded}, TrackHistory: time.Hour})
	if code, n := trackLen(t, broken, "/api/contacts/a:1/track"); code != 200 || n != 2 {
		t.Errorf("fallback: code %d, %d positions, want 200 and 2", code, n)
	}
}
