package enrich

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/services"
	"github.com/YakDriver/avizor/internal/smithy"
)

func TestWork(t *testing.T) {
	t.Parallel()

	d := &smithy.Diff{
		AddedOperations: []string{"CreateWidget", "DeleteWidget", "GetWidget", "ListWidgets", "DescribeGadget", "ListGadgets", "StartJob", "PutPolicy",
			"ListContactFlowAliases", "RegisterCapability", "DeregisterCapability", "GetCapability"},
		AddedMembers: []smithy.MemberChange{
			{Shape: "UpdateThingRequest", Member: "Size"},
			{Shape: "Thing", Member: "Status"},
			{Shape: "ThingNotFound", Member: "Reason"},
			// Only reached by a new operation: part of that operation's work.
			{Shape: "CreateWidgetRequest", Member: "Name"},
		},
		AddedEnumValues: []smithy.MemberChange{
			{Shape: "Color", Member: "BLUE", Value: "blue"},
			{Shape: "Color", Member: "GREEN", Value: "green"},
		},
		ChangedMembers:    []smithy.MemberChange{{Shape: "Thing", Member: "Count", Value: "Long", OldValue: "String"}},
		RemovedOperations: []string{"StopJob"},
	}
	usages := map[string]*smithy.Usage{
		"UpdateThingRequest":  {Input: []string{"UpdateThing"}},
		"Thing":               {Output: []string{"GetThing"}},
		"CreateWidgetRequest": {Input: []string{"CreateWidget"}},
		"Color":               {Input: []string{"UpdateThing"}, Output: []string{"GetThing"}},
	}
	type kw struct{ kind, subject string }
	var got []kw
	for _, w := range work(d, usages) {
		got = append(got, kw{w.Kind, w.Subject})
	}
	want := []kw{
		{WorkNewResource, "Widget"},
		{WorkNewDataSource, "Gadget"},
		{WorkNewOperation, "Job"},
		{WorkNewResource, "Policy"},
		{WorkNewDataSource, "ContactFlowAlias"},
		{WorkNewResource, "Capability"},
		{WorkNewArgument, "UpdateThingRequest.Size"},
		{WorkNewAttribute, "Thing.Status"},
		{WorkNewEnumValue, "Color"},
		{WorkTypeChange, "Thing.Count"},
		{WorkRemoval, "StopJob"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("work =\n%v\nwant\n%v", got, want)
	}

	// Without the model, members are reported without operations.
	ws := work(&smithy.Diff{AddedMembers: []smithy.MemberChange{{Shape: "Thing", Member: "Status"}}}, nil)
	if len(ws) != 1 || ws[0].Kind != WorkNewArgument || ws[0].Operations != nil {
		t.Errorf("work without usages = %+v", ws)
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		a, b string
		want int
		ok   bool
	}{
		{"v1.74.6", "v1.65.0", 1, true},
		{"v1.9.0", "v1.10.0", -1, true},
		{"v1.2.3", "v1.2.3", 0, true},
		{"v1.2.3-rc.1", "v1.2.3", 0, false},
		{"1.2.3", "v1.2.3", 0, false},
		{"", "v1.2.3", 0, false},
	} {
		got, ok := compareVersions(tc.a, tc.b)
		if got != tc.want || ok != tc.ok {
			t.Errorf("compareVersions(%q, %q) = %d, %t, want %d, %t", tc.a, tc.b, got, ok, tc.want, tc.ok)
		}
	}
}

// checkout builds a provider clone whose go.mod moves the widget module through versions, one commit each.
func checkout(t *testing.T, commits []struct {
	at      time.Time
	subject string
	version string
}) string {
	t.Helper()
	dir := t.TempDir()
	git := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), append(env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git(nil, "init", "-q", "-b", "main")
	for _, c := range commits {
		gomod := "module example.com/provider\n\ngo 1.27\n\nrequire (\n\tgithub.com/aws/aws-sdk-go-v2/service/widget " + c.version + "\n\tgithub.com/aws/aws-sdk-go-v2/service/other v1.0.0\n)\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600); err != nil {
			t.Fatal(err)
		}
		ts := c.at.Format(time.RFC3339)
		git(nil, "add", "-A")
		git([]string{"GIT_AUTHOR_DATE=" + ts, "GIT_COMMITTER_DATE=" + ts, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"},
			"commit", "-q", "-m", c.subject)
	}
	src := filepath.Join(dir, "internal", "service", "widget")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"widget.go":      "package widget\nfunc f() { conn.UpdateWidget(ctx, &input); widget.NewListWidgetsPaginator(conn, &in) }\n",
		"widget_test.go": "package widget\nfunc g() { conn.GetWidget(ctx, &input) }\n",
	} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestEnrich(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	day := func(d int) time.Time { return time.Date(2026, 1, d, 12, 0, 0, 0, time.UTC) }
	co := checkout(t, []struct {
		at      time.Time
		subject string
		version string
	}{
		{day(1), "Initial", "v1.1.0"},
		{day(3), "Merge pull request #10 from hashicorp/td-aws-sdk-go-v2-updates", "v1.2.0"},
		{day(5), "deps: bump modules (#11)", "v1.3.0"},
	})
	svcs := services.New(
		&services.Service{Package: "widget", Module: "widget"},
		&services.Service{Package: "gone", Module: "gone", NotImplemented: true},
	)
	e := &Enricher{Target: "tf", Services: svcs, LabelPrefix: "service/", Checkout: co}

	item := func(module, version string, published time.Time, d *smithy.Diff) *awssdk.Item {
		return &awssdk.Item{
			Service: awssdk.Service{Module: module, ModulePath: "github.com/aws/aws-sdk-go-v2/service/" + module, ModuleVersion: version},
			Release: awssdk.Release{PublishedAt: published},
			Diff:    d,
		}
	}

	// v1.2.0 was released on day 2 and first required by #10 on day 3.
	diff := &smithy.Diff{AddedMembers: []smithy.MemberChange{{Shape: "UpdateWidgetRequest", Member: "Size"}}}
	res := e.Enrich(ctx, item("widget", "v1.2.0", day(2), diff))
	if res.Service.Status != ServiceImplemented || res.Service.Label != "service/widget" {
		t.Errorf("service = %+v", res.Service)
	}
	av := res.Availability
	if av == nil || av.Status != Available || av.ProviderVersion != "v1.3.0" || av.Bump == nil || av.Bump.PR != 10 || av.Bump.Version != "v1.2.0" {
		t.Errorf("availability = %+v, bump %+v", av, av.Bump)
	}
	// Without a models clone, member work has no operations, so the code search is incomplete.
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "no model") || res.CodeSearched {
		t.Errorf("warnings = %q, code searched %v", res.Warnings, res.CodeSearched)
	}

	// The (#N) form is recognized too.
	if res := e.Enrich(ctx, item("widget", "v1.3.0", day(4), nil)); res.Availability.Bump == nil || res.Availability.Bump.PR != 11 {
		t.Errorf("bump for v1.3.0 = %+v", res.Availability.Bump)
	}

	if res := e.Enrich(ctx, item("widget", "v1.4.0", day(6), nil)); res.Availability.Status != Pending || res.Availability.Bump != nil {
		t.Errorf("v1.4.0 availability = %+v", res.Availability)
	}
	if res := e.Enrich(ctx, item("widget", "", day(6), nil)); res.Availability.Status != AvailabilityUnknown {
		t.Errorf("unversioned availability = %+v", res.Availability)
	}
	res = e.Enrich(ctx, item("gone", "v1.0.0", day(6), nil))
	if res.Service.Status != ServiceNotImplemented || res.Service.Label != "" || res.Availability.Status != NotADependency {
		t.Errorf("not-implemented service = %+v, %+v", res.Service, res.Availability)
	}
	if res := e.Enrich(ctx, item("nope", "v1.0.0", day(6), nil)); res.Service.Status != ServiceUnknown {
		t.Errorf("unknown service = %+v", res.Service)
	}
}

