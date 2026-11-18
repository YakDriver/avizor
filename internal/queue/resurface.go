package queue

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/YakDriver/avizor/internal/index"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

// IssueLookup returns an issue or PR from the index, or nil if it isn't there.
type IssueLookup func(number int) *index.Issue

// Resurface attaches a decision to an entry and says whether the item needs another look:
//
//   - any decision whose revisit date has passed;
//   - duplicate, when a covering issue was closed without being completed, or a covering PR was closed unmerged;
//   - skipped and deferred, when the evidence differs from what the decision recorded.
//
// A created decision otherwise stands: the item's work is filed.
func Resurface(e *Entry, d items.Disposition, lookup IssueLookup, now time.Time) {
	e.Decision = &d
	var why []string
	if !d.RevisitAfter.IsZero() && !now.Before(d.RevisitAfter) {
		why = append(why, "revisit date "+d.RevisitAfter.UTC().Format(time.DateOnly)+" passed")
	}
	switch d.Decision {
	case items.DecisionDuplicate:
		// Each number covers part of the item's work (one per resource), so losing any one re-surfaces it.
		for _, n := range d.Numbers() {
			is := lookup(n)
			if is == nil || is.State != "closed" {
				continue
			}
			switch {
			case is.Kind == index.KindPR && !is.Merged:
				why = append(why, fmt.Sprintf("#%d was closed unmerged", n))
			case is.Kind == index.KindIssue && is.StateReason != "completed":
				why = append(why, fmt.Sprintf("#%d was closed as %s", n, cmp(is.StateReason, "not completed")))
			}
		}
	case items.DecisionSkipped, items.DecisionDeferred:
		var was Evidence
		if len(d.Evidence) > 0 && json.Unmarshal(d.Evidence, &was) == nil {
			why = append(why, Changes(was, e.Evidence)...)
		}
	}
	e.Resurfaced = strings.Join(why, "; ")
}

// Changes describes how evidence changed, or returns nil if it didn't. Code candidates are compared only if both
// reconciliations searched code, and the state only if both enrichments and reconciliations did the same searches,
// since a code candidate can make an item covered. Availability and issue or PR candidates are always compared.
func Changes(was, now Evidence) []string {
	var out []string
	sameCode := was.CodeCandidates == now.CodeCandidates
	if sameCode && was.CodeSearched == now.CodeSearched && was.State != now.State {
		out = append(out, fmt.Sprintf("state %s → %s", was.State, now.State))
	}
	// Evidence saved before relative announcement URLs were resolved compares as its absolute URL.
	if was.Announcements && now.Announcements && whatsnew.Absolute(was.Announcement) != whatsnew.Absolute(now.Announcement) {
		out = append(out, fmt.Sprintf("announcement %s → %s", cmp(was.Announcement, "none"), cmp(now.Announcement, "none")))
	}
	if was.Availability != now.Availability {
		out = append(out, fmt.Sprintf("availability %s → %s", cmp(was.Availability, "unknown"), cmp(now.Availability, "unknown")))
	}
	keep := func(cs []Covering) []Covering {
		if sameCode && now.CodeCandidates {
			return cs
		}
		return slices.DeleteFunc(slices.Clone(cs), func(c Covering) bool { return c.Number == 0 })
	}
	wasCov, nowCov := keep(was.Covering), keep(now.Covering)
	old := map[string]bool{}
	for _, c := range wasCov {
		old[c.String()] = true
	}
	cur := map[string]bool{}
	for _, c := range nowCov {
		cur[c.String()] = true
		if !old[c.String()] {
			out = append(out, "new: "+c.String())
		}
	}
	for _, c := range wasCov {
		if !cur[c.String()] {
			out = append(out, "gone: "+c.String())
		}
	}
	return out
}

func cmp(s, dflt string) string {
	if s == "" {
		return dflt
	}
	return s
}
