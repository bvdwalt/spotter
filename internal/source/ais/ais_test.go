package ais

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

func loadFixture(t *testing.T) []json.RawMessage {
	t.Helper()
	b, err := os.ReadFile("testdata/messages.json")
	if err != nil {
		t.Fatal(err)
	}
	var msgs []json.RawMessage
	if err := json.Unmarshal(b, &msgs); err != nil {
		t.Fatal(err)
	}
	return msgs
}

func TestHandleFixture(t *testing.T) {
	s := New("key", BBox{{-31.2, 29.5}, {-28.5, 33.0}})
	var contacts []model.Contact
	for _, raw := range loadFixture(t) {
		c, err := s.handle(raw, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if c != nil {
			contacts = append(contacts, *c)
		}
	}
	if len(contacts) != 3 {
		t.Fatalf("want 3 position contacts, got %d", len(contacts))
	}
	if len(s.static) != 2 {
		t.Errorf("want 2 static entries, got %d", len(s.static))
	}
	c := contacts[0]
	if c.ID != "v:249391000" || c.Kind != model.KindVessel || c.Label != "LEDA C" {
		t.Errorf("bad contact: %+v", c)
	}
	if c.HeadingDeg == nil || *c.HeadingDeg != 163 {
		t.Errorf("heading should prefer TrueHeading: %+v", c.HeadingDeg)
	}
	if c.NavStatus == nil || *c.NavStatus != 1 {
		t.Errorf("nav status: %+v", c.NavStatus)
	}
	want := time.Date(2026, 10, 23, 20, 13, 17, 348156560, time.UTC)
	if !c.Seen.Equal(want) {
		t.Errorf("seen = %v, want %v", c.Seen, want)
	}
	if c.Source != "aisstream" || c.Lat != -29.74124 {
		t.Errorf("source/lat: %+v", c)
	}
}

func TestStaticDataEnrichesLaterPosition(t *testing.T) {
	s := New("key", BBox{})
	static := `{"MessageType":"ShipStaticData","MetaData":{"MMSI":1,"ShipName":"X ","latitude":0,"longitude":0,"time_utc":"bad"},
	  "Message":{"ShipStaticData":{"Name":"MY SHIP  ","Type":80,"Destination":"ZA DUR   "}}}`
	pos := `{"MessageType":"PositionReport","MetaData":{"MMSI":1,"ShipName":"","latitude":-30,"longitude":31,"time_utc":"bad"},
	  "Message":{"PositionReport":{"Cog":360,"Sog":2.5,"TrueHeading":511,"NavigationalStatus":0}}}`
	if c, err := s.handle(json.RawMessage(static), time.Now()); err != nil || c != nil {
		t.Fatalf("static: %v %v", c, err)
	}
	now := time.Unix(1_700_000_000, 0)
	c, err := s.handle(json.RawMessage(pos), now)
	if err != nil || c == nil {
		t.Fatalf("position: %v %v", c, err)
	}
	if c.Label != "MY SHIP" || c.Type != "Tanker" || c.Destination != "ZA DUR" {
		t.Errorf("static not merged: %+v", c)
	}
	if c.ETA != nil {
		t.Errorf("missing Eta should give nil, got %v", c.ETA)
	}
	if c.HeadingDeg != nil {
		t.Errorf("unavailable heading and cog should give nil, got %v", *c.HeadingDeg)
	}
	if !c.Seen.Equal(now) {
		t.Errorf("unparseable time should fall back to now")
	}
}

func TestHandleServerError(t *testing.T) {
	s := New("key", BBox{})
	if _, err := s.handle(json.RawMessage(`{"error":"Api Key Is Not Valid"}`), time.Now()); err == nil {
		t.Fatal("expected error")
	}
}

func TestShipType(t *testing.T) {
	cases := map[int]string{0: "", 30: "Fishing", 52: "Tug", 65: "Passenger", 79: "Cargo", 80: "Tanker", 99: "Type 99"}
	for code, want := range cases {
		if got := ShipType(code); got != want {
			t.Errorf("ShipType(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestParseETA(t *testing.T) {
	now := time.Date(2026, 10, 23, 20, 0, 0, 0, time.UTC)
	if got := parseETA(0, 0, 24, 60, now); got != nil {
		t.Errorf("unavailable ETA should be nil, got %v", got)
	}
	if got := parseETA(10, 25, 6, 30, now); got == nil || !got.Equal(time.Date(2026, 10, 25, 6, 30, 0, 0, time.UTC)) {
		t.Errorf("same-year ETA: %v", got)
	}
	if got := parseETA(1, 5, 12, 0, now); got == nil || got.Year() != 2027 {
		t.Errorf("January ETA seen in October should roll to next year: %v", got)
	}
}

func TestFixtureETA(t *testing.T) {
	s := New("key", BBox{})
	now := time.Date(2026, 10, 23, 20, 0, 0, 0, time.UTC)
	for _, raw := range loadFixture(t) {
		if _, err := s.handle(raw, now); err != nil {
			t.Fatal(err)
		}
	}
	info := s.static["414604000"]
	if info.eta == nil || info.eta.Month() != 10 || info.eta.Day() != 22 || info.eta.Hour() != 16 {
		t.Errorf("fixture ETA = %v", info.eta)
	}
}
