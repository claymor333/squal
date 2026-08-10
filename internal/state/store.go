package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Verdict int

const (
	LoggedOnly Verdict = iota
	Undoable
)

type Status int

const (
	Applied Status = iota
	Undone
)

type Action struct {
	ID         string
	Verdict    Verdict
	Kind       string // insert | update | delete
	Connection string
	Database   string
	Table      string
	PK         map[string]string
	Before     map[string]string
	After      map[string]string
	SQL        string
	Status     Status
	CreatedAt  time.Time
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS actions (
		id TEXT PRIMARY KEY,
		verdict INTEGER NOT NULL,
		kind TEXT NOT NULL,
		connection TEXT NOT NULL,
		database TEXT NOT NULL,
		table_name TEXT NOT NULL,
		pk_json TEXT NOT NULL,
		before_json TEXT NOT NULL,
		after_json TEXT NOT NULL,
		sql_text TEXT NOT NULL,
		status INTEGER NOT NULL,
		created_at TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS turns (
		rowid INTEGER PRIMARY KEY AUTOINCREMENT,
		connection TEXT NOT NULL,
		tool TEXT NOT NULL,
		args_json TEXT NOT NULL,
		result TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Record(a *Action) error {
	pk, _ := json.Marshal(a.PK)
	before, _ := json.Marshal(a.Before)
	after, _ := json.Marshal(a.After)
	_, err := s.db.Exec(`INSERT INTO actions
		(id, verdict, kind, connection, database, table_name, pk_json, before_json, after_json, sql_text, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Verdict, a.Kind, a.Connection, a.Database, a.Table,
		string(pk), string(before), string(after), a.SQL, a.Status, a.CreatedAt.Format(time.RFC3339))
	return err
}

func (s *Store) SetStatus(id string, st Status) error {
	_, err := s.db.Exec("UPDATE actions SET status = ? WHERE id = ?", st, id)
	return err
}

func (s *Store) Find(id string) (*Action, error) {
	row := s.db.QueryRow(`SELECT id, verdict, kind, connection, database, table_name,
		pk_json, before_json, after_json, sql_text, status, created_at FROM actions WHERE id = ?`, id)
	return scanAction(row)
}

func (s *Store) List(limit int) ([]*Action, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT id, verdict, kind, connection, database, table_name,
		pk_json, before_json, after_json, sql_text, status, created_at
		FROM actions ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Action
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAction(r rowScanner) (*Action, error) {
	var a Action
	var pk, before, after, createdAt string
	err := r.Scan(&a.ID, &a.Verdict, &a.Kind, &a.Connection, &a.Database, &a.Table,
		&pk, &before, &after, &a.SQL, &a.Status, &createdAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(pk), &a.PK)
	_ = json.Unmarshal([]byte(before), &a.Before)
	_ = json.Unmarshal([]byte(after), &a.After)
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &a, nil
}

func (v Verdict) String() string {
	if v == Undoable {
		return "undoable"
	}
	return "logged-only"
}

func (s Status) String() string {
	if s == Undone {
		return "undone"
	}
	return "applied"
}

func NewID(kind, dbName, table string) string {
	return fmt.Sprintf("%s-%s-%s-%d", kind, dbName, table, time.Now().UnixNano())
}
