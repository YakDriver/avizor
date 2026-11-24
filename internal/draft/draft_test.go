package draft

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func item(module string, notes ...string) *awssdk.Item {
	it := &awssdk.Item{
		ID: "aws-sdk-go-v2/" + module + "/release-2026-01-02", Status: "new",
		Service: awssdk.Service{SDKID: "Widget", Module: module, ModulePath: "github.com/aws/aws-sdk-go-v2/service/" + module, ModuleVersion: "v1.2.0"},
		Release: awssdk.Release{Tag: "release-2026-01-02", URL: "https://github.com/aws/aws-sdk-go-v2/releases/tag/release-2026-01-02",
			CommitURL: "https://github.com/aws/aws-sdk-go-v2/commit/abc", PublishedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
	}
	for _, n := range notes {
		it.Notes = append(it.Notes, awssdk.Note{Kind: "Feature", Text: n})
	}
	return it
}

func implemented(pkg string, work ...enrich.Work) *enrich.Result {
	return &enrich.Result{
		Service:      enrich.ServiceMatch{Status: enrich.ServiceImplemented, Package: pkg, Label: "service/" + pkg, Basis: "test"},
		Availability: &enrich.Availability{Status: enrich.Available, Bump: &enrich.Bump{PR: 10}},
		Work:         work,
	}
}

var opts = Options{Repo: "o/r", Prefix: "aws_"}

const requires = "Requires `service/widget` v1.2.0 ([Release 2026-01-02](https://github.com/aws/aws-sdk-go-v2/commit/abc)), which the provider has."

func inferred(d *Draft, field string) []string {
	var out []string
	for _, in := range d.Inferred {
		if in.Field == field {
			out = append(out, in.Value)
		}
	}
	return out
}

func TestNewResource(t *testing.T) {
	t.Parallel()

	en := implemented("widget", enrich.Work{Kind: enrich.WorkNewResource, Subject: "S3AccessPoint"}, enrich.Work{Kind: enrich.WorkTagging, Subject: "tags"})
	d, err := New(item("widget", "Adds S3 access points. More detail here.", "Second note."), en, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "New resource: `aws_widget_s3_access_point`" {
		t.Errorf("title = %q", d.Title)
	}
	if !slices.Equal(d.Labels, []string{"enhancement", "service/widget", "new-resource"}) {
		t.Errorf("labels = %q", d.Labels)
	}
	want := "> Adds S3 access points. More detail here.\n\n> Second note.\n\n" + requires + "\n\n## Scope\n\nNew resource `aws_widget_s3_access_point`.\n"
	if d.Body != want {
		t.Errorf("body =\n%s\nwant\n%s", d.Body, want)
	}
	if got := inferred(d, "terraform_name"); !slices.Equal(got, []string{"aws_widget_s3_access_point"}) {
		t.Errorf("terraform names = %q", got)
	}
	wantArgs := []string{"issue", "create", "--repo", "o/r", "--title", d.Title, "--label", "enhancement", "--label", "service/widget", "--label", "new-resource", "--body-file", "-"}
	if !slices.Equal(d.GHArgs, wantArgs) {
		t.Errorf("gh args = %q", d.GHArgs)
	}
}

func TestNewArgument(t *testing.T) {
	t.Parallel()

	co := t.TempDir()
	src := filepath.Join(co, "internal", "service", "widget")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "widget.go"), []byte("package widget\n\n// @FrameworkResource(\"aws_widget_thing\", name=\"Thing\")\nfunc f() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	en := implemented("widget", enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateThingRequest.Size", UsedBy: []string{"internal/service/widget/widget.go"}})
	o := opts
	o.Checkout = co
	d, err := New(item("widget", "Adds a size setting."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "r/`aws_widget_thing`: Add `size`" || !strings.Contains(d.Body, "\n- New argument `size`") {
		t.Errorf("title = %q, body =\n%s", d.Title, d.Body)
	}

	// With a new resource, the title names it, so the line keeps the existing resource's name.
	en.Work = append(en.Work, enrich.Work{Kind: enrich.WorkNewResource, Subject: "Gadget"})
	if d, err = New(item("widget", "Adds a size setting."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	if d.Title != "New resource: `aws_widget_gadget`" || !strings.Contains(d.Body, "\n- `aws_widget_thing`: new argument `size`") {
		t.Errorf("with a new resource, title = %q, body =\n%s", d.Title, d.Body)
	}
}

func TestFallbackTitleAndWarnings(t *testing.T) {
	t.Parallel()

	en := implemented("widget")
	en.Availability = &enrich.Availability{Status: enrich.Pending, ModulePath: "github.com/aws/aws-sdk-go-v2/service/widget", ProviderVersion: "v1.1.0", ItemVersion: "v1.2.0"}
	rec := &reconcile.Result{Candidates: []reconcile.Candidate{
		{Category: reconcile.CategoryTracked, Kind: reconcile.KindIssue, Number: 5, Title: "Widget sizes"},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindIssue, Number: 6, Title: "Widget colors", URL: "u6",
			Signals: []reconcile.Match{{Type: "label", Value: "service/widget"}, {Type: "keyword", Value: "widget"}, {Type: "keyword", Value: "colors"}}},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindIssue, Number: 7, Title: "[Bug]: widget crash", Labels: []string{"bug"},
			Signals: []reconcile.Match{{Type: "label", Value: "service/widget"}, {Type: "keyword", Value: "widget"}, {Type: "keyword", Value: "crash"}}},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindIssue, Number: 8, Title: "Other",
			Signals: []reconcile.Match{{Type: "keyword", Value: "widget"}, {Type: "keyword", Value: "other"}}},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindPR, Number: 9, Title: "Bump the SDK",
			Signals: []reconcile.Match{{Type: reconcile.MatchBump, Value: "widget"}, {Type: "label", Value: "service/widget"}, {Type: "keyword", Value: "widget"}, {Type: "keyword", Value: "sdk"}}},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindPR, Number: 10, Title: "Widget sizes PR",
			Signals: []reconcile.Match{{Type: "label", Value: "service/widget"}, {Type: "keyword", Value: "widget"}, {Type: "keyword", Value: "sizes"}}},
	}}
	d, err := New(item("widget", "Adds sizes."), en, rec, opts)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Widget: Adds sizes" {
		t.Errorf("title = %q", d.Title)
	}
	if strings.Contains(d.Body, "/pull/") {
		t.Errorf("pending item's body has a bump PR:\n%s", d.Body)
	}
	if len(d.Warnings) != 2 || !strings.Contains(d.Warnings[0], "isn't available yet") || !strings.Contains(d.Warnings[1], "already tracked: #5") {
		t.Errorf("warnings = %q", d.Warnings)
	}
	if len(d.Relates) != 2 || d.Relates[0].Number != 6 || d.Relates[1].Number != 10 || d.Relates[1].Kind != reconcile.KindPR {
		t.Errorf("relates = %+v", d.Relates)
	}
	if strings.Contains(d.Body, "Relates") {
		t.Errorf("body has suggested relates:\n%s", d.Body)
	}
}

