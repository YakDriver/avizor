package index

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

// TestSearchAsOfStable checks that records created after a view's cutoff can't change its ranking. With FTS5's bm25(),
// they did: document frequencies covered the whole table.
func TestSearchAsOfStable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	early := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	add := func(n int, title string, created time.Time) {
		t.Helper()
		tx, err := st.Begin(ctx, "o/r")
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := tx.UpsertIssue(ctx, &Issue{Number: n, Kind: KindIssue, State: "open", Title: title, Body: "x y",
			CreatedAt: created, UpdatedAt: created}, 1); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	add(1, "alpha", early)
	add(2, "beta", early)
	add(3, "beta", early)
	add(4, "gamma", early)

	order := func(s *Store) []int {
		t.Helper()
		hits, err := s.Search(ctx, []string{"alpha", "beta"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		var out []int
		for _, h := range hits {
			out = append(out, h.Number)
		}
		return out
	}
	view := st.AsOf(early.Add(24 * time.Hour))
	before := order(view)
	if len(before) != 3 || before[0] != 1 {
		t.Fatalf("ranking = %v, want the rarer term's match (#1) first", before)
	}

	for n := range 50 {
		add(100+n, "alpha", late)
	}
	if after := order(view); !slices.Equal(after, before) {
		t.Errorf("as-of ranking changed after later records were added: %v, then %v", before, after)
	}
	if full := order(st); full[0] == 1 {
		t.Errorf("full-store ranking = %v; alpha is now common, so it shouldn't rank first", full[:3])
	}
}

func TestSearchQuoting(t *testing.T) {
	t.Parallel()

	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	// FTS5 syntax in terms must not error.
	if _, err := st.Search(context.Background(), []string{`a"b`, "OR", "NEAR(", "-x", "*", ""}, 5); err != nil {
		t.Errorf("Search with syntax-like terms: %v", err)
	}
}

// Clearing a comment's signals runs once per synced comment, so it must use an index, not scan the table.
func TestDeleteCommentSignalsUsesIndex(t *testing.T) {
	t.Parallel()

	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if plan := queryPlan(t, st, deleteCommentSignals, 1); !slices.ContainsFunc(plan, func(d string) bool { return strings.Contains(d, "USING INDEX signals_comment") }) {
		t.Errorf("plan = %q, want a search using signals_comment", plan)
	}
}

// The counts run for every reconciliation, so they mustn't read every issue row.
func TestCountsUseIndexes(t *testing.T) {
	t.Parallel()

	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	cond, cargs := st.AsOf(time.Now()).asOf("i.created_at")
	for _, tc := range []struct {
		query string
		args  []any
		want  []string
	}{
		{countIssues + cond, cargs, []string{"COVERING INDEX issues_created"}},
		{hiddenIssues, cargs, []string{"COVERING INDEX issues_created"}},
		{statsQuery, nil, []string{"COVERING INDEX issues_kind", "COVERING INDEX signals_number"}},
	} {
		plan := queryPlan(t, st, tc.query, tc.args...)
		for _, w := range tc.want {
			if !slices.ContainsFunc(plan, func(d string) bool { return strings.Contains(d, w) }) {
				t.Errorf("plan for %q = %q, want %s", tc.query, plan, w)
			}
		}
		if slices.ContainsFunc(plan, func(d string) bool { return strings.HasPrefix(d, "SCAN issues") || d == "SCAN i" }) {
			t.Errorf("plan for %q = %q, scans issues", tc.query, plan)
		}
	}
}

func queryPlan(t *testing.T, st *Store, query string, args ...any) []string {
	t.Helper()
	rows, err := st.db.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestOpenMemoryMaps(t *testing.T) {
	t.Parallel()

	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var n int64
	if err := st.db.QueryRowContext(context.Background(), "PRAGMA mmap_size").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("mmap_size = 0, want the index memory-mapped")
	}
}

// With several connections, concurrent writers wait for the write lock instead of failing, and reads run meanwhile.
func TestConcurrentUse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	errs := make(chan error, 2*maxConns)
	var wg sync.WaitGroup
	for i := range maxConns {
		wg.Go(func() {
			tx, err := st.Begin(ctx, "o/r")
			if err != nil {
				errs <- err
				return
			}
			defer tx.Rollback()
			// Read before writing, as a transaction that would need to upgrade its lock does.
			if _, err := tx.tx.ExecContext(ctx, `SELECT COUNT(*) FROM issues`); err != nil {
				errs <- err
				return
			}
			if err := tx.UpsertIssue(ctx, &Issue{Number: i + 1, Kind: KindIssue, State: "open", Title: "widget", Body: "x"}, 1); err != nil {
				errs <- err
				return
			}
			errs <- tx.Commit()
		})
		wg.Go(func() {
			_, err := st.Search(ctx, []string{"widget"}, 10)
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if s, err := st.Stats(ctx); err != nil || s.Issues != maxConns {
		t.Errorf("Stats = %+v, %v; want %d issues", s, err, maxConns)
	}
}
