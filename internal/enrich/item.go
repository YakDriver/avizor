package enrich

import (
	"fmt"
	"slices"
	"strings"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/reconcile"
)

// ReconcileItem describes an SDK item and its enrichment for reconciliation.
func ReconcileItem(it *awssdk.Item, res *Result) *reconcile.Item {
	name := it.Service.SDKID
	if name == "" {
		name = it.Service.Module
	}
	var notes []string
	for _, n := range it.Notes {
		notes = append(notes, n.Text)
	}
	title := name
	if len(notes) > 0 {
		title += ": " + notes[0]
	}

	ri := &reconcile.Item{
		ID:               it.ID,
		Title:            title,
		URL:              it.Release.URL,
		Text:             strings.Join(notes, "\n"),
		ReleaseCommitURL: it.Release.CommitURL,
	}
	if res != nil && res.Service.Status == ServiceImplemented {
		ri.Package = res.Service.Package
	}
	if res != nil && res.Availability != nil && res.Availability.Bump != nil {
		ri.BumpPR = res.Availability.Bump.PR
	}
	if it.Diff != nil {
		// Tagging operations are shared by every service, so they aren't specific to this change.
		ri.NewOperations = slices.DeleteFunc(slices.Clone(it.Diff.AddedOperations), func(op string) bool { return slices.Contains(taggingOperations, op) })
	}

	ri.Request, ri.RequestBasis = reconcile.RequestChange, "the model change has no new operations"
	if res != nil {
		var kinds []string
		for _, w := range res.Work {
			switch w.Kind {
			case WorkTagging:
				continue
			case WorkNewResource, WorkNewDataSource, WorkNewOperation:
				if !slices.Contains(kinds, w.Kind) {
					kinds = append(kinds, w.Kind)
				}
				continue
			}
			for _, op := range w.Operations {
				if !slices.Contains(ri.NewOperations, op) && !slices.Contains(ri.Operations, op) {
					ri.Operations = append(ri.Operations, op)
				}
			}
		}
		if len(kinds) > 0 {
			ri.Request = reconcile.RequestNewType
			ri.RequestBasis = fmt.Sprintf("inferred work includes %s", strings.Join(kinds, ", "))
		}
	} else if len(ri.NewOperations) > 0 {
		ri.Request, ri.RequestBasis = reconcile.RequestNewType, "the model change adds operations"
	}
	slices.Sort(ri.Operations)
	return ri
}
