package index

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAsOf(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	tx, err := st.Begin(ctx, "o/r")
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range []*Issue{
		{Number: 1, Kind: KindIssue, State: "closed", StateReason: "duplicate", Title: "widget support", Body: "CreateWidget",
			CreatedAt: day(1), UpdatedAt: day(5), ClosedAt: day(5)},
		{Number: 2, Kind: KindPR, State: "closed", Merged: true, MergedAt: day(4), Title: "widget resource", Body: "CreateWidget",
			CreatedAt: day(3), UpdatedAt: day(4), ClosedAt: day(4)},
	} {
		if err := tx.UpsertIssue(ctx, is, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.ReplaceCommentSignals(ctx, 1, 10, day(6), "see https://example.com/announce"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// Day 2: #2 doesn't exist yet and #1 is still open.
	v := st.AsOf(day(2))
	if is, _ := v.Issue(ctx, 2); is != nil {
		t.Error("#2 visible before it was created")
	}
	is, err := v.Issue(ctx, 1)
	if err != nil || is == nil {
		t.Fatalf("#1 = %v, %v", is, err)
	}
	if is.State != "open" || is.StateReason != "" {
		t.Errorf("#1 state as of day 2 = %s/%s, want open", is.State, is.StateReason)
	}
	if n, _ := v.SignalCount(ctx, "op", "CreateWidget"); n != 1 {
		t.Errorf("op count as of day 2 = %d, want 1", n)
	}
	if hits, _ := v.Search(ctx, []string{"widget"}, 10); len(hits) != 1 || hits[0].Number != 1 {
		t.Errorf("search as of day 2 = %+v", hits)
	}

	// Day 3.5: #2 exists but isn't merged; the comment isn't posted.
	v = st.AsOf(day(3).Add(12 * time.Hour))
	if m, _ := v.Issues(ctx, []int{2}); m[2] == nil || m[2].Merged || m[2].State != "open" {
		t.Errorf("#2 as of day 3.5 = %+v, want open and unmerged", m[2])
	}
	if hits, _ := v.SignalHits(ctx, "url", "https://example.com/announce"); len(hits) != 0 {
		t.Errorf("comment signal visible before it was posted: %+v", hits)
	}

	// The full store is unaffected.
	if is, _ := st.Issue(ctx, 1); is.State != "closed" || is.StateReason != "duplicate" {
		t.Errorf("full store #1 = %s/%s", is.State, is.StateReason)
	}
	if hits, _ := st.SignalHits(ctx, "url", "https://example.com/announce"); len(hits) != 1 {
		t.Errorf("full store comment hits = %+v", hits)
	}
}
