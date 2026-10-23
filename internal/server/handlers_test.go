package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
