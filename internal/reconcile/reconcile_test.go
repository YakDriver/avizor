package reconcile

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/config"
	"github.com/YakDriver/avizor/internal/index"
	"github.com/YakDriver/avizor/internal/services"
	"github.com/YakDriver/avizor/internal/signal"
)

const repo = "hashicorp/terraform-provider-aws"

func seed(t *testing.T) *index.Store {
	t.Helper()
	ctx := context.Background()
	st, err := index.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	issues := []*index.Issue{
		{Number: 45362, Kind: index.KindIssue, State: "open", Labels: []string{"enhancement", "service/fsx", "service/s3control"},
			Title: "r/aws_fsx_s3_access_point_attachment: Attach to Amazon FSx for NetApp ONTAP file systems",
			Body:  "* [Announcement](https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/)\nRequires https://github.com/aws/aws-sdk-go-v2/commit/abc123. Similar to aws_s3_access_point.\nRelates #43195. Bumped in #45358."},
		{Number: 45358, Kind: index.KindPR, State: "closed", Merged: true, Labels: []string{"dependencies"},
			Title: "aws-sdk-go-v2 updates", Body: "https://github.com/aws/aws-sdk-go-v2/commit/abc123"},
		{Number: 45400, Kind: index.KindPR, State: "closed", Merged: true, Labels: []string{"service/fsx"},
			Title: "New resource: aws_fsx_s3_access_point_attachment",
			Body:  "Closes #45362. Uses CreateAndAttachS3AccessPoint."},
		{Number: 43195, Kind: index.KindIssue, State: "open", Labels: []string{"service/s3control"},
			Title: "S3 access point support for more sources", Body: "Access points everywhere."},
		{Number: 40000, Kind: index.KindIssue, State: "closed", StateReason: "not_planned", Labels: []string{"service/fsx"},
			Title: "Support S3 access points for FSx ONTAP volumes", Body: "Call DetachAndDeleteS3AccessPoint too."},
		{Number: 30000, Kind: index.KindIssue, State: "open", Labels: []string{"service/ec2"},
			Title: "aws_instance crashes", Body: "Unrelated, mentions aws_fsx_s3_access_point_attachment in a log."},
		{Number: 45361, Kind: index.KindIssue, State: "open", Labels: []string{"service/s3tables"},
			Title: "S3 Tables Intelligent-Tiering storage class",
			Body:  "Requires https://github.com/aws/aws-sdk-go-v2/commit/abc123. See aws_s3_access_point for prior art."},
		{Number: 45084, Kind: index.KindPR, State: "closed", Merged: true, Labels: []string{"service/s3control"},
			Title: "r/aws_s3_access_point: Test tagging", Body: ""},
		{Number: 30001, Kind: index.KindIssue, State: "open", Labels: []string{"service/lambda"},
			Title: "Lambda timeout", Body: "nothing here"},
	}
	tx, err := st.Begin(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range issues {
		is.CreatedAt, is.UpdatedAt, is.URL = ts, ts, "https://github.com/"+repo+"/issues/"
		if err := tx.UpsertIssue(ctx, is, 1); err != nil {
			t.Fatal(err)
		}
	}
	// A comment on #43195 pastes the announcement.
	if err := tx.ReplaceCommentSignals(ctx, 43195, 9, ts.Add(time.Hour), "AWS announced https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMeta(ctx, "last_sync", ts.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	return st
}

func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"internal/service/fsx/s3_access_point_attachment.go":          "package fsx\n// @FrameworkResource(\"aws_fsx_s3_access_point_attachment\", name=\"S3 Access Point Attachment\")\nfunc f() { conn.CreateAndAttachS3AccessPoint(ctx, input) }\n",
		"internal/service/fsx/s3_access_point_attachment_test.go":     "package fsx\n// CreateAndAttachS3AccessPoint in a test\n",
		"website/docs/r/fsx_s3_access_point_attachment.html.markdown": "# aws_fsx_s3_access_point_attachment\n",
	}
	for p, body := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func reconciler(t *testing.T, st *index.Store, co string) *Reconciler {
	t.Helper()
	return &Reconciler{
		Store: st,
		Services: services.New(
			&services.Service{Package: "fsx", Names: []string{"FSx"}, ResourcePrefixes: []string{"aws_fsx_"}},
			&services.Service{Package: "s3control", Names: []string{"S3 Control"}, ResourcePrefixes: []string{"aws_s3control_"}},
		),
		Target:  &config.Target{Name: "tfaws", Repo: repo, Checkout: co, ServiceLabelPrefix: "service/"},
		Options: Options{NoFetch: true},
	}
}

type got struct {
	category, kind string
	number         int
	path           string
}

func summarizeCandidates(cs []Candidate) []got {
	var out []got
	for _, c := range cs {
		out = append(out, got{c.Category, c.Kind, c.Number, c.Path})
	}
	return out
}

func hasSignal(c Candidate, typ, value string) bool {
	return slices.ContainsFunc(c.Signals, func(m Match) bool { return m.Type == typ && m.Value == value })
}

func find(t *testing.T, cs []Candidate, number int, path string) Candidate {
	t.Helper()
	for _, c := range cs {
		if c.Number == number && c.Path == path {
			return c
		}
	}
	t.Fatalf("candidate #%d %s not found in %v", number, path, summarizeCandidates(cs))
	return Candidate{}
}

func TestRunURL(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), "")
	res, err := r.Run(context.Background(), "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/?trk=x", "")
	if err != nil {
		t.Fatal(err)
	}
	c := find(t, res.Candidates, 45362, "")
	if c.Category != CategoryTracked || !hasSignal(c, "url", "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access") {
		t.Errorf("#45362 = %+v", c)
	}
	c = find(t, res.Candidates, 43195, "")
	if c.Category != CategoryTracked || c.Signals[0].Where != index.WhereComment {
		t.Errorf("#43195 should match through its comment: %+v", c)
	}
	if res.Summary.Unmatched {
		t.Error("summary says unmatched")
	}
}

