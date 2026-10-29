package db

import (
	"path/filepath"
	"testing"
)

func TestConnectMigratesOnceAndUsesWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for i := 0; i < 2; i++ { // second open must not re-apply migrations
		d, err := Connect(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		var mode string
		if err := d.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
			t.Errorf("journal_mode = %q %v, want wal", mode, err)
		}
		var applied int
		if err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil || applied != 2 {
			t.Errorf("schema_migrations rows = %d %v, want 2", applied, err)
		}
		if _, err := d.Exec(`INSERT INTO positions (contact_id, kind, ts, lat, lon) VALUES ('a:1', 'aircraft', 1, 0, 0)`); err != nil {
			t.Errorf("positions table missing: %v", err)
		}
		_ = d.Close()
	}
}
