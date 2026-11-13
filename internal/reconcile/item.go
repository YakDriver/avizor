package reconcile

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/YakDriver/avizor/internal/signal"
)

// InputItem is the kind of an upstream item input, such as an AWS SDK release item.
const InputItem = "item"

// Item is an upstream change to reconcile, as the caller describes it. Reconciliation doesn't depend on where items
// come from.
type Item struct {
	ID string
	// Title is a one-line description, such as the service and its first release note.
	Title string
	URL   string
	// Text is the item's description, such as its release notes. It drives keywords and service detection.
	Text string
	// Package is the target's service package for the item, if known. Strong signals require its label.
	Package string
	// NewOperations are operations the change adds. A candidate naming one, with the service label, is a strong match.
	NewOperations []string
	// Operations are existing operations the change touches. They're weak: issues about other changes name them too.
	Operations []string
	// ReleaseCommitURL is the commit the release was cut from. Maintainers cite it in trackers, so a candidate citing
	// it in its title or body, with the service label, is a strong match. Many services share one release, so it's
	// weak without the label.
	ReleaseCommitURL string
	// Request is RequestNewType or RequestChange, and RequestBasis why.
	Request, RequestBasis string
	// BumpPR is the target PR that brought the item's SDK version in, if known. It makes the change available but
	// doesn't implement it, so it's only related, however strongly it matches.
	BumpPR int
}

// RunItem reconciles an upstream item.
func (r *Reconciler) RunItem(ctx context.Context, it *Item) (*Result, error) {
	if it == nil || it.ID == "" {
		return nil, errors.New("nothing to reconcile: no item")
	}
	r.begin()
	in := &input{
		info:       InputInfo{Kind: InputItem, Ref: it.ID, Title: it.Title, URL: it.URL},
		signalText: it.Text,
		focusText:  strings.TrimSpace(it.Title + "\n" + it.Text),
		item:       it,
	}
	if it.Package != "" {
		in.info.Labels = []string{r.Target.ServiceLabelPrefix + it.Package}
	}
	return r.run(ctx, in)
}

// itemSignals are the item's own exact signals, beyond those in its text.
func (it *Item) signals() []signal.Signal {
	var out []signal.Signal
	for _, op := range slices.Concat(it.NewOperations, it.Operations) {
		out = append(out, signal.Signal{Kind: signal.KindOp, Value: op})
	}
	if it.ReleaseCommitURL != "" {
		out = append(out, signal.Signal{Kind: signal.KindURL, Value: signal.NormalizeURL(it.ReleaseCommitURL)})
	}
	return out
}

// isReleaseCommit reports whether sg is the item's release commit URL.
func (it *Item) isReleaseCommit(sg signal.Signal) bool {
	return it.ReleaseCommitURL != "" && sg.Kind == signal.KindURL && sg.Value == signal.NormalizeURL(it.ReleaseCommitURL)
}

// strength rates an exact match for an item input before the candidate's labels are known. New operations and the
// release commit (in a title or body) are strong, and keepStrong later requires the service label for them; anything
// else is weak.
func (it *Item) strength(kind, value, where string) string {
	switch {
	case kind == signal.KindOp && slices.Contains(it.NewOperations, value):
		return StrengthStrong
	case it.isReleaseCommit(signal.Signal{Kind: kind, Value: value}) && where != "comment":
		return StrengthStrong
	}
	return StrengthWeak
}

// MatchBump marks the PR that brought in the item's SDK version.
const MatchBump = "sdk_bump"

// demoteBump makes the bump PR's matches weak and says why.
func demoteBump(c *Candidate) {
	for i := range c.Signals {
		c.Signals[i].Strength = StrengthWeak
	}
	c.addSignal(Match{Type: MatchBump, Value: "brings in the SDK version; doesn't implement the change", Strength: StrengthWeak})
}

// requireLabel downgrades a candidate's strong matches to weak unless it has the item's service label. A new operation
// name or a release commit is only specific to the item within its service: other services have operations of the same
// name, and a release spans many services.
// MatchExistingCode marks an issue or PR demoted because it matched only on operations the target's code already calls.
const MatchExistingCode = "existing_code"

// demoteExisting makes an issue or PR related when its only strong signals are operations the target's code already
// calls, or references to candidates demoted for that reason. Such a match is usually a bug report or enhancement for
// the resource that calls them, not work on the item, and the code candidate already shows that the item is
// implemented. Issues and PRs whose title asks for a new type keep their category: they're the requests and PRs that
// added it.
func demoteExisting(cands []Candidate, calls map[string]bool) {
	if len(calls) == 0 {
		return
	}
	demoted := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for i := range cands {
			c := &cands[i]
			if c.Kind == KindCode || c.Category == CategoryRelated || addsType(c.Title) {
				continue
			}
			existing := true
			for _, m := range c.Signals {
				if m.Strength != StrengthStrong {
					continue
				}
				if !(m.Type == signal.KindOp && calls[m.Value]) && !(m.Type == matchReferencesCandidate && demoted[m.Value]) {
					existing = false
					break
				}
			}
			if !existing {
				continue
			}
			for j := range c.Signals {
				c.Signals[j].Strength = StrengthWeak
			}
			c.addSignal(Match{Type: MatchExistingCode, Value: "matched only operations the target's code already calls; probably about that code", Strength: StrengthWeak})
			c.Category = CategoryRelated
			demoted["#"+strconv.Itoa(c.Number)] = true
			changed = true
		}
	}
}

var (
	// addsTypeRE matches titles like "add Autonomous Database resource" and "feat: add aws_x_y".
	addsTypeRE = regexp.MustCompile("(?i)\\badd(?:s|ed|ing)?\\s+(?:`?aws_\\w+|[\\w ]{0,40}?\\b(?:resource|data[ -]source)s?\\b)")
	// addsToTypeRE matches words that make an "add" title a change to an existing type: "add x to aws_y", "add tags
	// support", "add resource policy support".
	addsToTypeRE = regexp.MustCompile(`(?i)\b(?:to|support|supports|arguments?|attributes?|blocks?|tags?|fields?|options?|parameters?)\b`)
	newServiceRE = regexp.MustCompile(`(?i)\bnew service\b`)
	// listResourceRE matches list resources, which add listing to an existing type, not a new one.
	listResourceRE = regexp.MustCompile(`(?i)\blist[ -]resources?\b`)
	// forcesNewRE matches bug titles like "should force a new resource", which aren't requests for one.
	forcesNewRE = regexp.MustCompile(`(?i)\bforces?\s+(?:a\s+)?new\s+resource`)
)

// addsType reports whether an issue or PR title asks for or adds a new type or service, such as "New resource: aws_x",
// "add Autonomous Database resource", or "[New Service]: AWS X". Additions to an existing type, like "add tags
// support" or a list resource, don't count.
func addsType(title string) bool {
	if forcesNewRE.MatchString(title) {
		return false
	}
	title = listResourceRE.ReplaceAllString(title, "")
	if newTypeRE.MatchString(title) || newServiceRE.MatchString(title) {
		return true
	}
	return addsTypeRE.MatchString(title) && !addsToTypeRE.MatchString(title)
}

func requireLabel(c *Candidate, hasLabel bool) {
	if hasLabel {
		return
	}
	for i := range c.Signals {
		if c.Signals[i].Type == signal.KindOp || c.Signals[i].Type == signal.KindURL {
			c.Signals[i].Strength = StrengthWeak
		}
	}
}