func TestNewService(t *testing.T) {
	t.Parallel()

	en := &enrich.Result{Service: enrich.ServiceMatch{Status: enrich.ServiceUnknown}, Work: []enrich.Work{{Kind: enrich.WorkNewResource, Subject: "Gadget"}}}
	o := opts
	o.LabelPrefix = "svc:"
	d, err := New(item("gizmo", "New AWS service client module", "Initial release of Gizmo."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.Labels, []string{"enhancement", "new-service", "svc:gizmo", "new-resource"}) {
		t.Errorf("labels = %q", d.Labels)
	}
	if !strings.HasPrefix(d.Body, "> Initial release of Gizmo.\n") || strings.Contains(d.Body, NewModuleNote) {
		t.Errorf("body =\n%s", d.Body)
	}
	if d.Title != "New resource: `aws_gizmo_gadget`" {
		t.Errorf("title = %q", d.Title)
	}
}

func TestTemplate(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "t.md")
	if err := os.WriteFile(p, []byte("{{.Release}} for {{.Service.Package}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := opts
	o.Template = p
	d, err := New(item("widget"), implemented("widget"), nil, o)
	if err != nil {
		t.Fatal(err)
	}
	if d.Body != "Release 2026-01-02 for widget\n" {
		t.Errorf("body = %q", d.Body)
	}

	if err := os.WriteFile(p, []byte("{{.Nope}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(item("widget"), implemented("widget"), nil, o); err == nil {
		t.Error("template with an unknown field didn't fail")
	}
}

func TestSnake(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"Channel": "channel", "S3AccessPoint": "s3_access_point", "KMSKey": "kms_key", "KeyLastUsage": "key_last_usage", "VPCEndpoint": "vpc_endpoint",
		"SaslOAuthBearer": "sasl_oauth_bearer", "OAuth2Config": "oauth2_config", "IPv6Address": "ipv6_address", "Ipv4Pool": "ipv4_pool",
		"WarmThroughputMiBps": "warm_throughput_mib_ps", "MemoryGiBPerVCpu": "memory_gib_per_v_cpu",
	} {
		if got := enrich.Snake(in); got != want {
			t.Errorf("Snake(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestAnnouncement(t *testing.T) {
	t.Parallel()

	en := implemented("widget")
	en.Announcement = &whatsnew.Match{Announcement: whatsnew.Announcement{Headline: "Amazon Widget now supports sizes", URL: "https://aws.amazon.com/wn/sizes/"},
		Score: 0.7, Basis: "text similarity 0.70"}
	d, err := New(item("widget", "Adds sizes. More."), en, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	// The match titles the draft but stays out of the body unless accepted.
	if d.Title != "Amazon Widget now supports sizes" || !strings.HasPrefix(d.Body, "> Adds sizes. More.\n") {
		t.Errorf("title = %q, body =\n%s", d.Title, d.Body)
	}
	if got := inferred(d, "announcement"); len(got) != 1 || !strings.Contains(got[0], "https://aws.amazon.com/wn/sizes/") {
		t.Errorf("inferred announcement = %q", got)
	}

	o := opts
	o.Announcement = true
	if d, err = New(item("widget", "Adds sizes. More."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	want := "> Adds sizes. More.\n\n- [Amazon Widget now supports sizes](https://aws.amazon.com/wn/sizes/)\n- " + requires + "\n"
	if d.Body != want {
		t.Errorf("body =\n%s\nwant\n%s", d.Body, want)
	}

	// Blog posts are listed after the announcement, in order and without repeats. A post that isn't stored is titled
	// with its URL, so it's linked as "Blog post". An inferred post is reported with its basis.
	o.BlogPosts = []BlogPost{
		{Post: whatsnew.Post{Title: "Widget sizes deep dive", URL: "https://aws.amazon.com/blogs/x/sizes/"}},
		{Post: whatsnew.Post{Title: "https://aws.amazon.com/blogs/x/other/", URL: "https://aws.amazon.com/blogs/x/other/"}},
		{Post: whatsnew.Post{Title: "Widget sizes deep dive", URL: "https://aws.amazon.com/blogs/x/sizes/"}, Basis: "matches"},
		{Post: whatsnew.Post{Title: "Sizing widgets", URL: "https://aws.amazon.com/blogs/x/sizing/"}, Basis: "matches the announcement"},
	}
	if d, err = New(item("widget", "Adds sizes. More."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	want = "> Adds sizes. More.\n\n- [Amazon Widget now supports sizes](https://aws.amazon.com/wn/sizes/)\n" +
		"- [Widget sizes deep dive](https://aws.amazon.com/blogs/x/sizes/)\n- [Blog post](https://aws.amazon.com/blogs/x/other/)\n" +
		"- [Sizing widgets](https://aws.amazon.com/blogs/x/sizing/)\n- " + requires + "\n"
	if d.Body != want {
		t.Errorf("body with blog posts =\n%s\nwant\n%s", d.Body, want)
	}
	if got := inferred(d, "blog_post"); !slices.Equal(got, []string{"Sizing widgets https://aws.amazon.com/blogs/x/sizing/"}) {
		t.Errorf("inferred blog posts = %q", got)
	}

	// Without the announcement, the blog posts start the list.
	o.Announcement = false
	o.BlogPosts = o.BlogPosts[:1]
	if d, err = New(item("widget", "Adds sizes. More."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	want = "> Adds sizes. More.\n\n- [Widget sizes deep dive](https://aws.amazon.com/blogs/x/sizes/)\n- " + requires + "\n"
	if d.Body != want {
		t.Errorf("body with only a blog post =\n%s\nwant\n%s", d.Body, want)
	}
	o.Announcement, o.BlogPosts = true, nil

	// Asking for an announcement when none matched warns, even with no release notes.
	if d, err = New(item("widget", NewModuleNote), implemented("widget"), nil, o); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(d.Warnings, func(w string) bool { return strings.Contains(w, "no announcement matched") }) {
		t.Errorf("warnings without notes = %q", d.Warnings)
	}

	// Asking for an announcement when none matched warns and keeps the notes.
	if d, err = New(item("widget", "Adds sizes."), implemented("widget"), nil, o); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.Body, "> Adds sizes.") || !slices.ContainsFunc(d.Warnings, func(w string) bool { return strings.Contains(w, "no announcement matched") }) {
		t.Errorf("body =\n%s\nwarnings %q", d.Body, d.Warnings)
	}
}

// Several new resources are named in the title.
func TestNewResources(t *testing.T) {
	t.Parallel()

	en := implemented("widget", enrich.Work{Kind: enrich.WorkNewResource, Subject: "Router"}, enrich.Work{Kind: enrich.WorkNewResource, Subject: "Policy"},
		enrich.Work{Kind: enrich.WorkNewResource, Subject: "Route"}, enrich.Work{Kind: enrich.WorkNewResource, Subject: "Rule"})
	d, err := New(item("widget", "Adds routers."), en, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if want := "New resources: `aws_widget_router`, `aws_widget_policy`, and 2 more"; d.Title != want {
		t.Errorf("title = %q, want %q", d.Title, want)
	}
	en.Work = en.Work[:2]
	if d, err = New(item("widget", "Adds routers."), en, nil, opts); err != nil {
		t.Fatal(err)
	}
	if want := "New resources: `aws_widget_router` and `aws_widget_policy`"; d.Title != want {
		t.Errorf("title = %q, want %q", d.Title, want)
	}
}

// Boilerplate notes the SDK adds to many services are left out of the quote and the title, unless every note is one.
func TestBoilerplateNotes(t *testing.T) {
	t.Parallel()

	const schema = "Enable schema-based (de)serialization for this service."
	d, err := New(item("widget", schema, "Adds hierarchy filters to dashboards."), implemented("widget"), nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Widget: Adds hierarchy filters to dashboards" || strings.Contains(d.Body, schema) {
		t.Errorf("title = %q, body =\n%s", d.Title, d.Body)
	}
	if d, err = New(item("widget", schema), implemented("widget"), nil, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.Body, "> "+schema+"\n") {
		t.Errorf("body with only boilerplate =\n%s", d.Body)
	}
}

// A data source whose operations only list is left out, and doesn't title the draft.
func TestListOnlyDataSource(t *testing.T) {
	t.Parallel()

	en := implemented("widget", enrich.Work{Kind: enrich.WorkNewDataSource, Subject: "Finding", Operations: []string{"ListFindings"}})
	d, err := New(item("widget", "Adds findings."), en, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Widget: Adds findings" || strings.Contains(d.Body, "aws_widget_finding") {
		t.Errorf("title = %q, body =\n%s", d.Title, d.Body)
	}
	if got := inferred(d, "left_out"); !slices.Equal(got, []string{"Finding"}) {
		t.Errorf("left out = %q", got)
	}
}

// TestTitleTwoResources checks that a change to two resources names both, unless an announcement's headline is the
// title, and that a data source among them falls back.
func TestTitleTwoResources(t *testing.T) {
	t.Parallel()

	infer := func(string, string, string) {}
	en := implemented("widget")
	two := []string{"aws_widget_stack", "aws_widget_builder"}
	if got, _ := titleAndLabels(item("widget", "Adds agents."), en, "", opts, two, []string{"agent_access"}, infer); got != "r/`aws_widget_stack`, r/`aws_widget_builder`: Add `agent_access`" {
		t.Errorf("title = %q", got)
	}
	if got, _ := titleAndLabels(item("widget", "Adds agents."), en, "Adds agents.", opts, []string{"aws_widget_stack", "aws_widget_stack" + dataSourceMark}, []string{"agent_access"}, infer); got != "Widget: Adds agents" {
		t.Errorf("with a data source: title = %q", got)
	}
	withAnn := implemented("widget")
	withAnn.Announcement = &whatsnew.Match{}
	if got, _ := titleAndLabels(item("widget", "Adds agents."), withAnn, "Widget now supports agents.", opts, two, []string{"agent_access"}, infer); got != "Widget now supports agents" {
		t.Errorf("with an announcement: title = %q", got)
	}
}
