package engine

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Simple SQLite-based persister prototype.
type Persister struct {
	db *sql.DB
}

// NewPersister opens (or creates) the SQLite database at path.
// Use ":memory:" for an in-memory DB for tests.
func NewPersister(dsn string) (*Persister, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	p := &Persister{db: db}
	if err := p.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return p, nil
}

func (p *Persister) Close() error {
	if p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *Persister) initSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT
);
CREATE TABLE IF NOT EXISTS torrents (
  infohash TEXT PRIMARY KEY,
  name TEXT,
  magnet TEXT,
  torrent_path TEXT,
  desired_state TEXT,
  added_at DATETIME,
  updated_at DATETIME
);
`
	_, err := p.db.Exec(schema)
	return err
}

func (p *Persister) UpsertTorrent(infohash, name, magnet, torrentPath, desiredState string) error {
	now := time.Now().UTC()
	_, err := p.db.Exec(`INSERT INTO torrents(infohash,name,magnet,torrent_path,desired_state,added_at,updated_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(infohash) DO UPDATE SET
  name=excluded.name,
  magnet=excluded.magnet,
  torrent_path=excluded.torrent_path,
  desired_state=excluded.desired_state,
  updated_at=excluded.updated_at`, infohash, name, magnet, torrentPath, desiredState, now, now)
	if err != nil {
		return fmt.Errorf("upsert torrent: %w", err)
	}
	return nil
}

func (p *Persister) GetAllTorrents() ([]map[string]string, error) {
	rows, err := p.db.Query(`SELECT infohash,name,magnet,torrent_path,desired_state FROM torrents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var infohash, name, magnet, torrentPath, desiredState sql.NullString
		if err := rows.Scan(&infohash, &name, &magnet, &torrentPath, &desiredState); err != nil {
			return nil, err
		}
		m := map[string]string{}
		if infohash.Valid {
			m["infohash"] = infohash.String
		}
		if name.Valid {
			m["name"] = name.String
		}
		if magnet.Valid {
			m["magnet"] = magnet.String
		}
		if torrentPath.Valid {
			m["torrent_path"] = torrentPath.String
		}
		if desiredState.Valid {
			m["desired_state"] = desiredState.String
		}
		out = append(out, m)
	}
	return out, nil
}

func (p *Persister) DeleteTorrent(infohash string) error {
	_, err := p.db.Exec(`DELETE FROM torrents WHERE infohash = ?`, infohash)
	if err != nil {
		return fmt.Errorf("delete torrent: %w", err)
	}
	return nil
}