func TestRunIssue(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), checkout(t))
	res, err := r.Run(context.Background(), "45362", "")
	if err != nil {
		t.Fatal(err)
	}

	// Merged PR naming the resource in its title, and referencing the input.
	pr := find(t, res.Candidates, 45400, "")
	if pr.Category != CategoryImplemented || !hasSignal(pr, "tfname", "aws_fsx_s3_access_point_attachment") || !hasSignal(pr, "references_input", "#45362") {
		t.Errorf("#45400 = %+v", pr)
	}

	// Code and docs found from the title's Terraform name. Test files are excluded from op matches.
	find(t, res.Candidates, 0, "website/docs/r/fsx_s3_access_point_attachment.html.markdown")
	code := find(t, res.Candidates, 0, "internal/service/fsx/s3_access_point_attachment.go")
	if code.Category != CategoryImplemented {
		t.Errorf("code candidate = %+v", code)
	}
	for _, c := range res.Candidates {
		if c.Path == "internal/service/fsx/s3_access_point_attachment_test.go" && hasSignal(c, "op", "CreateAndAttachS3AccessPoint") {
			t.Error("op matched in a test file")
		}
	}

	// A body-only Terraform name mention is weak, so the unrelated bug is only "related".
	if c := find(t, res.Candidates, 30000, ""); c.Category != CategoryRelated {
		t.Errorf("#30000 category = %s, want related", c.Category)
	}

	if c := find(t, res.Candidates, 45358, ""); c.Category != CategoryRelated || !c.crossReferenced() {
		t.Errorf("#45358 = %+v, want related cross-reference", c)
	}

	// Sharing an SDK release commit or a body-only Terraform name doesn't make a candidate tracked or implemented.
	for _, n := range []int{45361, 45084} {
		for _, c := range res.Candidates {
			if c.Number == n && c.Category != CategoryRelated {
				t.Errorf("#%d category = %s, want related or absent", n, c.Category)
			}
		}
	}

	// The cross-referenced issue appears; the input itself does not.
	find(t, res.Candidates, 43195, "")
	for _, c := range res.Candidates {
		if c.Number == 45362 {
			t.Error("input issue returned as its own candidate")
		}
		if c.Number == 30001 {
			t.Error("unrelated issue returned")
		}
	}

	// Services inferred from labels, and the request kind, each with a basis.
	if len(res.Inferences) < 3 || res.Inferences[0].Value != "fsx" || res.Inferences[0].Basis != "label service/fsx" {
		t.Errorf("inferences = %+v", res.Inferences)
	}
	if inf := res.Inferences[len(res.Inferences)-1]; inf.Type != "request" || inf.Value != RequestNewType {
		t.Errorf("request inference = %+v", inf)
	}

	// Within a category, direct cross-references sort ahead of other matches with the same number of strong signals.
	for i := 1; i < len(res.Candidates); i++ {
		a, b := res.Candidates[i-1], res.Candidates[i]
		if a.Category == b.Category && a.strongCount() == b.strongCount() && !a.crossReferenced() && b.crossReferenced() {
			t.Errorf("cross-referenced #%d sorted after #%d", b.Number, a.Number)
		}
	}

	// Implemented candidates sort first.
	if res.Candidates[0].Category != CategoryImplemented {
		t.Errorf("first candidate = %+v", res.Candidates[0])
	}
}

