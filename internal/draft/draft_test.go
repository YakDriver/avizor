package draft

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/radar/internal/awssdk"
	"github.com/YakDriver/radar/internal/enrich"
	"github.com/YakDriver/radar/internal/reconcile"
	"github.com/YakDriver/radar/internal/whatsnew"
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
	want := "> Adds S3 access points. More detail here.\n\n> Second note.\n\nRequires AWS SDK for Go v2 [Release 2026-01-02](https://github.com/aws/aws-sdk-go-v2/commit/abc): https://github.com/o/r/pull/10.\n"
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
	if d.Title != "r/aws_widget_thing: Adds a size setting" {
		t.Errorf("title = %q", d.Title)
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
	} {
		if got := snake(in); got != want {
			t.Errorf("snake(%s) = %s, want %s", in, got, want)
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
	want := "> Amazon Widget now supports sizes\n\n* [Announcement](https://aws.amazon.com/wn/sizes/)\n\nRequires AWS SDK for Go v2 [Release 2026-01-02](https://github.com/aws/aws-sdk-go-v2/commit/abc): https://github.com/o/r/pull/10.\n"
	if d.Body != want {
		t.Errorf("body =\n%s\nwant\n%s", d.Body, want)
	}

	// Asking for an announcement when none matched warns and keeps the notes.
	if d, err = New(item("widget", "Adds sizes."), implemented("widget"), nil, o); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.Body, "> Adds sizes.") || !slices.ContainsFunc(d.Warnings, func(w string) bool { return strings.Contains(w, "no announcement matched") }) {
		t.Errorf("body =\n%s\nwarnings %q", d.Body, d.Warnings)
	}
}
