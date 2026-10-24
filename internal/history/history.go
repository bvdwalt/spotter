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
	MinInterval time.Duration // at most one row per contact this often
	Heartbeat   time.Duration // a contact that has not moved is still written this often
	MinMoveM    float64       // below this distance a contact counts as not moved
	FlushEvery  time.Duration
	PruneEvery  time.Duration
	Retention   time.Duration
}

func DefaultOptions(minInterval, retention time.Duration) Options {
	return Options{
		MinInterval: minInterval,
		Heartbeat:   5 * time.Minute,
		MinMoveM:    50,
		FlushEvery:  5 * time.Second,
		PruneEvery:  10 * time.Minute,
		Retention:   retention,
	}
}

type fix struct {
	ts       time.Time
	lat, lon float64
}

type Store struct {
	db   *sql.DB
	opts Options

	mu   sync.Mutex
	last map[string]fix
	buf  []model.Contact
}

func New(db *sql.DB, opts Options) *Store {
	return &Store{db: db, opts: opts, last: make(map[string]fix)}
}

// Record buffers a position if the contact has moved or the heartbeat is due; it never blocks on the database.
func (s *Store) Record(c model.Contact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev, ok := s.last[c.ID]; ok {
		dt := c.Seen.Sub(prev.ts)
		if dt < s.opts.MinInterval {
			return
		}
		if metres(prev.lat, prev.lon, c.Lat, c.Lon) < s.opts.MinMoveM && dt < s.opts.Heartbeat {
			return
		}
	}
	s.last[c.ID] = fix{ts: c.Seen, lat: c.Lat, lon: c.Lon}
	s.buf = append(s.buf, c)
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
			final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.Flush(final); err != nil {
				slog.Warn("history final flush failed", "err", err)
			}
			cancel()
			return
		case <-flush.C:
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
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO positions (contact_id, kind, ts, lat, lon, alt_ft, speed_kt, heading_deg) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, c := range batch {
		if _, err := stmt.ExecContext(ctx, c.ID, string(c.Kind), c.Seen.UnixMilli(), c.Lat, c.Lon, c.AltFt, c.SpeedKt, c.HeadingDeg); err != nil {
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
	for id, f := range s.last {
		if f.ts.Before(cutoff) {
			delete(s.last, id)
		}
	}
	s.mu.Unlock()
	return res.RowsAffected()
}

// Track returns a contact's stored positions since the given time, oldest first.
func (s *Store) Track(ctx context.Context, id string, since time.Time) ([]model.Position, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ts, lat, lon, alt_ft, speed_kt, heading_deg FROM positions WHERE contact_id = ? AND ts >= ? ORDER BY ts`, id, since.UnixMilli())
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
		)
		if err := rows.Scan(&ts, &p.Lat, &p.Lon, &alt, &speed, &heading); err != nil {
			return nil, err
		}
		p.Time = time.UnixMilli(ts)
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
