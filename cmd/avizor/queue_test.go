package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/draft"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/queue"
	"github.com/YakDriver/avizor/internal/reconcile"
)

func TestWorkSummary(t *testing.T) {
	t.Parallel()

	got := workSummary(map[string]int{"new_argument": 3, "new_resource": 1, "tagging": 1})
	if got != "1 resource, 3 arguments, 1 tagging" {
		t.Errorf("workSummary = %q", got)
	}
	if got := workSummary(nil); got != "-" {
		t.Errorf("empty workSummary = %q", got)
	}
	if got := truncate("abcdef", 4); got != "abc…" {
		t.Errorf("truncate = %q", got)
	}
}

func TestWriteQueueText(t *testing.T) {
	t.Parallel()

	shown := []queue.Entry{
		{Published: "2026-11-11", Item: "sdk/a/r", State: queue.Gap, Work: map[string]int{"new_argument": 1}, Title: "A: thing",
			Relates: []draft.Relate{{Number: 5, Kind: reconcile.KindIssue}, {Number: 6, Kind: reconcile.KindPR}}},
	}
	var b bytes.Buffer
	if err := writeQueueText(&b, shown, map[string]int{queue.Gap: 1, queue.Covered: 2, queue.NoCode: 3}, 6, false); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"sdk/a/r", "1 argument", "related #5, #6 (PR)", "1 of 6 item(s) shown; gap 1; hidden: covered 2, no code 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestParseRevisit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)
	for in, want := range map[string]time.Time{
		"30d":                  now.AddDate(0, 0, 30),
		"12h":                  now.Add(12 * time.Hour),
		"2027-01-02":           time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC),
		"2027-01-02T03:04:05Z": time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC),
	} {
		if got, err := parseRevisit(in, now); err != nil || !got.Equal(want) {
			t.Errorf("parseRevisit(%s) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseRevisit("soon", now); err == nil {
		t.Error("parseRevisit(soon) didn't fail")
	}
}

func TestDecisionText(t *testing.T) {
	t.Parallel()

	at := time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)
	for want, d := range map[string]items.Disposition{
		"created #5":                   {Decision: items.DecisionCreated, Number: 5},
		"duplicate of #6":              {Decision: items.DecisionDuplicate, Number: 6},
		"skipped (no work)":            {Decision: items.DecisionSkipped, Reason: "no work"},
		"deferred, revisit 2027-01-02": {Decision: items.DecisionDeferred, RevisitAfter: at},
	} {
		if got := decisionText(d); got != want {
			t.Errorf("decisionText = %q, want %q", got, want)
		}
	}
}