func TestRunText(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), "")
	res, err := r.Run(context.Background(), "", "FSx S3 access point support")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Input.Keywords, []string{"fsx", "s3", "access", "point"}) {
		t.Errorf("keywords = %v", res.Input.Keywords)
	}
	// Keyword-only matches are "related", including the closed-not-planned issue that is worded differently.
	for _, n := range []int{45362, 40000, 43195} {
		if c := find(t, res.Candidates, n, ""); c.Category != CategoryRelated {
			t.Errorf("#%d category = %s, want related", n, c.Category)
		}
	}
	if !res.Summary.Unmatched {
		t.Error("keyword-only results should be summarized as unmatched")
	}
}

func TestRunOpInText(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), "")
	res, err := r.Run(context.Background(), "", "Does the provider support DetachAndDeleteS3AccessPoint?")
	if err != nil {
		t.Fatal(err)
	}
	if c := find(t, res.Candidates, 40000, ""); c.Category != CategoryConsidered {
		t.Errorf("#40000 category = %s, want previously_considered", c.Category)
	}
}

type opSet map[string]bool

func (o opSet) Has(name string) bool { return o[name] }

// TestRunOperationsFilter checks that with a set of real operation names, an operation-like word that isn't one doesn't
// count as an operation signal.
func TestRunOperationsFilter(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), "")
	r.Operations = opSet{"CreateAndAttachS3AccessPoint": true}
	res, err := r.Run(context.Background(), "", "Does the provider support DetachAndDeleteS3AccessPoint?")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(res.Evidence, func(e Evidence) bool { return e.Type == "op" }) {
		t.Errorf("evidence has an op signal for a name not in the operation set: %+v", res.Evidence)
	}
	for _, c := range res.Candidates {
		if c.Number == 40000 && c.Category != CategoryRelated {
			t.Errorf("#40000 category = %s, want related without the op signal", c.Category)
		}
	}
	if !slices.ContainsFunc(res.Warnings, func(w string) bool { return strings.Contains(w, "DetachAndDeleteS3AccessPoint") }) {
		t.Errorf("warnings = %q, want the ignored name", res.Warnings)
	}

	// A real operation still counts.
	r.Operations = opSet{"DetachAndDeleteS3AccessPoint": true}
	res, err = r.Run(context.Background(), "", "Does the provider support DetachAndDeleteS3AccessPoint?")
	if err != nil {
		t.Fatal(err)
	}
	if c := find(t, res.Candidates, 40000, ""); c.Category != CategoryConsidered {
		t.Errorf("#40000 category = %s, want previously_considered", c.Category)
	}
}

