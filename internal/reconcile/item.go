package reconcile

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/YakDriver/radar/internal/signal"
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
