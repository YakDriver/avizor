// Package index stores a local, searchable copy of a target repository's issues and pull requests.
package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YakDriver/radar/internal/signal"

	_ "modernc.org/sqlite" // SQLite driver.
)

// Record kinds.
const (
	KindIssue = "issue"
	KindPR    = "pr"
)

// Where a signal was found.
const (
	WhereTitle   = "title"
	WhereBody    = "body"
	WhereComment = "comment"
)

const schemaVersion = 1

const schema = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS issues (
	number       INTEGER PRIMARY KEY,
	kind         TEXT NOT NULL,
	state        TEXT NOT NULL,
	state_reason TEXT NOT NULL DEFAULT '',
	merged       INTEGER NOT NULL DEFAULT 0,
	title        TEXT NOT NULL,
	body         TEXT NOT NULL,
	labels       TEXT NOT NULL,
	author       TEXT NOT NULL,
	url          TEXT NOT NULL,
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL,
	closed_at    TEXT NOT NULL DEFAULT '',
	sync_gen     INTEGER NOT NULL DEFAULT 0
);

CREATE VIRTUAL TABLE IF NOT EXISTS issues_fts USING fts5(
	title, body, labels,
	content='issues', content_rowid='number',
	tokenize='porter unicode61'
);

CREATE TRIGGER IF NOT EXISTS issues_ai AFTER INSERT ON issues BEGIN
	INSERT INTO issues_fts(rowid, title, body, labels) VALUES (new.number, new.title, new.body, new.labels);
END;
CREATE TRIGGER IF NOT EXISTS issues_ad AFTER DELETE ON issues BEGIN
	INSERT INTO issues_fts(issues_fts, rowid, title, body, labels) VALUES ('delete', old.number, old.title, old.body, old.labels);
END;
CREATE TRIGGER IF NOT EXISTS issues_au AFTER UPDATE OF title, body, labels ON issues BEGIN
	INSERT INTO issues_fts(issues_fts, rowid, title, body, labels) VALUES ('delete', old.number, old.title, old.body, old.labels);
	INSERT INTO issues_fts(rowid, title, body, labels) VALUES (new.number, new.title, new.body, new.labels);
END;

-- Exact-match anchors. comment_id is 0 for title and body signals.
CREATE TABLE IF NOT EXISTS signals (
	number     INTEGER NOT NULL,
	kind       TEXT NOT NULL,
	value      TEXT NOT NULL,
	where_     TEXT NOT NULL,
	comment_id INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS signals_kind_value ON signals(kind, value);
CREATE INDEX IF NOT EXISTS signals_number ON signals(number);
CREATE INDEX IF NOT EXISTS signals_comment ON signals(comment_id) WHERE comment_id <> 0;
`

// Issue is an indexed issue or pull request.
type Issue struct {
	Number      int       `json:"number"`
	Kind        string    `json:"kind"`
	State       string    `json:"state"`
	StateReason string    `json:"state_reason,omitempty"`
	Merged      bool      `json:"merged,omitempty"`
	Title       string    `json:"title"`
	Body        string    `json:"-"`
	Labels      []string  `json:"labels"`
	Author      string    `json:"author"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ClosedAt    time.Time `json:"closed_at,omitzero"`
}

// Store is an open index database.
type Store struct {
	db *sql.DB
}

// Open opens or creates the index at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening index %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating index schema: %w", err)
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
		return nil, fmt.Errorf("index %s has schema version %s, want %d; delete it and run sync again", path, v, schemaVersion)
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO meta(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("writing meta %s: %w", key, err)
	}
	return nil
}

// Tx is a write transaction.
type Tx struct {
	tx   *sql.Tx
	repo string
}

// Begin starts a write transaction. repo scopes issue reference extraction.
func (s *Store) Begin(ctx context.Context, repo string) (*Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	return &Tx{tx: tx, repo: repo}, nil
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	return t.tx.Commit()
}

// Rollback aborts the transaction. It is safe to call after Commit.
func (t *Tx) Rollback() {
	_ = t.tx.Rollback()
}

