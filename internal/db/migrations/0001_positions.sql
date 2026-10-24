CREATE TABLE IF NOT EXISTS positions (
    contact_id  TEXT    NOT NULL,
    kind        TEXT    NOT NULL,
    ts          INTEGER NOT NULL, -- unix milliseconds
    lat         REAL    NOT NULL,
    lon         REAL    NOT NULL,
    alt_ft      INTEGER,
    speed_kt    REAL,
    heading_deg REAL
);
CREATE INDEX IF NOT EXISTS positions_contact_ts ON positions (contact_id, ts);
CREATE INDEX IF NOT EXISTS positions_ts ON positions (ts);
