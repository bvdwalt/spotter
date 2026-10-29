// Package alert raises notifications for contacts on the watch list.
package alert

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

type Alert struct {
	Rule      string     `json:"rule"` // watchlist
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

// watchKey strips the kind prefix so hex codes and MMSIs match the watch list as typed.
func watchKey(contactID string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(contactID, "a:"), "v:"))
}

// Watch adds a contact id (or bare hex/MMSI) to the watch list; it fires on the next update.
func (e *Engine) Watch(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.watch[watchKey(id)] = true
}

// Unwatch removes a contact and forgets its cooldown so a later Watch alerts again.
func (e *Engine) Unwatch(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.watch, watchKey(id))
	for key := range e.fired {
		if strings.HasPrefix(key, "watchlist|") && watchKey(strings.TrimPrefix(key, "watchlist|")) == watchKey(id) {
			delete(e.fired, key)
		}
	}
}

func (e *Engine) Watching(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.watch[watchKey(id)]
}

func (e *Engine) WatchList() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, 0, len(e.watch))
	for k := range e.watch {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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

// Clear drops the recent list; cooldowns stay so cleared alerts do not immediately re-fire.
func (e *Engine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recent = nil
}

// Dismiss removes one alert from the recent list, matched by rule, contact, and time.
func (e *Engine) Dismiss(rule, contactID string, at time.Time) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, a := range e.recent {
		if a.Rule == rule && a.ContactID == contactID && a.Time.Equal(at) {
			e.recent = append(e.recent[:i], e.recent[i+1:]...)
			return true
		}
	}
	return false
}

func (e *Engine) evaluate(c model.Contact) []Alert {
	if !e.Watching(c.ID) {
		return nil
	}
	return []Alert{{
		Rule: "watchlist", ContactID: c.ID, Label: c.Label, Kind: c.Kind, Time: c.Seen,
		Title:   "Watch list: " + c.Label,
		Message: joinParts(c.Type, c.Reg, describeWhere(c)),
	}}
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