func TestRunChangeRequest(t *testing.T) {
	t.Parallel()

	// A request to change an existing type: its code and same-titled PRs are context, not implementation.
	r := reconciler(t, seed(t), checkout(t))
	res, err := r.Run(context.Background(), "", "aws_fsx_s3_access_point_attachment: add tags argument")
	if err != nil {
		t.Fatal(err)
	}
	if inf := res.Inferences[len(res.Inferences)-1]; inf.Value != RequestChange {
		t.Errorf("request inference = %+v", inf)
	}
	for _, c := range res.Candidates {
		if c.Category != CategoryRelated {
			t.Errorf("candidate %d %s category = %s, want related", c.Number, c.Path, c.Category)
		}
	}
	find(t, res.Candidates, 0, "internal/service/fsx/s3_access_point_attachment.go")
	find(t, res.Candidates, 45400, "")
}

func TestInferRequest(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]string{
		"r/aws_fsx_s3_access_point_attachment: Attach to Amazon FSx for NetApp ONTAP file systems": RequestNewType,
		"New resource: aws_fsx_s3_access_point_attachment":                                         RequestNewType,
		"[New Resource]: aws_s3tables_table_bucket_logging":                                        RequestNewType,
		"[Enhancement]: aws_imagebuilder_infrastructure_configuration: support parent_instance":    RequestChange,
		"r/aws_instance: Add cpu_options argument":                                                 RequestChange,
		"FSx S3 access point": RequestUnknown,
	} {
		if got := inferRequest(text, &Result{}); got != want {
			t.Errorf("inferRequest(%q) = %s, want %s", text, got, want)
		}
	}
}

func TestResolveErrors(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), "")
	ctx := context.Background()
	for _, tc := range []struct{ arg, text string }{
		{"", ""},
		{"45362", "also text"},
		{"not-a-thing", ""},
		{"99999", ""}, // not indexed, no fetcher
	} {
		if _, err := r.Run(ctx, tc.arg, tc.text); err == nil {
			t.Errorf("Run(%q, %q): expected error", tc.arg, tc.text)
		}
	}
}

