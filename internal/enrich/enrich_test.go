package enrich

import (
	"context"
	"maps"
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
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestWork(t *testing.T) {
	t.Parallel()

	d := &smithy.Diff{
		AddedOperations: []string{"CreateWidget", "DeleteWidget", "GetWidget", "ListWidgets", "DescribeGadget", "ListGadgets", "StartJob", "PutPolicy",
			"ListContactFlowAliases", "RegisterCapability", "DeregisterCapability", "GetCapability",
			// A plural delete and a plural Get join the singular's group, as EC2's DeleteNetworkingFlexCostAllocationPolicies
			// and Lightsail's GetSubdomains do.
			"CreateSubdomain", "DeleteSubdomains", "GetSubdomains"},
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
	ws := work(d, usages)
	for _, w := range ws {
		got = append(got, kw{w.Kind, w.Subject})
	}
	// The enum's new values, grouped.
	if i := slices.IndexFunc(ws, func(w Work) bool { return w.Kind == WorkNewEnumValue }); i < 0 || !slices.Equal(ws[i].Values, []string{"blue", "green"}) {
		t.Errorf("enum work = %+v", ws)
	}
	want := []kw{
		{WorkNewResource, "Widget"},
		{WorkNewDataSource, "Gadget"},
		{WorkNewOperation, "Job"},
		{WorkNewResource, "Policy"},
		{WorkNewDataSource, "ContactFlowAlias"},
		{WorkNewResource, "Capability"},
		{WorkNewResource, "Subdomain"},
		{WorkNewArgument, "UpdateThingRequest.Size"},
		{WorkNewAttribute, "Thing.Status"},
		{WorkNewEnumValue, "Color"},
		{WorkTypeChange, "Thing.Count"},
		{WorkRemoval, "StopJob"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("work =\n%v\nwant\n%v", got, want)
	}

	// An argument whose shape is also in an output records the operations that return it.
	shared := work(&smithy.Diff{AddedMembers: []smithy.MemberChange{{Shape: "Color", Member: "Shade"}}}, usages)
	if len(shared) != 1 || shared[0].Kind != WorkNewArgument || !slices.Equal(shared[0].Returns, []string{"GetThing"}) {
		t.Errorf("work for a shape in input and output = %+v", shared)
	}

	// Without the model, members are reported without operations.
	ws = work(&smithy.Diff{AddedMembers: []smithy.MemberChange{{Shape: "Thing", Member: "Status"}}}, nil)
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

func TestOperationWorkCancel(t *testing.T) {
	t.Parallel()

	ws := operationWork([]string{"CreateDBCapacityReservation", "CancelDBCapacityReservation", "DescribeDBCapacityReservations", "CreateWidget"})
	var got []string
	for _, w := range ws {
		got = append(got, w.Subject+": "+w.Basis)
	}
	want := []string{
		"DBCapacityReservation: new create and cancel operations for DBCapacityReservation",
		"Widget: new create operation for Widget, without a delete",
	}
	if !slices.Equal(got, want) {
		t.Errorf("work = %q, want %q", got, want)
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

func TestFindMembers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(p, s string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("names/attr_consts_gen.go", "const (\n\tAttrKMSKeyARN = \"kms_key_arn\"\n\tAttrKMSKeyARNPrefix = \"kms_key_arn_prefix\"\n)\n")
	write("thing.go", `"secondary_interface": {}, names.AttrKMSKeyARNPrefix: {}`)
	write("other.go", `names.AttrKMSKeyARN: {}`)
	ws := []Work{
		{Kind: WorkNewArgument, Subject: "Req.SecondaryInterfaces", Operations: []string{"RunThing"}, UsedBy: []string{"thing.go"}},
		{Kind: WorkNewArgument, Subject: "Req.KmsKeyArn", Operations: []string{"CreateThing"}, UsedBy: []string{"thing.go", "other.go"}},
		{Kind: WorkNewArgument, Subject: "Req.Color", Operations: []string{"UpdateThing"}, UsedBy: []string{"thing.go"}},
		// Inputs of reads are filters, not arguments.
		{Kind: WorkNewArgument, Subject: "ListReq.Color", Operations: []string{"ListThings"}, UsedBy: []string{"thing.go"}},
	}
	(&Enricher{Checkout: dir}).findMembers(ws, nil)

	for i, want := range []struct {
		member string
		found  []string
	}{{"secondary_interfaces", []string{"thing.go"}}, {"kms_key_arn", []string{"other.go"}}, {"color", nil}, {"", nil}} {
		if ws[i].Member != want.member || !slices.Equal(ws[i].FoundIn, want.found) {
			t.Errorf("%s: member %q found in %q, want %q in %q", ws[i].Subject, ws[i].Member, ws[i].FoundIn, want.member, want.found)
		}
	}
}

// TestFindMembersBlock checks that a nested structure's member counts only inside its parent's block, from the schema
// or the model struct, as in aws_bedrock_guardrail, where tier_config is on the content and topic policies only.
func TestFindMembersBlock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := `package bedrock

func (r *guardrailResource) Schema() {
	s := schema.Schema{
		Blocks: map[string]schema.Block{
			"content_policy_config": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"tier_config": framework.SingleNestedChildObjectAttribute[contentTierModel](ctx), // "x", {
					},
				},
			},
			"contextual_grounding_policy_config": schema.ListNestedBlock{
				Description: "Unlike content policies, has no \"tier_config\".",
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"filters": schema.ListNestedBlock{},
						// "tier_config": schema.ListNestedBlock{},
					},
				},
			},
			"word_policy_config": schema.ListNestedBlock{},
			"sensitive_information_policy_config": schema.ListNestedBlock{},
			"legacy": {
				Type: schema.TypeList,
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{"mode": {Type: schema.TypeString}}},
			},
			"step": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"lambda_config": lambdaBlock(ctx),
					},
				},
			},
			"model": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"litellm_config": schema.ListNestedBlock{Attributes: map[string]schema.Attribute{"api_base": schema.StringAttribute{}}},
					},
				},
			},
			"top_level_size": schema.Int64Attribute{},
		},
	}
}

type guardrailModel struct {
	ContentPolicy    fwtypes.ListNestedObjectValueOf[contentPolicyModel] ` + "`tfsdk:\"content_policy_config\"`" + `
	GroundingPolicy  fwtypes.ListNestedObjectValueOf[groundingPolicyModel] ` + "`tfsdk:\"contextual_grounding_policy_config\"`" + `
	WordPolicy       fwtypes.ListNestedObjectValueOf[wordPolicyModel] ` + "`tfsdk:\"word_policy_config\"`" + `
}

// A tag-like comment without a struct tag: tfsdk:"contextual_grounding_policy_config"
type groundingPolicyModel struct {
	Filters fwtypes.ListNestedObjectValueOf[filterModel] ` + "`tfsdk:\"filters\"`" + `
}

type wordPolicyModel struct {
	TierConfig fwtypes.ListNestedObjectValueOf[wordTierModel] ` + "`tfsdk:\"tier_config\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "guardrail.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &smithy.Model{Shapes: map[string]smithy.Shape{
		"CreateGuardrail":        {Type: smithy.TypeOperation, Input: "CreateGuardrailRequest"},
		"CreateGuardrailRequest": {Type: smithy.TypeStructure, Members: map[string]string{"contentPolicyConfig": "ContentConfig", "contextualGroundingPolicyConfig": "GroundingConfig", "wordPolicyConfig": "WordConfig", "sensitiveInformationPolicyConfig": "SensitiveConfig", "legacy": "LegacyList", "name": "String", "steps": "StepList", "model": "ModelConfig", "newThing": "NewThing"}},
		"CreateVIF":              {Type: smithy.TypeOperation, Input: "CreateVIFInput"},
		"CreateVIFInput":         {Type: smithy.TypeStructure, Members: map[string]string{"connectionId": "String", "newVif": "NewVIF"}},
		"NewVIF":                 {Type: smithy.TypeStructure, Members: map[string]string{"topLevelSize": "Integer"}},
		"StepList":               {Type: "list", Elems: []string{"Step"}},
		"Step":                   {Type: smithy.TypeStructure, Members: map[string]string{"executionBlockConfiguration": "ExecutionBlock"}},
		"ExecutionBlock":         {Type: smithy.TypeUnion, Members: map[string]string{"lambdaConfig": "String", "rdsConfig": "String"}},
		"ModelConfig":            {Type: smithy.TypeUnion, Members: map[string]string{"liteLlmConfig": "LiteLLM", "openAiModelConfig": "OpenAI"}},
		"LiteLLM":                {Type: smithy.TypeStructure, Members: map[string]string{"apiBase": "String"}},
		"OpenAI":                 {Type: smithy.TypeStructure, Members: map[string]string{"apiBase": "String"}},
		"NewThing":               {Type: smithy.TypeStructure, Members: map[string]string{"topLevelSize": "Integer"}},
		"ContentConfig":          {Type: smithy.TypeStructure, Members: map[string]string{"tierConfig": "Tier"}},
		"GroundingConfig":        {Type: smithy.TypeStructure, Members: map[string]string{"filtersConfig": "Filters", "tierConfig": "Tier"}},
		"WordConfig":             {Type: smithy.TypeStructure, Members: map[string]string{"tierConfig": "Tier"}},
		"SensitiveConfig":        {Type: smithy.TypeStructure, Members: map[string]string{"tierConfig": "Tier"}},
		"LegacyList":             {Type: "list", Elems: []string{"Legacy"}},
		"Legacy":                 {Type: smithy.TypeStructure, Members: map[string]string{"mode": "String", "tierConfig": "Tier"}},
	}}
	op := []string{"CreateGuardrail"}
	used := []string{"guardrail.go"}
	ws := []Work{
		{Kind: WorkNewArgument, Subject: "ContentConfig.tierConfig", Operations: op, UsedBy: used},
		{Kind: WorkNewArgument, Subject: "GroundingConfig.tierConfig", Operations: op, UsedBy: used},
		{Kind: WorkNewArgument, Subject: "WordConfig.tierConfig", Operations: op, UsedBy: used},
		{Kind: WorkNewArgument, Subject: "SensitiveConfig.tierConfig", Operations: op, UsedBy: used},
		{Kind: WorkNewArgument, Subject: "Legacy.mode", Operations: op, UsedBy: used},
		{Kind: WorkNewArgument, Subject: "Legacy.tierConfig", Operations: op, UsedBy: used},
		// An operation input's members are top-level arguments: the whole file counts.
		{Kind: WorkNewArgument, Subject: "CreateGuardrailRequest.contentPolicyConfig", Operations: op, UsedBy: used},
		// A union is flattened into its parent's block (step), so its members count there.
		{Kind: WorkNewArgument, Subject: "ExecutionBlock.lambdaConfig", Operations: op, UsedBy: used},
		// A structure with no block of its own (the provider names it openai_model_config) doesn't climb to model,
		// where api_base is another structure's.
		{Kind: WorkNewArgument, Subject: "OpenAI.apiBase", Operations: op, UsedBy: used},
		// A structure with no block, beside other structures in the input, isn't flattened, as with EC2 Fleet's
		// ReservedCapacityOptions.
		{Kind: WorkNewArgument, Subject: "NewThing.topLevelSize", Operations: op, UsedBy: used},
		// A structure an operation input wraps is flattened into top-level arguments.
		{Kind: WorkNewArgument, Subject: "NewVIF.topLevelSize", Operations: []string{"CreateVIF"}, UsedBy: used},
	}
	(&Enricher{Checkout: dir}).findMembers(ws, m)

	for i, want := range []struct {
		found, outside []string
	}{
		{used, nil}, // schema entry
		{nil, used}, // tier_config is only on other blocks, a comment, and a description
		{used, nil}, // model struct
		{nil, used}, // the parent's block has no tier_config
		{used, nil}, // inline SDKv2 block of a list element
		{nil, used},
		{used, nil},
		{used, nil},
		{nil, used},
		{nil, used},
		{used, nil},
	} {
		if !slices.Equal(ws[i].FoundIn, want.found) || !slices.Equal(ws[i].OutsideBlock, want.outside) {
			t.Errorf("%s (parent %q): found in %q, outside block %q; want %q, %q", ws[i].Subject, ws[i].Parent, ws[i].FoundIn, ws[i].OutsideBlock, want.found, want.outside)
		}
	}
	// Parent names the blocks checked: the climbed-to block for a union, none for a wrapped input structure.
	for i, want := range map[int][]string{1: {"contextual_grounding_policy_config"}, 7: {"step", "steps"}, 10: nil} {
		if !slices.Equal(ws[i].Parent, want) {
			t.Errorf("%s: parent %q, want %q", ws[i].Subject, ws[i].Parent, want)
		}
	}
}

func TestNewMembers(t *testing.T) {
	t.Parallel()

	m := &smithy.Model{Shapes: map[string]smithy.Shape{
		"PutAlarm":               {Type: smithy.TypeOperation, Input: "PutAlarmInput"},
		"PutAlarmInput":          {Type: smithy.TypeStructure, Members: map[string]string{"AlarmName": "String", "ActionConfiguration": "ActionConfiguration"}},
		"ActionConfiguration":    {Type: smithy.TypeStructure, Members: map[string]string{"AlarmTagsPropagation": "Boolean", "AlarmName": "String"}},
		"CreateThing":            {Type: smithy.TypeOperation, Input: "CreateThingInput"},
		"CreateThingInput":       {Type: smithy.TypeStructure, Members: map[string]string{"ThingSetting": "String"}},
		"CreateCEInput":          {Type: smithy.TypeStructure, Members: map[string]string{"EksConfiguration": "EksConfiguration"}},
		"UpdateCEInput":          {Type: smithy.TypeStructure, Members: map[string]string{"EksConfiguration": "EksConfigurationUpdate"}},
		"EksConfiguration":       {Type: smithy.TypeStructure, Members: map[string]string{"EksClusterArn": "String"}},
		"EksConfigurationUpdate": {Type: smithy.TypeStructure, Members: map[string]string{"AccessEntry": "String", "EksClusterArn": "String"}},
	}}
	d := &smithy.Diff{
		AddedOperations: []string{"CreateThing"},
		AddedShapes:     []smithy.ShapeRef{{Name: "ActionConfiguration", Type: "structure"}, {Name: "CreateThingInput", Type: "structure"}, {Name: "EksConfigurationUpdate", Type: "structure"}},
		AddedMembers:    []smithy.MemberChange{{Shape: "PutAlarmInput", Member: "ActionConfiguration"}, {Shape: "UpdateCEInput", Member: "EksConfiguration"}},
	}
	ws := []Work{
		{Kind: WorkNewArgument, Subject: "PutAlarmInput.ActionConfiguration"},
		{Kind: WorkNewArgument, Subject: "UpdateCEInput.EksConfiguration"},
	}
	markSharedNames(ws, d, m)
	if ws[0].SharedName || !ws[1].SharedName {
		t.Errorf("shared names = %v, %v; want false, true", ws[0].SharedName, ws[1].SharedName)
	}
	// AlarmTagsPropagation and AccessEntry are in new structures that new members target, even when the member's own
	// name is shared, as EksConfiguration is with the create input. AlarmName and EksClusterArn already exist, and
	// ThingSetting is a new operation's parameter.
	if got, want := newMembers(ws, d, m), []string{"ActionConfiguration", "AlarmTagsPropagation", "AccessEntry"}; !slices.Equal(got, want) {
		t.Errorf("newMembers = %v, want %v", got, want)
	}
}

func TestReconcileItemAnnouncementFloor(t *testing.T) {
	t.Parallel()

	for score, want := range map[float64]string{0.42: "", 0.57: "https://aws.amazon.com/x"} {
		res := &Result{Announcement: &whatsnew.Match{Announcement: whatsnew.Announcement{URL: "https://aws.amazon.com/x"}, Score: score}}
		if got := ReconcileItem(&awssdk.Item{ID: "x"}, res).AnnouncementURL; got != want {
			t.Errorf("score %.2f: announcement URL = %q, want %q", score, got, want)
		}
	}
}

// TestMarkNotSettable checks wording from the API models: members documented as not settable become attributes, and
// looser wording that describes real arguments doesn't.
func TestMarkNotSettable(t *testing.T) {
	t.Parallel()

	docs := map[string]string{
		"SystemTableStatuses":  "<p>The read-only enablement status of each system table for the account. This field is returned by <code>GetWorkGroup</code> and is ignored on write.</p>",
		"SelectedInstanceType": "<p>The instance type, set after an instance type is selected. This field is read-only and isn't accepted in <code>CreateTrainingJob</code> requests.</p>",
		"HealthStatus":         "<p>A dataflow endpoint health status. This field is ignored when calling <code>CreateDataflowEndpointGroup</code>.</p>",
		"LastModified":         "<p>Engines use it to verify that the resource hasn't changed. This is an output-only field.</p>",
		"EffectiveVersion":     "<p>Read only. The engine version on which the query runs.</p>",
		"Name":                 "<p>The name. The <code>guardDutyActivate</code> parameter is read-only in Amazon ECS.</p>",
		"GrantType":            "<p>DelegateGrant - Gives the grantee read-only visibility into the license.</p>",
		"Visibility":           "<p>This value cannot be changed once the API has been created.</p>",
		"AllocationStrategy":   "<p>The only supported value is <code>prioritized</code>.</p>",
		"TargetNamespace":      "<p>Use this parameter only for table buckets; it is not accepted for other target types.</p>",
	}
	m := &smithy.Model{Shapes: map[string]smithy.Shape{"Config": {Type: smithy.TypeStructure, Docs: docs}}}
	var ws []Work
	for _, member := range slices.Sorted(maps.Keys(docs)) {
		ws = append(ws, Work{Kind: WorkNewArgument, Subject: "Config." + member, Basis: "new member"})
	}
	ws = append(ws, Work{Kind: WorkNewArgument, Subject: "Config.Undocumented", Basis: "new member"})
	markNotSettable(ws, m)

	notSettable := []string{"EffectiveVersion", "HealthStatus", "LastModified", "SelectedInstanceType", "SystemTableStatuses"}
	for _, w := range ws {
		_, member, _ := strings.Cut(w.Subject, ".")
		want := WorkNewArgument
		if slices.Contains(notSettable, member) {
			want = WorkNewAttribute
		}
		if w.Kind != want {
			t.Errorf("%s = %s (%s), want %s", member, w.Kind, w.Basis, want)
		}
	}
	if got := ws[slices.IndexFunc(ws, func(w Work) bool { return w.Subject == "Config.SystemTableStatuses" })].Basis; got != `new member; documented as not settable ("The read-only")` {
		t.Errorf("basis = %q", got)
	}
}