// UpsertIssue inserts or replaces an issue and its title and body signals.
func (t *Tx) UpsertIssue(ctx context.Context, is *Issue, gen int64) error {
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO issues(number, kind, state, state_reason, merged, title, body, labels, author, url, created_at, updated_at, closed_at, sync_gen)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(number) DO UPDATE SET
	kind = excluded.kind, state = excluded.state, state_reason = excluded.state_reason, merged = excluded.merged,
	title = excluded.title, body = excluded.body, labels = excluded.labels, author = excluded.author, url = excluded.url,
	created_at = excluded.created_at, updated_at = excluded.updated_at, closed_at = excluded.closed_at,
	sync_gen = excluded.sync_gen`,
		is.Number, is.Kind, is.State, is.StateReason, is.Merged, is.Title, is.Body, strings.Join(is.Labels, "\n"),
		is.Author, is.URL, formatTime(is.CreatedAt), formatTime(is.UpdatedAt), formatTime(is.ClosedAt), gen)
	if err != nil {
		return fmt.Errorf("upserting #%d: %w", is.Number, err)
	}

	if _, err := t.tx.ExecContext(ctx, `DELETE FROM signals WHERE number = ? AND comment_id = 0`, is.Number); err != nil {
		return fmt.Errorf("clearing signals for #%d: %w", is.Number, err)
	}
	if err := t.insertSignals(ctx, is.Number, 0, WhereTitle, signal.Extract(is.Title, t.repo)); err != nil {
		return err
	}
	return t.insertSignals(ctx, is.Number, 0, WhereBody, signal.Extract(is.Body, t.repo))
}

// ReplaceCommentSignals replaces the signals extracted from one comment.
func (t *Tx) ReplaceCommentSignals(ctx context.Context, number int, commentID int64, body string) error {
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM signals WHERE comment_id = ?`, commentID); err != nil {
		return fmt.Errorf("clearing signals for comment %d: %w", commentID, err)
	}
	return t.insertSignals(ctx, number, commentID, WhereComment, signal.Extract(body, t.repo))
}

func (t *Tx) insertSignals(ctx context.Context, number int, commentID int64, where string, sigs []signal.Signal) error {
	for _, sg := range sigs {
		// Self-references (a PR body linking its own URL) carry no information.
		if sg.Kind == signal.KindIssueRef && sg.Value == fmt.Sprint(number) {
			continue
		}
		if _, err := t.tx.ExecContext(ctx, `INSERT INTO signals(number, kind, value, where_, comment_id) VALUES (?, ?, ?, ?, ?)`,
			number, sg.Kind, sg.Value, where, commentID); err != nil {
			return fmt.Errorf("inserting signal for #%d: %w", number, err)
		}
	}
	return nil
}

// DeleteNotSeen removes issues whose sync generation is older than gen, after a full sync. It returns the number removed.
func (t *Tx) DeleteNotSeen(ctx context.Context, gen int64) (int64, error) {
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM signals WHERE number IN (SELECT number FROM issues WHERE sync_gen < ?)`, gen); err != nil {
		return 0, fmt.Errorf("deleting stale signals: %w", err)
	}
	res, err := t.tx.ExecContext(ctx, `DELETE FROM issues WHERE sync_gen < ?`, gen)
	if err != nil {
		return 0, fmt.Errorf("deleting stale issues: %w", err)
	}
	return res.RowsAffected()
}

const issueColumns = `number, kind, state, state_reason, merged, title, body, labels, author, url, created_at, updated_at, closed_at`

func scanIssue(sc interface{ Scan(...any) error }) (*Issue, error) {
	var (
		is                       Issue
		labels                   string
		created, updated, closed string
	)
	if err := sc.Scan(&is.Number, &is.Kind, &is.State, &is.StateReason, &is.Merged, &is.Title, &is.Body, &labels,
		&is.Author, &is.URL, &created, &updated, &closed); err != nil {
		return nil, err
	}
	if labels != "" {
		is.Labels = strings.Split(labels, "\n")
	}
	is.CreatedAt = parseTime(created)
	is.UpdatedAt = parseTime(updated)
	is.ClosedAt = parseTime(closed)
	return &is, nil
}

// Issue returns one issue, or nil if it isn't indexed.
func (s *Store) Issue(ctx context.Context, number int) (*Issue, error) {
	is, err := scanIssue(s.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE number = ?`, number))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading #%d: %w", number, err)
	}
	return is, nil
}

