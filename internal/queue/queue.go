// Package queue sorts enriched, reconciled items into the ones that need a decision and the ones that probably
// don't, with the reason for each.
package queue

import (
	"fmt"
	"slices"
	"strings"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/draft"
	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/signal"
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
	// Partial: a covered item with work nothing accounts for: a new resource whose operations no code or confident
	// candidate matches, or a new argument whose likely Terraform name the code calling it doesn't use.
	Partial = "partial"
	// NoCode: the item's work touches no code the target has and adds no new resource. In the gap sample, items like
	// these needed no provider work.
	NoCode = "no_code"
	// Unimplemented: the item's service isn't one the target implements, and it isn't a new SDK module.
	Unimplemented = "unimplemented"
	// NoWork: the item has no inferred work, such as documentation-only model changes.
	NoWork = "no_work"
	// Decided groups items whose recorded decision still stands, and Resurfaced the decided items that need another
	// look. They're groups, not states Classify returns.
	Decided    = "decided"
	Resurfaced = "resurfaced"
)

// Shown reports whether a state or group is in the default queue.
func Shown(group string) bool {
	return group == Gap || group == Pending || group == NewService || group == Partial || group == Resurfaced
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
	// Announcement is the matched What's New post's URL, if any. Matching is an inference; see the item's enrichment.
	Announcement string `json:"announcement,omitempty"`
	// Warnings are the enrichment's, such as a failed model load or code search.
	Warnings []string `json:"warnings,omitempty"`
	// Evidence is what a decision about the item records, to notice when it changes.
	Evidence Evidence `json:"evidence"`

	// Decision is the item's current decision, if any. Resurfaced says why a decided item needs another look; it's
	// empty while the decision stands.
	Decision   *items.Disposition `json:"decision,omitempty"`
	Resurfaced string             `json:"resurfaced,omitempty"`
}

// Evidence is the part of an entry that a skipped or deferred decision depends on.
type Evidence struct {
	// CodeSearched is the enrichment's: whether the model loaded and the code search ran, which no_code depends on.
	CodeSearched bool `json:"code_searched"`
	// CodeCandidates is reconciliation's: whether it searched the checkout for code candidates without errors.
	CodeCandidates bool       `json:"code_candidates"`
	State          string     `json:"state"`
	Availability   string     `json:"availability,omitempty"`
	Covering       []Covering `json:"covering,omitempty"`
	// Announcements is whether announcements were matched against, and Announcement the match's URL, if any.
	Announcements bool   `json:"announcements,omitempty"`
	Announcement  string `json:"announcement,omitempty"`
}

// Covering is a confident candidate.
type Covering struct {
	Category string `json:"category"`
	Number   int    `json:"number,omitempty"`
	Path     string `json:"path,omitempty"`
	State    string `json:"state,omitempty"`
}

func (c Covering) String() string {
	ref := c.Path
	if c.Number != 0 {
		ref = fmt.Sprintf("#%d", c.Number)
	}
	s := strings.ReplaceAll(c.Category, "_", " ") + " " + ref
	if c.State != "" {
		s += " (" + c.State + ")"
	}
	return s
}

// Group is the entry's state, or Decided or Resurfaced for a decided item.
func (e Entry) Group() string {
	switch {
	case e.Decision == nil:
		return e.State
	case e.Resurfaced != "":
		return Resurfaced
	}
	return Decided
}

// Classify sorts an item into a state. d is the item's draft; its title, labels, and related matches are copied.
// An item is NoCode only if en.CodeSearched: without a complete code search, nothing is known not to touch code.
//
// texts holds the titles and bodies of rec's candidates by number, for finding a covered item's other work in them. Nil
// uses titles only.
func Classify(it *awssdk.Item, en *enrich.Result, rec *reconcile.Result, d *draft.Draft, texts map[int]string) Entry {
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
			cv := Covering{Category: c.Category, Number: c.Number, Path: c.Path, State: c.State}
			e.Evidence.Covering = append(e.Evidence.Covering, cv)
			e.Covered = append(e.Covered, Covering{Category: cv.Category, Number: cv.Number, Path: cv.Path}.String())
		}
		slices.SortFunc(e.Evidence.Covering, func(a, b Covering) int { return strings.Compare(a.String(), b.String()) })
	}
	if en.Availability != nil {
		e.Evidence.Availability = en.Availability.Status
	}
	if rec != nil {
		e.Evidence.CodeCandidates = rec.CodeSearched
	}
	e.Evidence.Announcements = en.AnnouncementsMatched
	if en.Announcement != nil {
		e.Announcement = en.Announcement.Announcement.URL
		e.Evidence.Announcement = e.Announcement
	}

	newModule := slices.ContainsFunc(it.Notes, func(n awssdk.Note) bool { return n.Text == draft.NewModuleNote })
	switch {
	case len(e.Covered) > 0:
		e.State, e.Reason = Covered, strings.Join(e.Covered, ", ")
		if rest := uncovered(en, rec, texts); en.CodeSearched && len(rest) > 0 && !releaseTracked(it, rec) {
			e.State, e.Reason = Partial, "no code or candidate found for "+strings.Join(rest, ", ")+"; covered: "+e.Reason
		}
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
	e.Evidence.State, e.Evidence.CodeSearched = e.State, en.CodeSearched
	return e
}

