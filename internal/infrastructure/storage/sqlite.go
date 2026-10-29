package storage

import (
	"database/sql"
	"encoding/json"
	_ "modernc.org/sqlite"
	"tcpconnect/internal/domain"
)

type SQLite struct{ db *sql.DB }

func Open(path string) (*SQLite, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS workspace (id INTEGER PRIMARY KEY CHECK(id=1), data TEXT NOT NULL)`); e != nil {
		db.Close()
		return nil, e
	}
	return &SQLite{db}, nil
}
func (s *SQLite) Load() (domain.Workspace, error) {
	w := domain.Workspace{Version: 1, Services: []domain.Service{}, Requests: []domain.Request{}, Functions: []domain.Function{}, Dictionaries: []domain.DictionaryFile{}}
	var b []byte
	e := s.db.QueryRow(`SELECT data FROM workspace WHERE id=1`).Scan(&b)
	if e == sql.ErrNoRows {
		return w, nil
	}
	if e != nil {
		return w, e
	}
	e = json.Unmarshal(b, &w)
	return w, e
}
func (s *SQLite) Save(w domain.Workspace) error {
	b, e := json.Marshal(w)
	if e != nil {
		return e
	}
	_, e = s.db.Exec(`INSERT INTO workspace(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, b)
	return e
}
func (s *SQLite) Close() error { return s.db.Close() }
