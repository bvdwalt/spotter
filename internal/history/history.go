// Package history stores contact positions in SQLite so paths outlive expiry and restarts.
package history

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/bvdwalt/spotter/internal/model"
)

type Options struct {
	MinInterval    time.Duration // at most one row per contact this often
	Heartbeat      time.Duration // a contact moving in a straight line is written at least this often
	StillHeartbeat time.Duration // a contact that has not left ToleranceM of its last row is written this often
	ToleranceM     float64       // samples within this distance of the straight line between stored rows are dropped
	AltToleranceFt int           // likewise for altitude against a straight climb or descent
	Idle           time.Duration // a contact unseen this long has its newest held sample written
	FlushEvery     time.Duration
	PruneEvery     time.Duration
	Retention      time.Duration
}

func DefaultOptions(minInterval, retention time.Duration) Options {
	return Options{
		MinInterval:    minInterval,
		Heartbeat:      5 * time.Minute,
		StillHeartbeat: 30 * time.Minute,
		ToleranceM:     50,
		AltToleranceFt: 300,
		Idle:           2 * time.Minute,
		FlushEvery:     5 * time.Second,
		PruneEvery:     10 * time.Minute,
		Retention:      retention,
	}
}

// tracker is what the store remembers per contact: the last row written and the samples seen since,
// which are held back until a later sample shows they were not on a straight line.
type tracker struct {
	anchor model.Contact
	held   []model.Contact
	at     time.Time // when the newest accepted sample was seen
	// callsign is the newest non-empty one seen, so a feed that drops it now and then is not a new flight.
	callsign string
}

type Store struct {
	db   *sql.DB
	opts Options

	mu   sync.Mutex
	last map[string]*tracker
	buf  []model.Contact
}

func New(db *sql.DB, opts Options) *Store {
	return &Store{db: db, opts: opts, last: make(map[string]*tracker)}
}

// Record buffers a position unless it lies on the straight line between rows already stored; it never blocks on the database.
func (s *Store) Record(c model.Contact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.last[c.ID]
	if !ok {
		s.last[c.ID] = &tracker{anchor: c, at: c.Seen, callsign: c.Callsign}
		s.buf = append(s.buf, c)
		return
	}
	if c.Seen.Sub(t.at) < s.opts.MinInterval {
		return
	}
	// A new flight or a touchdown or takeoff marks where one leg ends, so it is stored even on a straight line.
	newFlight := c.Callsign != "" && t.callsign != "" && c.Callsign != t.callsign
	if c.Callsign != "" {
		t.callsign = c.Callsign
	}
	if newFlight || c.OnGround != t.anchor.OnGround {
		if len(t.held) > 0 {
			s.write(t, t.held[len(t.held)-1])
		}
		t.at = c.Seen
		s.write(t, c)
		return
	}
	if len(t.held) == 0 && c.Seen.Sub(t.anchor.Seen) < s.opts.StillHeartbeat &&
		metres(t.anchor.Lat, t.anchor.Lon, c.Lat, c.Lon) < s.opts.ToleranceM {
		return
	}
	if !s.onLine(t, c) {
		s.write(t, t.held[len(t.held)-1])
	}
	t.at = c.Seen
	if c.Seen.Sub(t.anchor.Seen) >= s.opts.Heartbeat {
		s.write(t, c)
		return
	}
	t.held = append(t.held, c)
}

// onLine reports whether every held sample still lies within tolerance of the line from the anchor to c.
func (s *Store) onLine(t *tracker, c model.Contact) bool {
	for _, h := range t.held {
		if offLineM(t.anchor, c, h) > s.opts.ToleranceM {
			return false
		}
		if t.anchor.AltFt != nil && c.AltFt != nil && h.AltFt != nil {
			span := c.Seen.Sub(t.anchor.Seen).Seconds()
			f := h.Seen.Sub(t.anchor.Seen).Seconds() / span
			want := float64(*t.anchor.AltFt) + f*float64(*c.AltFt-*t.anchor.AltFt)
			if math.Abs(float64(*h.AltFt)-want) > float64(s.opts.AltToleranceFt) {
				return false
			}
		}
	}
	return true
}

// write queues c as the contact's new stored row and drops the samples held before it.
func (s *Store) write(t *tracker, c model.Contact) {
	t.anchor = c
	t.held = nil
	s.buf = append(s.buf, c)
}

// releaseHeld writes the newest held sample of every contact unseen for at least idle, so a contact that
// left coverage keeps its last position. An idle of zero writes them all, for shutdown.
func (s *Store) releaseHeld(now time.Time, idle time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.last {
		if len(t.held) > 0 && now.Sub(t.at) >= idle {
			s.write(t, t.held[len(t.held)-1])
		}
	}
}

