package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/YakDriver/radar/internal/draft"
	"github.com/YakDriver/radar/internal/queue"
	"github.com/YakDriver/radar/internal/reconcile"
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
