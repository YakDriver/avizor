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
	// A new argument whose name the calling code doesn't use, and one it does.
	missing := enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateThingRequest.Color", Operations: []string{"CreateThing"}, UsedBy: []string{"thing.go"}, Member: "color"}
	found := missing
	found.FoundIn = []string{"thing.go"}
	policies := enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateThingRequest.Policies", Operations: []string{"CreateThing"}, UsedBy: []string{"thing.go"}, Member: "policies"}
	gadget := enrich.Work{Kind: enrich.WorkNewResource, Subject: "Gadget", Operations: []string{"CreateGadget", "DeleteGadget"}}
	trackedText := map[int]string{7: "Track the color argument"}
	// A member named in a candidate about another resource, or put out of scope, isn't tracked by it (#80).
	thingColor := missing
	thingColor.Resources = []string{"aws_widget_thing", "aws_widget_thing_data"}
	titled := func(title string) *reconcile.Result {
		return &reconcile.Result{Candidates: []reconcile.Candidate{{Category: reconcile.CategoryTracked, Number: 7, Title: title}}}
	}
	otherResource := titled("r/aws_widget_gadget: Add size")
	ownResource := titled("r/aws_widget_thing: Add size")
	colorTitle := titled("r/aws_widget_gadget: Add color")
	untitled := titled("Widget size")
	oneWord := titled("r/aws_instance: Add size")
	servicePrefix := titled("r/aws_widget: Support colors")
	policyColor := missing
	policyColor.Resources = []string{"aws_widget_thing_policy"}
	mentions := map[int]string{7: "Add size.\n\nThe same release added color to things."}
	outOfScope := map[int]string{7: "Add size.\n\nThe same release added color to things. That is a separate change and not part of this request."}
	release := "https://github.com/aws/aws-sdk-go-v2/commit/abc"
	tracker := &reconcile.Result{Candidates: []reconcile.Candidate{{Category: reconcile.CategoryTracked, Kind: reconcile.KindIssue, Number: 7,
		Signals: []reconcile.Match{{Type: "url", Value: release, Strength: reconcile.StrengthStrong}}}}}

	for name, tc := range map[string]struct {
		notes    []string
		en       enrich.Result
		rec      *reconcile.Result
		texts    map[int]string
		noSearch bool
		state    string
		reason   string
	}{
		"gap":              {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used, unused}}, rec: related, state: Gap, reason: "work in code"},
		"gap new resource": {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{resource}}, state: Gap, reason: "new resource"},
		"gap new ephemeral resource": {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{{Kind: enrich.WorkNewEphemeral, Subject: "Token"}}},
			state: Gap, reason: "new ephemeral resource"},
		"gap without code":   {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{unused}}, noSearch: true, state: Gap, reason: "code not searched"},
		"gap, search failed": {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{unused}, Warnings: []string{"loading model: boom"}}, noSearch: true, state: Gap, reason: "code not searched: loading model: boom"},
		"pending":            {en: enrich.Result{Service: impl, Availability: pending, Work: []enrich.Work{used}}, state: Pending, reason: "needs m v2"},
		"covered":            {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used}}, rec: tracked, state: Covered, reason: "already tracked #7"},
		"partial argument":   {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{missing}}, rec: tracked, state: Partial, reason: "argument color"},
		"partial resource":   {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used, gadget}}, rec: tracked, state: Partial, reason: "resource Gadget"},
		"argument found":     {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{found}}, rec: tracked, state: Covered},
		"argument named":     {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{missing}}, rec: tracked, texts: trackedText, state: Covered},
		"argument named in another resource's request": {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: otherResource, texts: mentions, state: Partial, reason: "argument color"},
		"argument named in its resource's request":     {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: ownResource, texts: mentions, state: Covered},
		"argument in another resource's title":         {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: colorTitle, texts: map[int]string{7: "Add color"}, state: Covered},
		"argument named, no type in the title":         {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: untitled, texts: mentions, state: Covered},
		"argument named out of scope":                  {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: untitled, texts: outOfScope, state: Partial, reason: "argument color"},
		"argument named, one-word type":                {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: oneWord, texts: mentions, state: Partial, reason: "argument color"},
		"out of scope in another CRLF paragraph":       {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: untitled, texts: map[int]string{7: "Add color.\r\n\r\nSize is a separate change."}, state: Covered},
		"argument named under the service prefix":      {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: servicePrefix, texts: mentions, state: Covered},
		"out of scope after a whitespace line":         {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{thingColor}}, rec: untitled, texts: map[int]string{7: "Add color.\n \t\nSize is a separate change."}, state: Covered},
		"argument named under a shorter type":          {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{policyColor}}, rec: ownResource, texts: mentions, state: Partial, reason: "argument color"},
		"argument named, resources unknown":            {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{missing}}, rec: otherResource, texts: mentions, state: Covered},
		"resource noun named":                          {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used, gadget}}, rec: tracked, texts: map[int]string{7: "Support the Gadget API"}, state: Covered},
		"resource type named":                          {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used, gadget}}, rec: tracked, texts: map[int]string{7: "New resource: aws_widget_gadget"}, state: Covered},
		"resource name in a word":                      {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{used, gadget}}, rec: tracked, texts: map[int]string{7: "aws_widget_gadget_policy"}, state: Partial},
		"plural argument named":                        {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{policies}}, rec: tracked, texts: map[int]string{7: "Add a policy block"}, state: Covered},
		"release tracker":                              {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{missing, gadget}}, rec: tracker, state: Covered},
		"no code":                                      {en: enrich.Result{Service: impl, Availability: available, Work: []enrich.Work{unused}}, state: NoCode},
		"no code is pending":                           {en: enrich.Result{Service: impl, Availability: pending, Work: []enrich.Work{unused}}, state: NoCode},
		"no work":                                      {en: enrich.Result{Service: impl, Availability: available}, state: NoWork},
		"new service":                                  {notes: []string{draft.NewModuleNote, "Initial release."}, en: enrich.Result{Service: none, Work: []enrich.Work{resource}}, state: NewService, reason: "new SDK module; no service"},
		"unimplemented":                                {en: enrich.Result{Service: none, Work: []enrich.Work{resource}}, state: Unimplemented, reason: "no service uses"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			it := &awssdk.Item{ID: "sdk/widget/release-1", Service: awssdk.Service{Module: "widget"}, Release: awssdk.Release{CommitURL: release}}
			for _, n := range tc.notes {
				it.Notes = append(it.Notes, awssdk.Note{Text: n})
			}
			tc.en.CodeSearched = !tc.noSearch
			e := Classify(it, &tc.en, tc.rec, &draft.Draft{Title: "T"}, tc.texts)
			if e.State != tc.state || !strings.Contains(e.Reason, tc.reason) {
				t.Errorf("state = %s (%s), want %s (%s)", e.State, e.Reason, tc.state, tc.reason)
			}
			if Shown(e.State) != (tc.state == Gap || tc.state == Pending || tc.state == NewService || tc.state == Partial) {
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
	en.CodeSearched = true
	e := Classify(it, en, nil, d, nil)
	if e.Service != "widget" || e.Work[enrich.WorkNewArgument] != 2 || e.Work[enrich.WorkNewEnumValue] != 1 || e.Title != "T" || len(e.Relates) != 1 {
		t.Errorf("entry = %+v", e)
	}
}

// An argument with several work entries is uncovered when any entry is, and then every entry is in the work,
// whichever comes first (#80).
func TestUncoveredOrder(t *testing.T) {
	t.Parallel()

	impl := enrich.ServiceMatch{Status: enrich.ServiceImplemented, Package: "widget"}
	create := enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateThingRequest.Color", Operations: []string{"CreateThing"},
		UsedBy: []string{"thing.go"}, Member: "color", Resources: []string{"aws_widget_thing"}}
	// The same member, from a gadget operation, named by a request about the gadget only.
	gadget := create
	gadget.Subject, gadget.Operations, gadget.Resources = "CreateGadgetRequest.Color", []string{"CreateGadget"}, []string{"aws_widget_gadget"}
	rec := &reconcile.Result{Candidates: []reconcile.Candidate{{Category: reconcile.CategoryTracked, Number: 7, Title: "r/aws_widget_gadget: Add size"}}}
	texts := map[int]string{7: "Add size and color."}
	it := &awssdk.Item{ID: "sdk/widget/release-1"}
	for _, order := range [][]enrich.Work{{create, gadget}, {gadget, create}} {
		en := &enrich.Result{Service: impl, CodeSearched: true, Work: order}
		ws, labels := Uncovered(it, en, rec, texts)
		if len(ws) != 2 || len(labels) != 1 || labels[0] != "argument color" {
			t.Errorf("order %s, %s: work %d, labels %v; want 2, [argument color]", order[0].Subject, order[1].Subject, len(ws), labels)
		}
	}
}

// TestUncoveredEphemeral checks that an ephemeral resource no code calls or candidate names is uncovered, though it
// has no delete, and that a candidate naming it covers it.
func TestUncoveredEphemeral(t *testing.T) {
	t.Parallel()

	impl := enrich.ServiceMatch{Status: enrich.ServiceImplemented, Package: "widget"}
	token := enrich.Work{Kind: enrich.WorkNewEphemeral, Subject: "BearerToken", Operations: []string{"CreateBearerToken"}}
	en := &enrich.Result{Service: impl, CodeSearched: true, Work: []enrich.Work{token}}
	it := &awssdk.Item{ID: "sdk/widget/release-1"}
	rec := &reconcile.Result{Candidates: []reconcile.Candidate{{Category: reconcile.CategoryTracked, Number: 7, Title: "r/aws_widget_gadget: Add size"}}}
	if _, labels := Uncovered(it, en, rec, nil); !slices.Equal(labels, []string{"ephemeral resource BearerToken"}) {
		t.Errorf("labels = %v", labels)
	}
	if _, labels := Uncovered(it, en, rec, map[int]string{7: "Also add the aws_widget_bearer_token ephemeral resource."}); len(labels) != 0 {
		t.Errorf("named: labels = %v", labels)
	}
}
