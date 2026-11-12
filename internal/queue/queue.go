// Package queue sorts enriched, reconciled items into the ones that need a decision and the ones that probably
// don't, with the reason for each.
package queue

import (
	"fmt"
	"slices"
	"strings"

	"github.com/YakDriver/radar/internal/awssdk"
	"github.com/YakDriver/radar/internal/draft"
	"github.com/YakDriver/radar/internal/enrich"
	"github.com/YakDriver/radar/internal/reconcile"
)

// States. Gap, Pending, and NewService are shown by default; the others are hidden.
const (
	// Gap: the item touches code the target has, or adds a new resource, and nothing confidently covers it.
	Gap = "gap"
	// Pending: a gap whose SDK version the target doesn't require yet.
	Pending = "pending"
	// NewService: a new SDK module for a service the target doesn't implement.
	NewService = "new_service"
	// Covered: a confident candidate (already tracked, implemented, or previously considered) matches the item.
	Covered = "covered"
	// NoCode: the item's work touches no code the target has and adds no new resource. In the gap sample, items like
	// these needed no provider work.
	NoCode = "no_code"
	// Unimplemented: the item's service isn't one the target implements, and it isn't a new SDK module.
	Unimplemented = "unimplemented"
	// NoWork: the item has no inferred work, such as documentation-only model changes.
	NoWork = "no_work"
)

// Shown reports whether a state is in the default queue.
func Shown(state string) bool {
	return state == Gap || state == Pending || state == NewService
}

// Entry is one item in the queue.
type Entry struct {
	Item      string `json:"item"`
	Published string `json:"published"`
	Service   string `json:"service"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
	// Work counts the inferred work by kind.
	Work map[string]int `json:"work"`
	// Covered lists the confident candidates for a covered item.
	Covered []string `json:"covered,omitempty"`
	// Title and Labels are the draft's.
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	// Relates are weak matches worth a look before filing: issues and PRs in the service that share title keywords.
	Relates []draft.Relate `json:"relates"`
	// Warnings are the enrichment's, such as a failed model load or code search.
	Warnings []string `json:"warnings,omitempty"`
}

// Classify sorts an item into a state. d is the item's draft; its title, labels, and related matches are copied.
// An item is NoCode only if en.CodeSearched: without a complete code search, nothing is known not to touch code.
func Classify(it *awssdk.Item, en *enrich.Result, rec *reconcile.Result, d *draft.Draft) Entry {
	e := Entry{
		Item:      it.ID,
		Published: it.Release.PublishedAt.Format("2006-01-02"),
		Service:   it.Service.Module,
		Work:      map[string]int{},
		Title:     d.Title,
		Labels:    d.Labels,
		Relates:   d.Relates,
		Warnings:  en.Warnings,
	}
	if en.Service.Package != "" {
		e.Service = en.Service.Package
	}
	var used, newResource bool
	for _, w := range en.Work {
		e.Work[w.Kind]++
		used = used || len(w.UsedBy) > 0
		newResource = newResource || w.Kind == enrich.WorkNewResource
	}
	if rec != nil {
		for _, c := range rec.Candidates {
			if c.Category == reconcile.CategoryRelated {
				continue
			}
			ref := c.Path
			if c.Number != 0 {
				ref = fmt.Sprintf("#%d", c.Number)
			}
			e.Covered = append(e.Covered, strings.ReplaceAll(c.Category, "_", " ")+" "+ref)
		}
	}

	newModule := slices.ContainsFunc(it.Notes, func(n awssdk.Note) bool { return n.Text == draft.NewModuleNote })
	switch {
	case len(e.Covered) > 0:
		e.State, e.Reason = Covered, strings.Join(e.Covered, ", ")
	case len(en.Work) == 0:
		e.State, e.Reason = NoWork, "no inferred work"
	case en.Service.Status != enrich.ServiceImplemented && newModule:
		e.State, e.Reason = NewService, "new SDK module; "+en.Service.Basis
	case en.Service.Status != enrich.ServiceImplemented:
		e.State, e.Reason = Unimplemented, en.Service.Basis
	case en.CodeSearched && !used && !newResource:
		e.State, e.Reason = NoCode, "no inferred work in code the target has, and no new resource"
	case en.Availability != nil && en.Availability.Status == enrich.Pending:
		e.State = Pending
		e.Reason = fmt.Sprintf("needs %s %s; the target requires %s", en.Availability.ModulePath, en.Availability.ItemVersion, en.Availability.ProviderVersion)
	default:
		e.State = Gap
		switch {
		case used:
			e.Reason = "work in code the target has"
		case newResource:
			e.Reason = "new resource"
		case len(en.Warnings) > 0:
			e.Reason = "code not searched: " + strings.Join(en.Warnings, "; ")
		default:
			e.Reason = "code not searched"
		}
	}
	return e
}
