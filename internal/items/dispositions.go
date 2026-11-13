package items

import (
	"context"
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
	evidence      TEXT NOT NULL DEFAULT ''
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
	Number       int       `json:"number,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	DecidedAt    time.Time `json:"decided_at"`
	DecidedBy    string    `json:"decided_by,omitempty"`
	RevisitAfter time.Time `json:"revisit_after,omitzero"`
	// Evidence is what Avizor found when the decision was made, so a later change can re-surface the item.
	Evidence json.RawMessage `json:"evidence,omitempty"`
}

// Decide records a decision.
func (s *Store) Decide(ctx context.Context, d Disposition) error {
	var revisit string
	if !d.RevisitAfter.IsZero() {
		revisit = d.RevisitAfter.UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO dispositions(target, item, decision, number, reason, decided_at, decided_by, revisit_after, evidence)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Target, d.Item, d.Decision, d.Number, d.Reason, d.DecidedAt.UTC().Format(time.RFC3339), d.DecidedBy, revisit, string(d.Evidence))
	if err != nil {
		return fmt.Errorf("recording decision for %s: %w", d.Item, err)
	}
	return nil
}

// Current returns each item's current decision for a target, keyed by item ID. Items whose latest decision is
// cleared are left out.
func (s *Store) Current(ctx context.Context, target string) (map[string]Disposition, error) {
	ds, err := s.queryDispositions(ctx, `
SELECT target, item, decision, number, reason, decided_at, decided_by, revisit_after, evidence FROM dispositions d
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
	q := `SELECT target, item, decision, number, reason, decided_at, decided_by, revisit_after, evidence FROM dispositions WHERE target = ?`
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
		var decided, revisit, evidence string
		if err := rows.Scan(&d.Target, &d.Item, &d.Decision, &d.Number, &d.Reason, &decided, &d.DecidedBy, &revisit, &evidence); err != nil {
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
		out = append(out, d)
	}
	return out, rows.Err()
}
