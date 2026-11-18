package items

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Decisions.
const (
	// DecisionCreated: a new issue was filed for the item.
	DecisionCreated = "created"
	// DecisionDuplicate: existing work covers the item.
	DecisionDuplicate = "duplicate"
	// DecisionSkipped: the item needs no work, for a stated reason.
	DecisionSkipped = "skipped"
	// DecisionDeferred: decide later, by RevisitAfter.
	DecisionDeferred = "deferred"
	// DecisionCleared withdraws the item's previous decision.
	DecisionCleared = "cleared"
)

// Decisions lists the decisions that can be recorded.
var Decisions = []string{DecisionCreated, DecisionDuplicate, DecisionSkipped, DecisionDeferred, DecisionCleared}

const dispositionsSchema = `
CREATE TABLE IF NOT EXISTS dispositions (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	target        TEXT NOT NULL,
	item          TEXT NOT NULL,
	decision      TEXT NOT NULL,
	number        INTEGER NOT NULL DEFAULT 0,
	reason        TEXT NOT NULL DEFAULT '',
	decided_at    TEXT NOT NULL,
	decided_by    TEXT NOT NULL DEFAULT '',
	revisit_after TEXT NOT NULL DEFAULT '',
	evidence      TEXT NOT NULL DEFAULT '',
	also          TEXT NOT NULL DEFAULT '',
	comments      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS dispositions_item ON dispositions(target, item, id);
`

// Disposition is a recorded decision about an item for a target. Decisions are kept as history: the latest one for
// an item is current, and a cleared one leaves the item undecided.
type Disposition struct {
	Target   string `json:"target"`
	Item     string `json:"item"`
	Decision string `json:"decision"`
	// Number is the issue or PR filed (created) or covering the item (duplicate).
	Number int `json:"number,omitempty"`
	// Also are more issues or PRs filed or covering the item, when its work needed several (one per resource).
	Also []int `json:"also,omitempty"`
	// Comments are URLs of comments made on existing issues or PRs for the item.
	Comments     []string  `json:"comments,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	DecidedAt    time.Time `json:"decided_at"`
	DecidedBy    string    `json:"decided_by,omitempty"`
	RevisitAfter time.Time `json:"revisit_after,omitzero"`
	// Evidence is what Avizor found when the decision was made, so a later change can re-surface the item.
	Evidence json.RawMessage `json:"evidence,omitempty"`
}

// Numbers returns every issue or PR the decision names: Number, then Also.
func (d Disposition) Numbers() []int {
	if d.Number == 0 {
		return d.Also
	}
	return append([]int{d.Number}, d.Also...)
}

// dispositionColumns are columns added after the table was first created. Open adds any that an older database lacks.
var dispositionColumns = []string{"also", "comments"}

// migrateDispositions adds columns that an older dispositions table lacks.
func migrateDispositions(db *sql.DB) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info('dispositions')`)
	if err != nil {
		return fmt.Errorf("reading dispositions columns: %w", err)
	}
	have := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("reading dispositions columns: %w", err)
		}
		have[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading dispositions columns: %w", err)
	}
	for _, c := range dispositionColumns {
		if have[c] {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE dispositions ADD COLUMN ` + c + ` TEXT NOT NULL DEFAULT ''`); err != nil {
			// Another process opening the same database may have added it first.
			if ok, cerr := hasColumn(db, "dispositions", c); cerr == nil && ok {
				continue
			}
			return fmt.Errorf("adding dispositions column %s: %w", c, err)
		}
	}
	return nil
}

// hasColumn reports whether a table has a column.
func hasColumn(db *sql.DB, table, column string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
	return n > 0, err
}

// Decide records a decision.
func (s *Store) Decide(ctx context.Context, d Disposition) error {
	var revisit string
	if !d.RevisitAfter.IsZero() {
		revisit = d.RevisitAfter.UTC().Format(time.RFC3339)
	}
	also, err := jsonList(d.Also)
	if err != nil {
		return err
	}
	comments, err := jsonList(d.Comments)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO dispositions(target, item, decision, number, reason, decided_at, decided_by, revisit_after, evidence, also, comments)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Target, d.Item, d.Decision, d.Number, d.Reason, d.DecidedAt.UTC().Format(time.RFC3339), d.DecidedBy, revisit,
		string(d.Evidence), also, comments)
	if err != nil {
		return fmt.Errorf("recording decision for %s: %w", d.Item, err)
	}
	return nil
}

// jsonList encodes a list as JSON, or "" if it's empty.
func jsonList[T any](v []T) (string, error) {
	if len(v) == 0 {
		return "", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encoding decision: %w", err)
	}
	return string(b), nil
}

// Current returns each item's current decision for a target, keyed by item ID. Items whose latest decision is
// cleared are left out.
func (s *Store) Current(ctx context.Context, target string) (map[string]Disposition, error) {
	ds, err := s.queryDispositions(ctx, `
SELECT target, item, decision, number, reason, decided_at, decided_by, revisit_after, evidence, also, comments FROM dispositions d
WHERE target = ? AND id = (SELECT max(id) FROM dispositions WHERE target = d.target AND item = d.item)`, target)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Disposition, len(ds))
	for _, d := range ds {
		if d.Decision != DecisionCleared {
			out[d.Item] = d
		}
	}
	return out, nil
}

// History returns every decision recorded for a target, oldest first, optionally for one item.
func (s *Store) History(ctx context.Context, target, item string) ([]Disposition, error) {
	q := `SELECT target, item, decision, number, reason, decided_at, decided_by, revisit_after, evidence, also, comments FROM dispositions WHERE target = ?`
	args := []any{target}
	if item != "" {
		q += ` AND item = ?`
		args = append(args, item)
	}
	return s.queryDispositions(ctx, q+` ORDER BY id`, args...)
}

func (s *Store) queryDispositions(ctx context.Context, q string, args ...any) ([]Disposition, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("reading decisions: %w", err)
	}
	defer rows.Close()
	var out []Disposition
	for rows.Next() {
		var d Disposition
		var decided, revisit, evidence, also, comments string
		if err := rows.Scan(&d.Target, &d.Item, &d.Decision, &d.Number, &d.Reason, &decided, &d.DecidedBy, &revisit, &evidence,
			&also, &comments); err != nil {
			return nil, fmt.Errorf("reading decisions: %w", err)
		}
		if d.DecidedAt, err = time.Parse(time.RFC3339, decided); err != nil {
			return nil, fmt.Errorf("decision for %s: %w", d.Item, err)
		}
		if revisit != "" {
			if d.RevisitAfter, err = time.Parse(time.RFC3339, revisit); err != nil {
				return nil, fmt.Errorf("decision for %s: %w", d.Item, err)
			}
		}
		if evidence != "" {
			d.Evidence = json.RawMessage(evidence)
		}
		if also != "" {
			if err := json.Unmarshal([]byte(also), &d.Also); err != nil {
				return nil, fmt.Errorf("decision for %s: %w", d.Item, err)
			}
		}
		if comments != "" {
			if err := json.Unmarshal([]byte(comments), &d.Comments); err != nil {
				return nil, fmt.Errorf("decision for %s: %w", d.Item, err)
			}
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
