package queue

import (
	"slices"
	"strings"
	"testing"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/draft"
	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/reconcile"
)

// A partial item's draft covers only the uncovered work and relates the trackers that cover the rest (#18).
func TestDraftPartial(t *testing.T) {
	t.Parallel()

	it := &awssdk.Item{ID: "sdk/widget/release-1", Status: "new", Service: awssdk.Service{SDKID: "Widget", Module: "widget"},
		Release: awssdk.Release{Tag: "release-1"}, Notes: []awssdk.Note{{Text: "Adds gadgets and colors."}}}
	gadget := enrich.Work{Kind: enrich.WorkNewResource, Subject: "Gadget", Operations: []string{"CreateGadget", "DeleteGadget"}}
	color := enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateThingRequest.Color", Operations: []string{"CreateThing"},
		UsedBy: []string{"thing.go"}, Member: "color"}
	en := &enrich.Result{Service: enrich.ServiceMatch{Status: enrich.ServiceImplemented, Package: "widget", Label: "service/widget"},
		Availability: &enrich.Availability{Status: enrich.Available}, CodeSearched: true, Work: []enrich.Work{gadget, color}}
	rec := &reconcile.Result{Candidates: []reconcile.Candidate{
		{Category: reconcile.CategoryTracked, Kind: reconcile.KindIssue, Number: 7, Title: "New resource: aws_widget_gadget"},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindIssue, Number: 8, Title: "Other"},
	}}
	texts := map[int]string{7: "New resource: aws_widget_gadget"}

	d, err := Draft(it, en, rec, texts, draft.Options{Repo: "o/r", Prefix: "aws_"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(d.Body, "aws_widget_gadget") || !strings.Contains(d.Body, "new argument `color`") {
		t.Errorf("partial draft body has the tracked resource or lacks the uncovered argument:\n%s", d.Body)
	}
	if !strings.Contains(d.Body, "\nRelates #7\n") || strings.Contains(d.Body, "#8") {
		t.Errorf("body relates =\n%s", d.Body)
	}
	if !slices.ContainsFunc(d.Warnings, func(w string) bool {
		return strings.HasPrefix(w, "partial: the draft covers only argument color; #7 covers the rest")
	}) ||
		slices.ContainsFunc(d.Warnings, func(w string) bool { return strings.Contains(w, "already tracked") }) {
		t.Errorf("warnings = %q", d.Warnings)
	}
	if strings.HasPrefix(d.Title, "New resource") {
		t.Errorf("title = %q", d.Title)
	}
	// Classify reports the same work the draft covers.
	if e := Classify(it, en, rec, d, texts); e.State != Partial || !strings.Contains(e.Reason, "argument color;") {
		t.Errorf("state = %s (%s)", e.State, e.Reason)
	}

	// A covered item's draft is the whole item, with the trackers as warnings.
	texts[7] += " and its color argument"
	if d, err = Draft(it, en, rec, texts, draft.Options{Repo: "o/r", Prefix: "aws_"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Body, "aws_widget_gadget") || strings.Contains(d.Body, "Relates") ||
		!slices.ContainsFunc(d.Warnings, func(w string) bool { return strings.Contains(w, "already tracked: #7") }) {
		t.Errorf("covered draft:\n%s\nwarnings %q", d.Body, d.Warnings)
	}
}
