package items

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestDispositions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	at := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for _, d := range []Disposition{
		{Target: "tf", Item: "a", Decision: DecisionDeferred, DecidedAt: at, RevisitAfter: at.AddDate(0, 0, 30), Evidence: json.RawMessage(`{"state":"gap"}`)},
		{Target: "tf", Item: "a", Decision: DecisionCreated, Number: 5, DecidedAt: at.Add(time.Hour), DecidedBy: "me"},
		{Target: "tf", Item: "b", Decision: DecisionSkipped, Reason: "no work", DecidedAt: at},
		{Target: "tf", Item: "b", Decision: DecisionCleared, DecidedAt: at.Add(time.Hour)},
		{Target: "other", Item: "c", Decision: DecisionSkipped, Reason: "x", DecidedAt: at},
	} {
		if err := s.Decide(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	cur, err := s.Current(ctx, "tf")
	if err != nil {
		t.Fatal(err)
	}
	if len(cur) != 1 || cur["a"].Decision != DecisionCreated || cur["a"].Number != 5 || cur["a"].DecidedBy != "me" {
		t.Errorf("current = %+v", cur)
	}
	h, err := s.History(ctx, "tf", "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 2 || h[0].Decision != DecisionDeferred || !h[0].RevisitAfter.Equal(at.AddDate(0, 0, 30)) || string(h[0].Evidence) != `{"state":"gap"}` {
		t.Errorf("history = %+v", h)
	}
	if all, _ := s.History(ctx, "tf", ""); len(all) != 4 {
		t.Errorf("history for tf has %d decisions, want 4", len(all))
	}
}