func TestParsePage(t *testing.T) {
	t.Parallel()

	title, desc, err := parsePage(`<html><head><title>Ignored</title>
<meta property="og:title" content="Amazon FSx for NetApp ONTAP now supports Amazon S3 access - AWS" />
<meta name="description" content="Posted On: Dec 2, 2025 &amp; more" /></head></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Amazon FSx for NetApp ONTAP now supports Amazon S3 access" || desc != "Posted On: Dec 2, 2025 & more" {
		t.Errorf("parsePage = %q, %q", title, desc)
	}
	if _, _, err := parsePage("<html></html>"); err == nil {
		t.Error("expected error for page without a title")
	}
}

func TestIssueNumber(t *testing.T) {
	t.Parallel()

	for arg, want := range map[string]int{
		"45362":  45362,
		"#45362": 45362,
		"https://github.com/hashicorp/terraform-provider-aws/issues/45362":                      45362,
		"https://github.com/hashicorp/terraform-provider-aws/pull/45358/files":                  45358,
		"https://github.com/hashicorp/terraform-provider-aws/issues/45362#issuecomment-1234567": 45362,
		"https://github.com/Hashicorp/Terraform-Provider-AWS/issues/45362":                      45362,
		"https://github.com/other/repo/issues/45362":                                            0,
		"https://github.com/hashicorp/terraform-provider-aws/issues/45362abc":                   0,
		"https://aws.amazon.com/about-aws/whats-new/2025/12/x/":                                 0,
		"0":   0,
		"FSx": 0,
	} {
		got, ok := IssueNumber(arg, repo)
		if got != want || ok != (want != 0) {
			t.Errorf("IssueNumber(%q) = %d, %t; want %d", arg, got, ok, want)
		}
	}
}

// TestRunAsOfHidesLaterReferences checks that issues filed after the cutoff never appear, even when the input's current
// body references them.
func TestRunAsOfHidesLaterReferences(t *testing.T) {
	t.Parallel()

	st := seed(t)
	ctx := context.Background()
	tx, err := st.Begin(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := tx.UpsertIssue(ctx, &index.Issue{Number: 50000, Kind: index.KindPR, State: "open", Title: "Later work",
		Body: "", CreatedAt: later, UpdatedAt: later}, 1); err != nil {
		t.Fatal(err)
	}
	// The input's body was edited to mention the later PR.
	if err := tx.UpsertIssue(ctx, &index.Issue{Number: 50001, Kind: index.KindIssue, State: "open", Title: "Request",
		Body: "Superseded by #50000. Also see #12345.", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: later}, 1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	r := reconciler(t, st, "")
	r.AsOf = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	res, err := r.Run(ctx, "50001", "")
	if err != nil {
		t.Fatal(err)
	}
	var sawUnindexed bool
	for _, c := range res.Candidates {
		if c.Number == 50000 {
			t.Errorf("#50000 filed after the cutoff returned as %+v", c)
		}
		if c.Number == 12345 {
			sawUnindexed = true
		}
	}
	if !sawUnindexed {
		t.Error("#12345, genuinely not indexed, should still appear as a cross-reference")
	}
}

// TestRunFollowsReferences checks that a PR referencing a strongly matched issue is returned with the issue's match, and
// that weakly matched issues and comment references aren't followed.
func TestRunFollowsReferences(t *testing.T) {
	t.Parallel()

	st := seed(t)
	ctx := context.Background()
	ts := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	tx, err := st.Begin(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range []*index.Issue{
		// Implements #45362 without repeating its announcement URL.
		{Number: 46000, Kind: index.KindPR, State: "closed", Merged: true, MergedAt: ts, Title: "r/aws_fsx_s3_access_point_attachment: new resource",
			Body: "Closes #45362."},
		// Open PR for #45362.
		{Number: 46001, Kind: index.KindPR, State: "open", Title: "Docs tweak", Body: "Relates #45362."},
		// An issue, not a PR, that references #45362.
		{Number: 46002, Kind: index.KindIssue, State: "open", Title: "Question", Body: "Is #45362 done?"},
		// A PR that references #30001, which matches only weakly.
		{Number: 46003, Kind: index.KindPR, State: "closed", Merged: true, MergedAt: ts, Title: "Lambda fix", Body: "Fixes #30001."},
		// A PR that mentions #45362 only in a comment.
		{Number: 46004, Kind: index.KindPR, State: "open", Title: "Unrelated", Body: "nothing"},
	} {
		is.CreatedAt, is.UpdatedAt = ts, ts
		if err := tx.UpsertIssue(ctx, is, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.ReplaceCommentSignals(ctx, 46004, 77, ts, "see #45362"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	r := reconciler(t, st, "")
	res, err := r.Run(ctx, "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/", "")
	if err != nil {
		t.Fatal(err)
	}
	if c := find(t, res.Candidates, 45362, ""); c.Category != CategoryTracked {
		t.Fatalf("#45362 = %+v", c)
	}
	if c := find(t, res.Candidates, 46000, ""); c.Category != CategoryImplemented || !hasSignal(c, matchReferencesCandidate, "#45362") {
		t.Errorf("#46000 = %+v, want implemented via #45362", c)
	}
	if c := find(t, res.Candidates, 46001, ""); c.Category != CategoryTracked {
		t.Errorf("#46001 category = %s, want already_tracked", c.Category)
	}
	for _, c := range res.Candidates {
		switch c.Number {
		case 46002, 46003, 46004:
			if hasSignal(c, matchReferencesCandidate, "#45362") || c.Category != CategoryRelated {
				t.Errorf("#%d should not be followed: %+v", c.Number, c)
			}
		}
	}

	// As of before the PRs existed, they aren't followed.
	r.AsOf = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	res, err = r.Run(ctx, "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Candidates {
		if c.Number == 46000 || c.Number == 46001 {
			t.Errorf("#%d returned before it was created", c.Number)
		}
	}
}

func TestRunItem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := seed(t)

	// The release commit, cited by #45362 (fsx), #45361 (s3tables), and the bump PR #45358 (no service label).
	r := reconciler(t, st, "")
	res, err := r.RunItem(ctx, &Item{
		ID: "sdk/fsx/release-1", Title: "FSx: S3 Access Points support for FSx for NetApp ONTAP", Text: "S3 Access Points support for FSx for NetApp ONTAP",
		Package: "fsx", Operations: []string{"CreateAndAttachS3AccessPoint"},
		ReleaseCommitURL: "https://github.com/aws/aws-sdk-go-v2/commit/abc123",
		Request:          RequestChange, RequestBasis: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Input.Kind != InputItem || res.Input.Ref != "sdk/fsx/release-1" {
		t.Errorf("input = %+v", res.Input)
	}
	if c := find(t, res.Candidates, 45362, ""); c.Category != CategoryTracked || !hasSignal(c, "url", "https://github.com/aws/aws-sdk-go-v2/commit/abc123") {
		t.Errorf("#45362 = %s %+v, want already_tracked by the release commit", c.Category, c.Signals)
	}
	for _, c := range res.Candidates {
		switch c.Number {
		case 45361, 45358, 45400:
			// Another service's tracker, the bump PR, and a PR matched only by an existing operation.
			if c.Category != CategoryRelated && !(c.Number == 45400 && hasSignal(c, matchReferencesCandidate, "#45362")) {
				t.Errorf("#%d category = %s, want related: %+v", c.Number, c.Category, c.Signals)
			}
		}
	}
	if i := slices.IndexFunc(res.Inferences, func(in Inference) bool { return in.Type == "request" }); i < 0 || res.Inferences[i].Value != RequestChange {
		t.Errorf("inferences = %+v", res.Inferences)
	}

	// The bump PR is only related, even with the label and release commit.
	r2 := reconciler(t, st, "")
	bump := &Item{ID: "sdk/fsx/release-1", Title: "FSx", Package: "fsx", ReleaseCommitURL: "https://github.com/aws/aws-sdk-go-v2/commit/abc123", BumpPR: 45400, NewOperations: []string{"CreateAndAttachS3AccessPoint"}, Request: RequestNewType}
	res, err = r2.RunItem(ctx, bump)
	if err != nil {
		t.Fatal(err)
	}
	if c := find(t, res.Candidates, 45400, ""); c.Category != CategoryRelated || !hasSignal(c, MatchBump, "brings in the SDK version; doesn't implement the change") {
		t.Errorf("bump PR #45400 = %s %+v, want related", c.Category, c.Signals)
	}

	// A new operation is strong only with the item's service label.
	item := &Item{ID: "sdk/x/release-1", Title: "X", NewOperations: []string{"DetachAndDeleteS3AccessPoint"}, Request: RequestNewType}
	for pkg, want := range map[string]string{"fsx": CategoryConsidered, "s3control": CategoryRelated, "": CategoryRelated} {
		item.Package = pkg
		res, err := r.RunItem(ctx, item)
		if err != nil {
			t.Fatal(err)
		}
		if c := find(t, res.Candidates, 40000, ""); c.Category != want {
			t.Errorf("package %q: #40000 category = %s, want %s", pkg, c.Category, want)
		}
	}
}

func TestRunItemCode(t *testing.T) {
	t.Parallel()
	r := reconciler(t, seed(t), checkout(t))
	item := &Item{ID: "sdk/fsx/release-1", Title: "FSx", Package: "fsx", NewOperations: []string{"CreateAndAttachS3AccessPoint"}, Request: RequestNewType}
	res, err := r.RunItem(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	if c := find(t, res.Candidates, 0, "internal/service/fsx/s3_access_point_attachment.go"); c.Category != CategoryImplemented {
		t.Errorf("code category = %s", c.Category)
	}
	if !res.CodeSearched {
		t.Error("code searched = false")
	}
	// Without a package, code isn't searched: other services may have an operation of the same name.
	item.Package = ""
	if res, err = r.RunItem(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(res.Candidates, func(c Candidate) bool { return c.Path != "" }) || res.CodeSearched {
		t.Errorf("code searched without a package (%v): %v", res.CodeSearched, summarizeCandidates(res.Candidates))
	}
}

// TestRunItemCodeMissingPackage checks that an item's code search doesn't fall back to other services when its package
// isn't in the checkout.
func TestRunItemCodeMissingPackage(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), checkout(t))
	item := &Item{ID: "sdk/s3control/release-1", Title: "S3 Control", Package: "s3control", NewOperations: []string{"CreateAndAttachS3AccessPoint"}, Request: RequestNewType}
	res, err := r.RunItem(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(res.Candidates, func(c Candidate) bool { return c.Path != "" }) || res.CodeSearched {
		t.Errorf("code from another package (searched %v): %v", res.CodeSearched, summarizeCandidates(res.Candidates))
	}
	if !slices.ContainsFunc(res.Warnings, func(w string) bool { return strings.Contains(w, "internal/service/s3control") }) {
		t.Errorf("warnings = %q", res.Warnings)
	}
}

func TestDemoteExisting(t *testing.T) {
	t.Parallel()

	op := func(v string) Match { return Match{Type: signal.KindOp, Value: v, Strength: StrengthStrong} }
	cands := []Candidate{
		{Kind: KindCode, Path: "internal/service/kafka/topic.go", Category: CategoryImplemented, Signals: []Match{op("CreateTopic")}},
		// A bug report on the existing resource, matched only by an operation its code calls.
		{Kind: KindIssue, Number: 1, Title: "r/aws_msk_topic: Reducing the partition count should force a new resource", Category: CategoryTracked, Signals: []Match{op("CreateTopic")}},
		// A PR that references it.
		{Kind: KindPR, Number: 2, Title: "r/aws_msk_topic: Force new when partitions shrink", Category: CategoryTracked, Signals: []Match{{Type: matchReferencesCandidate, Value: "#1", Strength: StrengthStrong}}},
		// The PR that added the resource keeps its category.
		{Kind: KindPR, Number: 3, Title: "Added msk_aws_topic resource and data source", Category: CategoryConsidered, Signals: []Match{op("CreateTopic")}},
		{Kind: KindIssue, Number: 4, Title: "[New Service]: Amazon MSK topics", Category: CategoryConsidered, Signals: []Match{op("CreateTopic")}},
		// Another strong signal, such as an operation the code doesn't call, keeps it.
		{Kind: KindIssue, Number: 5, Title: "Support topic configs", Category: CategoryTracked, Signals: []Match{op("CreateTopic"), op("UpdateTopicConfig")}},
	}
	demoteExisting(cands, map[string]bool{"CreateTopic": true})

	want := map[int]string{1: CategoryRelated, 2: CategoryRelated, 3: CategoryConsidered, 4: CategoryConsidered, 5: CategoryTracked}
	for _, c := range cands[1:] {
		if c.Category != want[c.Number] {
			t.Errorf("#%d category = %s, want %s", c.Number, c.Category, want[c.Number])
		}
		if c.Category == CategoryRelated && !slices.ContainsFunc(c.Signals, func(m Match) bool { return m.Type == MatchExistingCode }) {
			t.Errorf("#%d has no %s signal", c.Number, MatchExistingCode)
		}
	}
	if cands[0].Category != CategoryImplemented {
		t.Errorf("code category = %s", cands[0].Category)
	}
}

func TestAddsType(t *testing.T) {
	t.Parallel()

	for title, want := range map[string]bool{
		"New Resource: `aws_cloudwatch_alarm_mute_rule`":                            true,
		"feat(odb): add Autonomous Database resource and data source":               true,
		"feat: add aws_observabilityadmin_s3_table_integration and more":            true,
		"[New Service]: AWS Network Security Manager":                               true,
		"r/aws_ec2_secondary_network: add list resource support":                    false,
		"r/aws_msk_topic: Reducing the partition count should force a new resource": false,
		"[Enhancement]: aws_msk_topic: Add tags argument":                           false,
		"Fix(bedrockagentcore): handle computed harness environment variables":      false,
		"r/aws_bedrockdataautomation_library: New resource and list resource":       true,
		"Added msk_aws_topic resource and data source":                              true,
		"Add odb_network_arn to aws_route resource and data source":                 false,
		"add aws_msk_topic tags support":                                            false,
		"add resource policy support":                                               false,
		"New list resource: aws_ec2_secondary_network":                              false,
		"feat: add aws_ec2_secondary_network list resource support":                 false,
	} {
		if got := addsType(title); got != want {
			t.Errorf("addsType(%q) = %t, want %t", title, got, want)
		}
	}
}
