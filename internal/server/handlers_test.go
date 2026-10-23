package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	code, body := get(t, newTestServer(), "/api/alerts")
	if code != 200 || len(body["alerts"].([]any)) != 1 {
		t.Fatalf("alerts: %d %v", code, body)
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
