package queue

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/radar/internal/index"
	"github.com/YakDriver/radar/internal/items"
)

func TestResurface(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	ev := func(e Evidence) json.RawMessage { b, _ := json.Marshal(e); return b }
	gap := Evidence{CodeSearched: true, CodeCandidates: true, State: Gap, Availability: "pending"}
	issues := map[int]*index.Issue{
		1: {Kind: index.KindIssue, State: "open"},
		2: {Kind: index.KindIssue, State: "closed", StateReason: "not_planned"},
		3: {Kind: index.KindIssue, State: "closed", StateReason: "completed"},
		4: {Kind: index.KindPR, State: "closed"},
		5: {Kind: index.KindPR, State: "closed", Merged: true},
	}
	lookup := func(n int) *index.Issue { return issues[n] }

	for name, tc := range map[string]struct {
		d    items.Disposition
		now  Evidence
		want string
	}{
		"created stands":               {d: items.Disposition{Decision: items.DecisionCreated, Number: 9}, now: Evidence{State: Covered}},
		"created, revisit passed":      {d: items.Disposition{Decision: items.DecisionCreated, Number: 9, RevisitAfter: now}, want: "revisit date 2026-11-12 passed"},
		"deferred, revisit ahead":      {d: items.Disposition{Decision: items.DecisionDeferred, RevisitAfter: now.Add(time.Hour), Evidence: ev(gap)}, now: gap},
		"duplicate of open issue":      {d: items.Disposition{Decision: items.DecisionDuplicate, Number: 1}},
		"duplicate of not planned":     {d: items.Disposition{Decision: items.DecisionDuplicate, Number: 2}, want: "#2 was closed as not_planned"},
		"duplicate of completed":       {d: items.Disposition{Decision: items.DecisionDuplicate, Number: 3}},
		"duplicate of closed PR":       {d: items.Disposition{Decision: items.DecisionDuplicate, Number: 4}, want: "#4 was closed unmerged"},
		"duplicate of merged PR":       {d: items.Disposition{Decision: items.DecisionDuplicate, Number: 5}},
		"duplicate not in index":       {d: items.Disposition{Decision: items.DecisionDuplicate, Number: 6}},
		"skipped, same evidence":       {d: items.Disposition{Decision: items.DecisionSkipped, Evidence: ev(gap)}, now: gap},
		"skipped, availability change": {d: items.Disposition{Decision: items.DecisionSkipped, Evidence: ev(gap)}, now: Evidence{CodeSearched: true, CodeCandidates: true, State: Gap, Availability: "available"}, want: "availability pending → available"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			e := Entry{State: tc.now.State, Evidence: tc.now}
			Resurface(&e, tc.d, lookup, now)
			if e.Resurfaced != tc.want {
				t.Errorf("resurfaced = %q, want %q", e.Resurfaced, tc.want)
			}
			want := Decided
			if tc.want != "" {
				want = Resurfaced
			}
			if e.Group() != want || Shown(e.Group()) != (tc.want != "") {
				t.Errorf("group = %s", e.Group())
			}
		})
	}
}

func TestChanges(t *testing.T) {
	t.Parallel()

	was := Evidence{CodeSearched: true, CodeCandidates: true, State: Gap, Availability: "available", Covering: []Covering{
		{Category: "already_implemented", Path: "internal/service/x/a.go"},
		{Category: "already_tracked", Number: 5, State: "open"},
	}}
	now := Evidence{CodeSearched: true, CodeCandidates: true, State: Covered, Availability: "available", Covering: []Covering{
		{Category: "already_tracked", Number: 5, State: "closed"},
		{Category: "already_tracked", Number: 7, State: "open"},
	}}
	got := Changes(was, now)
	want := []string{"state gap → covered", "new: already tracked #5 (closed)", "new: already tracked #7 (open)",
		"gone: already implemented internal/service/x/a.go", "gone: already tracked #5 (open)"}
	if !slices.Equal(got, want) {
		t.Errorf("changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := Changes(was, was); got != nil {
		t.Errorf("no change = %q", got)
	}

	// Without a code search on one side, state and code candidates don't count.
	noCode := Evidence{State: Covered, Availability: "available", Covering: []Covering{{Category: "already_tracked", Number: 5, State: "open"}}}
	if got := Changes(was, noCode); got != nil {
		t.Errorf("changes across code search = %q", got)
	}

	// A failed model load leaves the enrichment incomplete while reconciliation still finds code. A later run
	// without code search isn't comparable on code either.
	modelFailed := Evidence{CodeCandidates: true, State: Covered, Availability: "available",
		Covering: []Covering{{Category: "already_implemented", Path: "internal/service/x/a.go"}}}
	later := Evidence{State: Gap, Availability: "available"}
	if got := Changes(modelFailed, later); got != nil {
		t.Errorf("changes after a failed model load = %q", got)
	}
}