// uncovered lists a covered item's work that neither code nor a candidate accounts for:
//   - new resources (with an operation that deletes, so not tokens or presigned URLs) whose operations no code calls,
//     no confident candidate matched, and no candidate's text names;
//   - new arguments of write operations whose likely Terraform name isn't in the files calling them or in a
//     candidate's text.
func uncovered(en *enrich.Result, rec *reconcile.Result, texts map[int]string) []string {
	matched := map[string]bool{}
	var docs []string
	if rec != nil {
		for _, c := range rec.Candidates {
			doc := c.Title
			if t, ok := texts[c.Number]; ok && c.Number != 0 {
				doc = t
			}
			docs = append(docs, strings.ToLower(doc))
			if c.Category == reconcile.CategoryRelated {
				continue
			}
			for _, m := range c.Signals {
				if m.Type == signal.KindOp {
					matched[m.Value] = true
				}
			}
		}
	}
	named := func(names ...string) bool {
		return slices.ContainsFunc(docs, func(d string) bool {
			return slices.ContainsFunc(names, func(n string) bool { return n != "" && hasWord(d, strings.ToLower(n)) })
		})
	}
	var out []string
	for _, w := range en.Work {
		switch {
		case w.Kind == enrich.WorkNewResource && len(w.UsedBy) == 0 && slices.ContainsFunc(w.Operations, deletes) &&
			!slices.ContainsFunc(w.Operations, func(op string) bool { return matched[op] }) &&
			!named(append(slices.Clone(w.Operations), enrich.Snake(w.Subject))...):
			out = append(out, "resource "+w.Subject)
		case w.Member != "" && len(w.FoundIn) == 0:
			_, member, _ := strings.Cut(w.Subject, ".")
			if a := "argument " + w.Member; !slices.Contains(out, a) && !named(w.Member, strings.TrimSuffix(w.Member, "s"), member) {
				out = append(out, a)
			}
		}
	}
	return out
}

// deleteVerbs start operations that end a resource's life.
var deleteVerbs = []string{"Delete", "Deregister", "Disassociate", "Disable", "Detach", "Remove"}

func deletes(op string) bool {
	return slices.ContainsFunc(deleteVerbs, func(v string) bool { return strings.HasPrefix(op, v) })
}

// hasWord reports whether s contains w with no letter, digit, or underscore on either side, so the member
// SystemTableStatuses isn't found in ListSystemTableStatuses.
func hasWord(s, w string) bool {
	word := func(b byte) bool {
		return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
	}
	for i := 0; ; {
		j := strings.Index(s[i:], w)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(w)
		if (start == 0 || !word(s[start-1])) && (end == len(s) || !word(s[end])) {
			return true
		}
		i = start + 1
	}
}

// releaseTracked reports whether a confident issue cites the item's release commit. Maintainers file one tracker per
// launch and cite the release, so the tracker covers all of the item's work, including parts Avizor can't find.
func releaseTracked(it *awssdk.Item, rec *reconcile.Result) bool {
	if rec == nil || it.Release.CommitURL == "" {
		return false
	}
	return slices.ContainsFunc(rec.Candidates, func(c reconcile.Candidate) bool {
		return c.Kind == reconcile.KindIssue && c.Category != reconcile.CategoryRelated && slices.ContainsFunc(c.Signals, func(m reconcile.Match) bool {
			return m.Strength == reconcile.StrengthStrong && m.Value == it.Release.CommitURL
		})
	})
}