// Run flushes and prunes on timers until ctx is done, then flushes what is left.
func (s *Store) Run(ctx context.Context) {
	flush := time.NewTicker(s.opts.FlushEvery)
	prune := time.NewTicker(s.opts.PruneEvery)
	defer flush.Stop()
	defer prune.Stop()
	if _, err := s.Prune(ctx, time.Now()); err != nil {
		slog.Warn("history prune failed", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			// The run context is gone, so the final flush gets its own short deadline.
			s.releaseHeld(time.Now(), 0)
			final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.Flush(final); err != nil {
				slog.Warn("history final flush failed", "err", err)
			}
			cancel()
			return
		case now := <-flush.C:
			s.releaseHeld(now, s.opts.Idle)
			if err := s.Flush(ctx); err != nil {
				slog.Warn("history flush failed", "err", err)
			}
		case now := <-prune.C:
			if n, err := s.Prune(ctx, now); err != nil {
				slog.Warn("history prune failed", "err", err)
			} else if n > 0 {
				slog.Info("history pruned", "rows", n)
			}
		}
	}
}

// Flush writes buffered positions in one transaction; on failure they are kept for the next attempt.
func (s *Store) Flush(ctx context.Context) error {
	s.mu.Lock()
	batch := s.buf
	s.buf = nil
	s.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	if err := s.insert(ctx, batch); err != nil {
		s.mu.Lock()
		s.buf = append(batch, s.buf...)
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *Store) insert(ctx context.Context, batch []model.Contact) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO positions (contact_id, kind, ts, lat, lon, alt_ft, speed_kt, heading_deg, callsign, on_ground) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, c := range batch {
		callsign := sql.NullString{String: c.Callsign, Valid: c.Callsign != ""}
		if _, err := stmt.ExecContext(ctx, c.ID, string(c.Kind), c.Seen.UnixMilli(), c.Lat, c.Lon, c.AltFt, c.SpeedKt, c.HeadingDeg, callsign, c.OnGround); err != nil {
			return fmt.Errorf("insert %s: %w", c.ID, err)
		}
	}
	return tx.Commit()
}

// Prune deletes rows older than the retention window and forgets contacts that have aged out.
func (s *Store) Prune(ctx context.Context, now time.Time) (int64, error) {
	cutoff := now.Add(-s.opts.Retention)
	res, err := s.db.ExecContext(ctx, `DELETE FROM positions WHERE ts < ?`, cutoff.UnixMilli())
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	for id, t := range s.last {
		if t.at.Before(cutoff) {
			delete(s.last, id)
		}
	}
	s.mu.Unlock()
	return res.RowsAffected()
}

// Track returns a contact's stored positions since the given time, oldest first.
func (s *Store) Track(ctx context.Context, id string, since time.Time) ([]model.Position, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ts, lat, lon, alt_ft, speed_kt, heading_deg, callsign, on_ground FROM positions WHERE contact_id = ? AND ts >= ? ORDER BY ts`, id, since.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []model.Position
	for rows.Next() {
		var (
			ts      int64
			p       model.Position
			alt     sql.NullInt64
			speed   sql.NullFloat64
			heading sql.NullFloat64
			call    sql.NullString
		)
		if err := rows.Scan(&ts, &p.Lat, &p.Lon, &alt, &speed, &heading, &call, &p.OnGround); err != nil {
			return nil, err
		}
		p.Time = time.UnixMilli(ts)
		p.Callsign = call.String
		if alt.Valid {
			v := int(alt.Int64)
			p.AltFt = &v
		}
		if speed.Valid {
			p.SpeedKt = &speed.Float64
		}
		if heading.Valid {
			p.HeadingDeg = &heading.Float64
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func metres(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// offLineM is how far h is from the segment a to b, on a flat projection that is accurate over a few hundred km.
func offLineM(a, b, h model.Contact) float64 {
	const mPerDeg = 111195.0
	cos := math.Cos(a.Lat * math.Pi / 180)
	bx, by := (b.Lon-a.Lon)*cos*mPerDeg, (b.Lat-a.Lat)*mPerDeg
	hx, hy := (h.Lon-a.Lon)*cos*mPerDeg, (h.Lat-a.Lat)*mPerDeg
	t := 0.0
	if l2 := bx*bx + by*by; l2 > 0 {
		t = math.Max(0, math.Min(1, (hx*bx+hy*by)/l2))
	}
	return math.Hypot(hx-t*bx, hy-t*by)
}