func TestUsedBy(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}

	co := checkout(t, []struct {
		at      time.Time
		subject string
		version string
	}{{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "Initial", "v1.0.0"}})
	e := &Enricher{Checkout: co}
	ws := []Work{
		{Kind: WorkNewArgument, Operations: []string{"UpdateWidget"}},
		{Kind: WorkNewAttribute, Operations: []string{"ListWidgets", "UpdateWidget"}},
		// Called only from a test file.
		{Kind: WorkNewAttribute, Operations: []string{"GetWidget"}},
	}
	if err := e.usedBy(context.Background(), "widget", ws); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("internal", "service", "widget", "widget.go")
	for i, want := range [][]string{{src}, {src}, nil} {
		if !slices.Equal(ws[i].UsedBy, want) {
			t.Errorf("work %d used by = %q, want %q", i, ws[i].UsedBy, want)
		}
	}
}

func TestSingular(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"Widgets": "Widget", "Policies": "Policy", "Aliases": "Alias", "Statuses": "Status", "Addresses": "Address",
		"Responses": "Response", "Boxes": "Box", "Status": "Status", "Access": "Access", "Analyses": "Analysis",
		"Alias": "Alias", "Analysis": "Analysis",
	} {
		if got := singular(in); got != want {
			t.Errorf("singular(%s) = %s, want %s", in, got, want)
		}
	}
}

