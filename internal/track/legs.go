package track

import (
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

// LegGap is a silence long enough to mean the contact stopped somewhere out of view. It is longer than
// the 30 minute heartbeat history keeps for a still contact, so a moored ship stays one leg.
const LegGap = 45 * time.Minute

// Leg is one flight or voyage: from takeoff to touchdown, between callsign changes, or between long gaps.
type Leg struct {
	Callsign  string           `json:"callsign,omitempty"`
	Start     time.Time        `json:"start"`
	End       time.Time        `json:"end"`
	Positions []model.Position `json:"positions"`
}

// Legs splits a track, oldest first, into legs. A leg keeps its touchdown point and the last ground
// point before its takeoff; the taxiing in between is dropped so a stop does not tangle the two legs.
func Legs(ps []model.Position) []Leg {
	legs := []Leg{}
	open := false               // the newest leg is still being flown
	var ground []model.Position // on-ground points since the last airborne one
	callsign := ""
	for i, p := range ps {
		if i > 0 && p.Time.Sub(ps[i-1].Time) > LegGap {
			open = false
			ground = nil
		}
		if p.OnGround {
			if open {
				legs[len(legs)-1].add(p)
				open = false
			} else {
				ground = append(ground, p)
			}
			continue
		}
		if p.Callsign != "" {
			if callsign != "" && p.Callsign != callsign {
				open = false
			}
			callsign = p.Callsign
		}
		if !open {
			legs = append(legs, Leg{})
			if len(ground) > 0 {
				legs[len(legs)-1].add(ground[len(ground)-1])
			}
			open = true
		}
		ground = nil
		legs[len(legs)-1].add(p)
	}
	if len(legs) == 0 && len(ground) > 0 {
		l := Leg{}
		for _, p := range ground {
			l.add(p)
		}
		legs = append(legs, l)
	}
	return legs
}

func (l *Leg) add(p model.Position) {
	if len(l.Positions) == 0 {
		l.Start = p.Time
	}
	if l.Callsign == "" {
		l.Callsign = p.Callsign
	}
	l.End = p.Time
	l.Positions = append(l.Positions, p)
}
