// Package items stores items produced by sources, such as one service's API changes in one SDK release.
package items

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // SQLite driver.
)

// Item statuses.
const (
	// StatusNew items have changes worth reviewing.
	StatusNew = "new"
	// StatusSkipped items were recorded with a reason but need no review, such as a release note with no API change.
	StatusSkipped = "skipped"
)

const schemaVersion = 1

const schema = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
	id           TEXT PRIMARY KEY,
	source       TEXT NOT NULL,
	service      TEXT NOT NULL,
	release      TEXT NOT NULL,
	published_at TEXT NOT NULL,
	status       TEXT NOT NULL,
	data         TEXT NOT NULL,
	pulled_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS items_published ON items(source, published_at);
CREATE INDEX IF NOT EXISTS items_service ON items(service, published_at);
`

// Record is a stored item. Data is the source-specific item, as JSON.
type Record struct {
	ID          string          `json:"id"`
	Source      string          `json:"source"`
	Service     string          `json:"service"`
	Release     string          `json:"release"`
	PublishedAt time.Time       `json:"published_at"`
	Status      string          `json:"status"`
	PulledAt    time.Time       `json:"pulled_at"`
	Data        json.RawMessage `json:"data"`
}

// Store is an open items database.
type Store struct {
	db *sql.DB
}

// Open opens or creates the items database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening items %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating items schema: %w", err)
	}
	s := &Store{db: db}
	v, err := s.Meta(context.Background(), "schema_version")
	if err != nil {
		db.Close()
		return nil, err
	}
	switch v {
	case "":
		if err := s.SetMeta(context.Background(), "schema_version", fmt.Sprint(schemaVersion)); err != nil {
			db.Close()
			return nil, err
		}
	case fmt.Sprint(schemaVersion):
	default:
		db.Close()
		return nil, fmt.Errorf("items %s has schema version %s, want %d; delete it and pull again", path, v, schemaVersion)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Meta returns a metadata value, or "" if unset.
func (s *Store) Meta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading meta %s: %w", key, err)
	}
	return v, nil
}

// SetMeta sets a metadata value.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO meta(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
		return fmt.Errorf("writing meta %s: %w", key, err)
	}
	return nil
}

// Put inserts or replaces records in one transaction.
func (s *Store) Put(ctx context.Context, recs []Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, r := range recs {
		_, err := tx.ExecContext(ctx, `
INSERT INTO items(id, source, service, release, published_at, status, data, pulled_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET source = excluded.source, service = excluded.service, release = excluded.release,
	published_at = excluded.published_at, status = excluded.status, data = excluded.data, pulled_at = excluded.pulled_at`,
			r.ID, r.Source, r.Service, r.Release, r.PublishedAt.UTC().Format(time.RFC3339), r.Status, string(r.Data),
			r.PulledAt.UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("storing item %s: %w", r.ID, err)
		}
	}
	return tx.Commit()
}

// Get returns one item, or nil if there is none.
func (s *Store) Get(ctx context.Context, id string) (*Record, error) {
	rows, err := s.query(ctx, `WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// Filter selects items for List. Zero fields match everything.
type Filter struct {
	Source  string
	Service string
	Status  string
	Since   time.Time
	Until   time.Time
	Limit   int
}

// List returns items matching the filter, newest release first.
func (s *Store) List(ctx context.Context, f Filter) ([]Record, error) {
	var (
		conds []string
		args  []any
	)
	add := func(cond string, arg any) {
		conds = append(conds, cond)
		args = append(args, arg)
	}
	if f.Source != "" {
		add("source = ?", f.Source)
	}
	if f.Service != "" {
		add("service = ?", f.Service)
	}
	if f.Status != "" {
		add("status = ?", f.Status)
	}
	if !f.Since.IsZero() {
		add("published_at >= ?", f.Since.UTC().Format(time.RFC3339))
	}
	if !f.Until.IsZero() {
		add("published_at <= ?", f.Until.UTC().Format(time.RFC3339))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	where += " ORDER BY published_at DESC, service"
	if f.Limit > 0 {
		where += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	return s.query(ctx, where, args...)
}

func (s *Store) query(ctx context.Context, where string, args ...any) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, source, service, release, published_at, status, data, pulled_at FROM items `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("reading items: %w", err)
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var (
			r                 Record
			published, pulled string
			data              string
		)
		if err := rows.Scan(&r.ID, &r.Source, &r.Service, &r.Release, &published, &r.Status, &data, &pulled); err != nil {
			return nil, fmt.Errorf("scanning item: %w", err)
		}
		r.PublishedAt, _ = time.Parse(time.RFC3339, published)
		r.PulledAt, _ = time.Parse(time.RFC3339, pulled)
		r.Data = json.RawMessage(data)
		out = append(out, r)
	}
	return out, rows.Err()
}
