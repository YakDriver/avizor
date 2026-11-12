package queue

import (
	"strings"
	"testing"

	"github.com/YakDriver/radar/internal/awssdk"
	"github.com/YakDriver/radar/internal/draft"
	"github.com/YakDriver/radar/internal/enrich"
	"github.com/YakDriver/radar/internal/reconcile"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	impl := enrich.ServiceMatch{Status: enrich.ServiceImplemented, Package: "widget", Label: "service/widget"}
	none := enrich.ServiceMatch{Status: enrich.ServiceUnknown, Basis: "no service uses module gizmo"}
	used := enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateThingRequest.Size", UsedBy: []string{"internal/service/widget/thing.go"}}
	unused := enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "DescribeJobOutput.Size"}
	resource := enrich.Work{Kind: enrich.WorkNewResource, Subject: "Gadget"}
	available := &enrich.Availability{Status: enrich.Available}
	pending := &enrich.Availability{Status: enrich.Pending, ModulePath: "m", ItemVersion: "v2", ProviderVersion: "v1"}
	tracked := &reconcile.Result{Candidates: []reconcile.Candidate{{Category: reconcile.CategoryTracked, Number: 7}, {Category: reconcile.CategoryRelated, Number: 8}}}
	related := &reconcile.Result{Candidates: []reconcile.Candidate{{Category: reconcile.CategoryRelated, Number: 8}}}

	for name, tc := range map[string]struct {
		notes        []string
		en           enrich.Result
		rec          *reconcile.Result
		codeSearched bool
		state        string
		reason       string
	}{
		"gap":                {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used, unused}}, rec: related, codeSearched: true, state: Gap, reason: "work in code"},
		"gap new resource":   {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{resource}}, codeSearched: true, state: Gap, reason: "new resource"},
		"gap without code":   {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{unused}}, state: Gap, reason: "code not searched"},
		"pending":            {en: enrich.Result{Service: impl, Availability: pending, Work: []enrich.Work{used}}, codeSearched: true, state: Pending, reason: "needs m v2"},
		"covered":            {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used}}, rec: tracked, codeSearched: true, state: Covered, reason: "already tracked #7"},
		"no code":            {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{unused}}, codeSearched: true, state: NoCode},
		"no code is pending": {en: enrich.Result{Service: impl, Availability: pending, Work: []enrich.Work{unused}}, codeSearched: true, state: NoCode},
		"no work":            {en: enrich.Result{Service: impl, Availability: available}, codeSearched: true, state: NoWork},
		"new service":        {notes: []string{draft.NewModuleNote, "Initial release."}, en: enrich.Result{Service: none, Work: []enrich.Work{resource}}, state: NewService, reason: "new SDK module; no service"},
		"unimplemented":      {en: enrich.Result{Service: none, Work: []enrich.Work{resource}}, state: Unimplemented, reason: "no service uses"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			it := &awssdk.Item{ID: "sdk/widget/release-1", Service: awssdk.Service{Module: "widget"}}
			for _, n := range tc.notes {
				it.Notes = append(it.Notes, awssdk.Note{Text: n})
			}
			e := Classify(it, &tc.en, tc.rec, &draft.Draft{Title: "T"}, tc.codeSearched)
			if e.State != tc.state || !strings.Contains(e.Reason, tc.reason) {
				t.Errorf("state = %s (%s), want %s (%s)", e.State, e.Reason, tc.state, tc.reason)
			}
			if Shown(e.State) != (tc.state == Gap || tc.state == Pending || tc.state == NewService) {
				t.Errorf("Shown(%s) = %v", e.State, Shown(e.State))
			}
		})
	}
}

func TestClassifyEntry(t *testing.T) {
	t.Parallel()

	it := &awssdk.Item{ID: "sdk/widget/release-1", Service: awssdk.Service{Module: "widgetmod"}}
	en := &enrich.Result{Service: enrich.ServiceMatch{Status: enrich.ServiceImplemented, Package: "widget"}, Work: []enrich.Work{
		{Kind: enrich.WorkNewArgument, UsedBy: []string{"x.go"}}, {Kind: enrich.WorkNewArgument}, {Kind: enrich.WorkNewEnumValue},
	}}
	d := &draft.Draft{Title: "T", Labels: []string{"enhancement"}, Relates: []draft.Relate{{Number: 3, Kind: reconcile.KindPR}}}
	e := Classify(it, en, nil, d, true)
	if e.Service != "widget" || e.Work[enrich.WorkNewArgument] != 2 || e.Work[enrich.WorkNewEnumValue] != 1 || e.Title != "T" || len(e.Relates) != 1 {
		t.Errorf("entry = %+v", e)
	}
}
