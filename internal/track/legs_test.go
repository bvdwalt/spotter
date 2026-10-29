package track

import (
	"testing"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

var legBase = time.Unix(1_700_000_000, 0)

func pos(min int, callsign string, ground bool) model.Position {
	return model.Position{Lat: -26, Lon: 28 + float64(min)*0.01, Callsign: callsign, OnGround: ground, Time: legBase.Add(time.Duration(min) * time.Minute)}
}

func legSizes(legs []Leg) []int {
	var n []int
	for _, l := range legs {
		n = append(n, len(l.Positions))
	}
	return n
}

func TestLegsSplitAtGroundStop(t *testing.T) {
	// PLZ to JNB, 40 minutes on the ground at JNB under a new flight number, then JNB to DUR.
	ps := []model.Position{
		pos(0, "SFR177", false), pos(5, "SFR177", false), pos(10, "SFR177", false),
		pos(15, "SFR177", true), pos(25, "SFR177", true), pos(40, "LNK553C", true), pos(55, "LNK553C", true),
		pos(56, "LNK553C", false), pos(60, "LNK553C", false),
	}
	legs := Legs(ps)
	if got := legSizes(legs); len(got) != 2 || got[0] != 4 || got[1] != 3 {
		t.Fatalf("want legs of 4 and 3 points (touchdown and takeoff points kept, taxiing dropped), got %v", got)
	}
	if legs[0].Callsign != "SFR177" || legs[1].Callsign != "LNK553C" {
		t.Errorf("callsigns = %q, %q", legs[0].Callsign, legs[1].Callsign)
	}
	if !legs[1].Start.Equal(legBase.Add(55*time.Minute)) || !legs[1].End.Equal(legBase.Add(60*time.Minute)) {
		t.Errorf("second leg spans %v to %v", legs[1].Start, legs[1].End)
	}
}

func TestLegsSplitOnCallsignChangeInTheAir(t *testing.T) {
	ps := []model.Position{pos(0, "SFR177", false), pos(5, "SFR177", false), pos(10, "SFR251", false), pos(15, "SFR251", false)}
	if got := legSizes(Legs(ps)); len(got) != 2 || got[0] != 2 || got[1] != 2 {
		t.Fatalf("want two legs of 2, got %v", got)
	}
}

func TestLegsIgnoreCallsignDropouts(t *testing.T) {
	ps := []model.Position{pos(0, "", false), pos(5, "SFR177", false), pos(10, "", false), pos(15, "SFR177", false)}
	legs := Legs(ps)
	if got := legSizes(legs); len(got) != 1 || got[0] != 4 {
		t.Fatalf("want one leg of 4, got %v", got)
	}
	if legs[0].Callsign != "SFR177" {
		t.Errorf("leg callsign = %q", legs[0].Callsign)
	}
}

func TestLegsSplitOnLongGap(t *testing.T) {
	// Older rows have no callsign or ground flag; a long silence is the only sign of a stop.
	ps := []model.Position{pos(0, "", false), pos(5, "", false), pos(60, "", false), pos(65, "", false)}
	if got := legSizes(Legs(ps)); len(got) != 2 || got[0] != 2 || got[1] != 2 {
		t.Fatalf("want two legs of 2, got %v", got)
	}
}

func TestLegsKeepMooredShipWhole(t *testing.T) {
	// A still vessel is stored once every 30 minutes, which must not read as a gap.
	ps := []model.Position{pos(0, "", false), pos(30, "", false), pos(60, "", false), pos(90, "", false)}
	if got := legSizes(Legs(ps)); len(got) != 1 || got[0] != 4 {
		t.Fatalf("want one leg of 4, got %v", got)
	}
}

func TestLegsGroundOnlyIsOneLeg(t *testing.T) {
	ps := []model.Position{pos(0, "SFR177", true), pos(2, "SFR177", true), pos(4, "SFR177", true)}
	if got := legSizes(Legs(ps)); len(got) != 1 || got[0] != 3 {
		t.Fatalf("an aircraft that has only taxied keeps its trail, got %v", got)
	}
}

func TestLegsEmpty(t *testing.T) {
	if legs := Legs(nil); len(legs) != 0 {
		t.Fatalf("want no legs, got %v", legs)
	}
}
