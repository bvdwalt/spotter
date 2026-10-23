// Package alert raises notifications for interesting contacts: emergencies, military, and a watch list.
package alert

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

type Alert struct {
	Rule      string     `json:"rule"` // emergency | military | watchlist
	ContactID string     `json:"contact_id"`
	Label     string     `json:"label"`
	Kind      model.Kind `json:"kind"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Time      time.Time  `json:"time"`
}

// Sink delivers an alert somewhere; errors are logged, never retried.
type Sink interface {
	Send(ctx context.Context, a Alert) error
}

type Options struct {
	WatchList []string      // hex codes and MMSIs, case-insensitive
	Cooldown  time.Duration // minimum gap between repeats of the same rule for the same contact
	Keep      int           // recent alerts kept for the API
}

type Engine struct {
	opts  Options
	sink  Sink
	watch map[string]bool

	mu     sync.Mutex
	fired  map[string]time.Time // rule + contact id
	recent []Alert
}

func New(opts Options, sink Sink) *Engine {
	if opts.Keep <= 0 {
		opts.Keep = 50
	}
	watch := make(map[string]bool, len(opts.WatchList))
	for _, w := range opts.WatchList {
		if w = strings.ToLower(strings.TrimSpace(w)); w != "" {
			watch[w] = true
		}
	}
	return &Engine{opts: opts, sink: sink, watch: watch, fired: make(map[string]time.Time)}
}

// Observe evaluates one contact update and sends any alerts that are not within cooldown.
func (e *Engine) Observe(ctx context.Context, c model.Contact) []Alert {
	var out []Alert
	for _, a := range e.evaluate(c) {
		if !e.shouldFire(a) {
			continue
		}
		out = append(out, a)
		if e.sink != nil {
			if err := e.sink.Send(ctx, a); err != nil {
				slog.Warn("alert send failed", "rule", a.Rule, "contact", a.ContactID, "err", err)
			}
		}
	}
	return out
}

func (e *Engine) Recent() []Alert {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Alert, len(e.recent))
	copy(out, e.recent)
	return out
}

func (e *Engine) evaluate(c model.Contact) []Alert {
	var out []Alert
	now := c.Seen
	where := describeWhere(c)
	if c.Emergency != "" || isEmergencySquawk(c.Squawk) {
		code := c.Emergency
		if code == "" {
			code = c.Squawk
		}
		out = append(out, Alert{
			Rule: "emergency", ContactID: c.ID, Label: c.Label, Kind: c.Kind, Time: now,
			Title:   "Emergency " + code + ": " + c.Label,
			Message: joinParts(emergencyText(code), where),
		})
	}
	if c.Military {
		out = append(out, Alert{
			Rule: "military", ContactID: c.ID, Label: c.Label, Kind: c.Kind, Time: now,
			Title:   "Military: " + c.Label,
			Message: joinParts(c.Type, c.Reg, where),
		})
	}
	if e.watch[strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(c.ID, "a:"), "v:"))] {
		out = append(out, Alert{
			Rule: "watchlist", ContactID: c.ID, Label: c.Label, Kind: c.Kind, Time: now,
			Title:   "Watch list: " + c.Label,
			Message: joinParts(c.Type, c.Reg, where),
		})
	}
	return out
}

func (e *Engine) shouldFire(a Alert) bool {
	key := a.Rule + "|" + a.ContactID
	e.mu.Lock()
	defer e.mu.Unlock()
	if last, ok := e.fired[key]; ok && a.Time.Sub(last) < e.opts.Cooldown {
		return false
	}
	e.fired[key] = a.Time
	e.recent = append(e.recent, a)
	if len(e.recent) > e.opts.Keep {
		e.recent = e.recent[len(e.recent)-e.opts.Keep:]
	}
	return true
}

func isEmergencySquawk(s string) bool {
	return s == "7500" || s == "7600" || s == "7700"
}

func emergencyText(code string) string {
	switch code {
	case "7500":
		return "Hijack squawk."
	case "7600":
		return "Radio failure squawk."
	case "7700":
		return "General emergency squawk."
	default:
		return "Emergency: " + code + "."
	}
}

func describeWhere(c model.Contact) string {
	var parts []string
	if c.AltFt != nil {
		parts = append(parts, formatInt(*c.AltFt)+" ft")
	}
	if c.SpeedKt != nil {
		parts = append(parts, formatInt(int(*c.SpeedKt))+" kt")
	}
	parts = append(parts, formatInt(int(c.DistanceKm))+" km from home")
	return strings.Join(parts, ", ")
}

func formatInt(n int) string { return strconv.Itoa(n) }

func joinParts(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " ")
}