// Issues returns the indexed issues among numbers, keyed by number.
func (s *Store) Issues(ctx context.Context, numbers []int) (map[int]*Issue, error) {
	out := map[int]*Issue{}
	if len(numbers) == 0 {
		return out, nil
	}
	args := make([]any, len(numbers))
	for i, n := range numbers {
		args[i] = n
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE number IN (`+placeholders(len(numbers))+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("reading issues: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		is, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning issue: %w", err)
		}
		out[is.Number] = is
	}
	return out, rows.Err()
}

// SignalHit is an issue containing an exact-match signal.
type SignalHit struct {
	Number int
	Kind   string
	Value  string
	Where  string
}

// SignalCount returns how many distinct issues contain the signal.
func (s *Store) SignalCount(ctx context.Context, kind, value string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT number) FROM signals WHERE kind = ? AND value = ?`, kind, value).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting signal %s:%s: %w", kind, value, err)
	}
	return n, nil
}

// SignalHits returns the issues containing a signal, one row per distinct (number, where).
func (s *Store) SignalHits(ctx context.Context, kind, value string) ([]SignalHit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT number, where_ FROM signals WHERE kind = ? AND value = ? ORDER BY number`, kind, value)
	if err != nil {
		return nil, fmt.Errorf("finding signal %s:%s: %w", kind, value, err)
	}
	defer rows.Close()
	var out []SignalHit
	for rows.Next() {
		h := SignalHit{Kind: kind, Value: value}
		if err := rows.Scan(&h.Number, &h.Where); err != nil {
			return nil, fmt.Errorf("scanning signal hit: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Signals returns all signals recorded for an issue, including comment signals.
func (s *Store) Signals(ctx context.Context, number int) ([]SignalHit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT kind, value, where_ FROM signals WHERE number = ? ORDER BY kind, value`, number)
	if err != nil {
		return nil, fmt.Errorf("reading signals for #%d: %w", number, err)
	}
	defer rows.Close()
	var out []SignalHit
	for rows.Next() {
		h := SignalHit{Number: number}
		if err := rows.Scan(&h.Kind, &h.Value, &h.Where); err != nil {
			return nil, fmt.Errorf("scanning signal: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// SearchHit is a full-text search result. Rank is FTS5 bm25: lower is better.
type SearchHit struct {
	Number int
	Rank   float64
}

// Search runs a full-text query over titles, bodies, and labels. Each term is matched as a quoted token; terms are
// OR'ed, and titles weigh more than bodies.
func (s *Store) Search(ctx context.Context, terms []string, limit int) ([]SearchHit, error) {
	q := ftsQuery(terms)
	if q == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT rowid, bm25(issues_fts, 10.0, 1.0, 3.0) AS rank
FROM issues_fts WHERE issues_fts MATCH ?
ORDER BY rank LIMIT ?`, q, limit)
	if err != nil {
		return nil, fmt.Errorf("searching %q: %w", q, err)
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.Number, &h.Rank); err != nil {
			return nil, fmt.Errorf("scanning search hit: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ftsQuery builds an FTS5 query that ORs quoted terms. Quoting prevents terms from being read as FTS5 syntax.
func ftsQuery(terms []string) string {
	var parts []string
	for _, t := range terms {
		t = strings.ReplaceAll(t, `"`, "")
		if t = strings.TrimSpace(t); t != "" {
			parts = append(parts, `"`+t+`"`)
		}
	}
	return strings.Join(parts, " OR ")
}

// Stats summarizes the index.
type Stats struct {
	Issues  int `json:"issues"`
	PRs     int `json:"prs"`
	Signals int `json:"signals"`
}

// Stats returns record counts.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `SELECT
	(SELECT COUNT(*) FROM issues WHERE kind = 'issue'),
	(SELECT COUNT(*) FROM issues WHERE kind = 'pr'),
	(SELECT COUNT(*) FROM signals)`).Scan(&st.Issues, &st.PRs, &st.Signals)
	if err != nil {
		return st, fmt.Errorf("reading stats: %w", err)
	}
	return st, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