// TestWorkExistingOperations checks that members and enum values are classified by operations that existed before the
// change, and that removals carry the operations still affected.
func TestWorkExistingOperations(t *testing.T) {
	t.Parallel()

	d := &smithy.Diff{
		AddedOperations:   []string{"CreateWidget"},
		AddedMembers:      []smithy.MemberChange{{Shape: "Widget", Member: "Size"}},
		RemovedMembers:    []smithy.MemberChange{{Shape: "Widget", Member: "Legacy"}},
		RemovedEnumValues: []smithy.MemberChange{{Shape: "Color", Member: "RED", Value: "red"}},
	}
	usages := map[string]*smithy.Usage{
		// The new CreateWidget takes Widget as input; the existing GetWidget returns it.
		"Widget": {Input: []string{"CreateWidget"}, Output: []string{"GetWidget"}},
		"Color":  {Input: []string{"UpdateWidget"}},
	}
	ws := work(d, usages)
	get := func(kind, subject string) *Work {
		for i := range ws {
			if ws[i].Kind == kind && ws[i].Subject == subject {
				return &ws[i]
			}
		}
		t.Fatalf("no %s %s in %+v", kind, subject, ws)
		return nil
	}
	if w := get(WorkNewAttribute, "Widget.Size"); !slices.Equal(w.Operations, []string{"GetWidget"}) {
		t.Errorf("Widget.Size operations = %v", w.Operations)
	}
	if w := get(WorkRemoval, "Widget.Legacy"); !slices.Equal(w.Operations, []string{"GetWidget"}) {
		t.Errorf("Widget.Legacy operations = %v", w.Operations)
	}
	if w := get(WorkRemoval, "Color"); !slices.Equal(w.Operations, []string{"UpdateWidget"}) {
		t.Errorf("Color operations = %v", w.Operations)
	}
}

func TestOperationWorkPlurals(t *testing.T) {
	t.Parallel()

	ws := operationWork([]string{"CreateAlias", "DeleteAlias", "ListAliases", "CreateSchema", "DeleteSchema", "ListSchemas", "GetAnalysis", "ListAnalyses"})
	var got []string
	for _, w := range ws {
		got = append(got, w.Kind+":"+w.Subject)
	}
	want := []string{WorkNewResource + ":Alias", WorkNewResource + ":Schema", WorkNewDataSource + ":Analysis"}
	if !slices.Equal(got, want) {
		t.Errorf("work = %v, want %v", got, want)
	}
}

func TestRequiredVersion(t *testing.T) {
	t.Parallel()

	gomod := `module x

go 1.27

require github.com/a/single v1.0.0 // indirect

require (
	github.com/a/b v1.2.0
	github.com/a/c v1.3.0 // indirect
)

exclude (
	github.com/a/excluded v9.0.0
)

replace github.com/a/b v1.1.0 => github.com/fork/b v1.1.1

replace (
	github.com/a/replaced v1.0.0 => ../replaced
)
`
	for path, want := range map[string]string{
		"github.com/a/single": "v1.0.0", "github.com/a/b": "v1.2.0", "github.com/a/c": "v1.3.0",
		"github.com/a/excluded": "", "github.com/a/replaced": "", "github.com/a/none": "",
	} {
		if got := requiredVersion(gomod, path); got != want {
			t.Errorf("requiredVersion(%s) = %q, want %q", path, got, want)
		}
	}
}

func TestOperationWorkTagging(t *testing.T) {
	t.Parallel()

	ws := operationWork([]string{"TagResource", "UntagResource", "ListTagsForResource", "CreateWidget"})
	if len(ws) != 2 || ws[0].Kind != WorkTagging || ws[1].Kind != WorkNewResource || ws[1].Subject != "Widget" {
		t.Errorf("work = %+v", ws)
	}
	ri := ReconcileItem(&awssdk.Item{ID: "x", Diff: &smithy.Diff{AddedOperations: []string{"TagResource", "CreateWidget"}}}, &Result{Work: ws})
	if !slices.Equal(ri.NewOperations, []string{"CreateWidget"}) {
		t.Errorf("new operations = %v", ri.NewOperations)
	}
}
