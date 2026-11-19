package awssdk

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/config"
	"github.com/YakDriver/avizor/internal/github"
	"github.com/YakDriver/avizor/internal/items"
)

func TestParseNotes(t *testing.T) {
	t.Parallel()

	body := "## General Highlights\n* **Dependency Update**: Updated to the latest SDK module versions\n\n## Module Highlights\n" +
		"* `github.com/aws/aws-sdk-go-v2`: v1.48.0\n  * **Feature**: Core change.\n" +
		"* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`: [v1.21.21](x)\n  * **Bug Fix**: Fix a panic.\n" +
		"* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.65.0](service/fsx/CHANGELOG.md#v1650-2025-12-02)\n" +
		"  * **Feature**: S3 Access Points support for FSx for NetApp ONTAP\n" +
		"  * **Documentation**: Doc fix.\n" +
		"* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.80.0](service/s3control/CHANGELOG.md)\r\n" +
		"  * **Feature**: Cross-account access grants.\r\n"

	got := ParseNotes(body)
	if len(got) != 2 {
		t.Fatalf("modules = %v, want fsx and s3control only", got)
	}
	fsx := got["fsx"]
	if fsx.Version != "v1.65.0" || len(fsx.Notes) != 2 || fsx.Notes[0] != (Note{"Feature", "S3 Access Points support for FSx for NetApp ONTAP"}) {
		t.Errorf("fsx = %+v", fsx)
	}
	if s3c := got["s3control"]; s3c.Version != "v1.80.0" || len(s3c.Notes) != 1 {
		t.Errorf("s3control = %+v", s3c)
	}
}

func TestModuleName(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{"FSx": "fsx", "S3 Control": "s3control", "ACM PCA": "acmpca", "Bedrock AgentCore Control": "bedrockagentcorecontrol"} {
		if got := ModuleName(in); got != want {
			t.Errorf("ModuleName(%q) = %q, want %q", in, got, want)
		}
	}
}

func model(sdkID string, ops ...string) string {
	shapes := map[string]any{
		"com.amazonaws.x#Svc": map[string]any{"type": "service", "traits": map[string]any{"aws.api#service": map[string]any{"sdkId": sdkID}}},
	}
	for _, op := range ops {
		shapes["com.amazonaws.x#"+op] = map[string]any{"type": "operation"}
	}
	b, _ := json.Marshal(map[string]any{"smithy": "2.0", "shapes": shapes})
	return string(b)
}

// gitRepo builds a bare models repo with one commit per entry, each committed at the given time.
type commitSpec struct {
	at    time.Time
	files map[string]string // path -> content; "" deletes
}

func gitRepo(t *testing.T, commits []commitSpec) string {
	t.Helper()
	work := t.TempDir()
	git := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", work}, args...)...)
		cmd.Env = append(os.Environ(), append(env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git(nil, "init", "-q", "-b", "main")
	for i, c := range commits {
		for p, body := range c.files {
			full := filepath.Join(work, p)
			if body == "" {
				os.Remove(full)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		ts := c.at.Format(time.RFC3339)
		env := []string{"GIT_AUTHOR_DATE=" + ts, "GIT_COMMITTER_DATE=" + ts, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x"}
		git(env, "add", "-A")
		git(env, "commit", "-q", "--allow-empty", "-m", "c"+string(rune('0'+i)))
	}
	bare := filepath.Join(t.TempDir(), "models.git")
	if out, err := exec.Command("git", "clone", "-q", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	return bare
}

type fakeReleases []github.Release

func (f fakeReleases) ListReleases(_ context.Context, _ string, since time.Time) ([]github.Release, error) {
	// Newest first, through the first release before since.
	var out []github.Release
	for _, v := range slices.Backward(f) {
		out = append(out, v)
		if !since.IsZero() && v.PublishedAt.Before(since) {
			break
		}
	}
	return out, nil
}

func TestPull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	day := func(d, h int) time.Time { return time.Date(2026, 1, d, h, 0, 0, 0, time.UTC) }
	widget := "models/widget/service/2020-01-01/widget-2020-01-01.json"
	gadget := "models/gadget/service/2020-01-01/gadget-2020-01-01.json"
	repo := gitRepo(t, []commitSpec{
		{day(1, 19), map[string]string{widget: model("Widget Ctl", "CreateWidget"), gadget: model("Gadget", "GetGadget")}},
		// Day 2: Widget gains an operation; Gadget changes only a trait.
		{day(2, 19), map[string]string{widget: model("Widget Ctl", "CreateWidget", "TagWidget"),
			gadget: strings.Replace(model("Gadget", "GetGadget"), `"sdkId":"Gadget"`, `"sdkId":"Gadget","x":1`, 1)}},
		// A second commit the same day, before the release, also counts.
		{day(2, 19).Add(30 * time.Minute), map[string]string{widget: model("Widget Ctl", "CreateWidget", "TagWidget", "UntagWidget")}},
		// Day 3, after the day-2 release: belongs to day 3.
		{day(3, 19), map[string]string{gadget: model("Gadget", "GetGadget", "PutGadget")}},
		// Day 4's models, so the clone provably covers the day-3 release.
		{day(4, 19), map[string]string{widget: model("Widget Ctl", "CreateWidget", "TagWidget", "UntagWidget", "ListWidgets")}},
	})

	rels := fakeReleases{
		{TagName: "release-2026-01-01", PublishedAt: day(1, 20), HTMLURL: "u1"},
		{TagName: "release-2026-01-02", PublishedAt: day(2, 20), HTMLURL: "u2", Body: "## Module Highlights\n" +
			"* `github.com/aws/aws-sdk-go-v2/service/widgetctl`: [v1.2.0](x)\n  * **Feature**: Tagging.\n" +
			"* `github.com/aws/aws-sdk-go-v2/service/gadget`: [v1.1.0](x)\n  * **Feature**: Adds CBOR.\n" +
			"* `github.com/aws/aws-sdk-go-v2/service/sprocket`: [v1.0.1](x)\n  * **Feature**: Enable schema-based (de)serialization.\n"},
		{TagName: "release-2026-01-03", PublishedAt: day(3, 20), HTMLURL: "u3"},
	}

	st, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	fetch := false
	src := &config.Source{Name: "sdk", Type: config.SourceAWSSDKGoV2, ReleasesRepo: "aws/aws-sdk-go-v2", ModelsURL: "file://x",
		ModelsBranch: "main", Fetch: &fetch, Start: "2026-01-01"}
	tags := fakeTags{"release-2026-01-02": "abc123"}
	p := &Puller{Source: src, Models: &ModelsRepo{Dir: repo, Branch: "main"}, Releases: rels, Tags: tags, Store: st, Version: "test"}

	res, err := p.Pull(ctx, PullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Release 1 is the baseline only (published on the start date, at 20:00, so after start, but has no predecessor).
	if res.Releases != 2 || res.Latest != "release-2026-01-03" || len(res.Warnings) != 1 {
		t.Errorf("result = %+v", res)
	}

	get := func(id string) *Item {
		t.Helper()
		r, err := st.Get(ctx, id)
		if err != nil || r == nil {
			t.Fatalf("item %s: %v, %v", id, r, err)
		}
		var it Item
		if err := json.Unmarshal(r.Data, &it); err != nil {
			t.Fatal(err)
		}
		return &it
	}

	w := get("sdk/widgetctl/release-2026-01-02")
	if w.Release.Commit != "abc123" || w.Release.CommitURL != "https://github.com/aws/aws-sdk-go-v2/commit/abc123" {
		t.Errorf("release = %+v", w.Release)
	}
	if w.Status != items.StatusNew || strings.Join(w.Diff.AddedOperations, ",") != "TagWidget,UntagWidget" {
		t.Errorf("widget = %+v %+v", w, w.Diff)
	}
	if w.Service.SDKID != "Widget Ctl" || w.Service.ModuleVersion != "v1.2.0" || w.Notes[0].Text != "Tagging." || w.Provenance.ModelPath != widget {
		t.Errorf("widget service/notes/provenance = %+v %+v %+v", w.Service, w.Notes, w.Provenance)
	}

	g := get("sdk/gadget/release-2026-01-02")
	if g.Status != items.StatusSkipped || g.SkipReason != SkipNoAPIChange || g.Diff != nil {
		t.Errorf("gadget day 2 = %+v", g)
	}
	if s := get("sdk/sprocket/release-2026-01-02"); s.Status != items.StatusSkipped || s.SkipReason != SkipNoModel {
		t.Errorf("sprocket = %+v", s)
	}
	if g3 := get("sdk/gadget/release-2026-01-03"); g3.Status != items.StatusNew || strings.Join(g3.Diff.AddedOperations, ",") != "PutGadget" {
		t.Errorf("gadget day 3 = %+v", g3)
	}

	// The checkpoint is the last release; a second pull finds nothing new.
	res, err = p.Pull(ctx, PullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Releases != 0 {
		t.Errorf("second pull = %+v", res)
	}

	// Service filters apply.
	st2, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	src2 := *src
	src2.Services = []string{"gadget"}
	p2 := &Puller{Source: &src2, Models: p.Models, Releases: rels, Store: st2}
	if _, err := p2.Pull(ctx, PullOptions{}); err != nil {
		t.Fatal(err)
	}
	recs, err := st2.List(ctx, items.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if r.Service != "gadget" {
			t.Errorf("filtered pull stored %s", r.ID)
		}
	}
	if len(recs) != 2 {
		t.Errorf("filtered pull stored %d items, want 2", len(recs))
	}
}

// TestPullStaleClone checks that without a fetch, releases the clone may not cover aren't pulled, so their changes
// aren't stored as "no change", and the checkpoint stays put until a pull with a current clone.
func TestPullStaleClone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	day := func(d, h int) time.Time { return time.Date(2026, 1, d, h, 0, 0, 0, time.UTC) }
	widget := "models/widget/service/2020-01-01/widget-2020-01-01.json"
	upstream := gitRepo(t, []commitSpec{
		{day(1, 19), map[string]string{widget: model("Widget", "CreateWidget")}},
		{day(2, 19), map[string]string{widget: model("Widget", "CreateWidget", "TagWidget")}},
	})
	// The local clone is a copy of upstream as it was on day 1.
	stale := filepath.Join(t.TempDir(), "stale.git")
	if out, err := exec.Command("git", "clone", "-q", "--bare", upstream, stale).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", stale, "update-ref", "refs/heads/main", "main~1").CombinedOutput(); err != nil {
		t.Fatalf("rewind: %v: %s", err, out)
	}

	rels := fakeReleases{
		{TagName: "release-2026-01-01", PublishedAt: day(1, 20)},
		{TagName: "release-2026-01-02", PublishedAt: day(2, 20)},
	}
	st, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	noFetch := false
	src := &config.Source{Name: "sdk", Type: config.SourceAWSSDKGoV2, ModelsURL: upstream, ModelsBranch: "main", Fetch: &noFetch, Start: "2026-01-01"}
	p := &Puller{Source: src, Models: &ModelsRepo{Dir: stale, URL: upstream, Branch: "main"}, Releases: rels, Store: st}

	res, err := p.Pull(ctx, PullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Releases != 0 || !strings.Contains(strings.Join(res.Warnings, "\n"), "stopped before release-2026-01-02") {
		t.Errorf("stale pull = %+v", res)
	}
	if r, _ := st.Get(ctx, "sdk/widget/release-2026-01-02"); r != nil {
		t.Errorf("stale pull stored %s", r.ID)
	}
	if v, _ := st.Meta(ctx, p.checkpointKey()); v != "" {
		t.Errorf("stale pull moved the checkpoint to %s", v)
	}

	// With a fetch, the release is pulled and its new operation is found.
	fetch := true
	src.Fetch = &fetch
	res, err = p.Pull(ctx, PullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Releases != 1 {
		t.Fatalf("fetched pull = %+v", res)
	}
	r, err := st.Get(ctx, "sdk/widget/release-2026-01-02")
	if err != nil || r == nil || r.Status != items.StatusNew {
		t.Errorf("fetched item = %+v, %v", r, err)
	}
}

// TestPullSinceInclusive checks that an explicit --since includes a release published exactly then, while resuming
// from the checkpoint doesn't repeat the last release.
func TestPullSinceInclusive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	day := func(d, h int) time.Time { return time.Date(2026, 1, d, h, 0, 0, 0, time.UTC) }
	widget := "models/widget/service/2020-01-01/widget-2020-01-01.json"
	repo := gitRepo(t, []commitSpec{
		{day(1, 19), map[string]string{widget: model("Widget", "CreateWidget")}},
		{day(2, 19), map[string]string{widget: model("Widget", "CreateWidget", "TagWidget")}},
		{day(3, 19), map[string]string{widget: model("Widget", "CreateWidget", "TagWidget", "UntagWidget")}},
	})
	rels := fakeReleases{
		{TagName: "release-2026-01-01", PublishedAt: day(1, 20)},
		{TagName: "release-2026-01-02", PublishedAt: day(2, 20)},
	}
	st, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	noFetch := false
	src := &config.Source{Name: "sdk", Type: config.SourceAWSSDKGoV2, ModelsBranch: "main", Fetch: &noFetch, Start: "2026-01-01"}
	p := &Puller{Source: src, Models: &ModelsRepo{Dir: repo, Branch: "main"}, Releases: rels, Store: st}

	res, err := p.Pull(ctx, PullOptions{Since: day(2, 20)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Releases != 1 || res.Latest != "release-2026-01-02" {
		t.Errorf("--since at a release's publication time = %+v", res)
	}

	if _, err := p.Pull(ctx, PullOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err = p.Pull(ctx, PullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Releases != 0 {
		t.Errorf("resume from checkpoint repeated a release: %+v", res)
	}
}

func TestOperations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	day := func(d int) time.Time { return time.Date(2026, 1, d, 19, 0, 0, 0, time.UTC) }
	widget := "models/widget/service/2020-01-01/widget-2020-01-01.json"
	gadget := "models/gadget/service/2020-01-01/gadget-2020-01-01.json"
	dir := gitRepo(t, []commitSpec{
		{day(1), map[string]string{widget: model("Widget", "CreateWidget"), gadget: model("Gadget", "GetGadget")}},
	})
	repo := &ModelsRepo{Dir: dir, Branch: "main"}
	cache := filepath.Join(t.TempDir(), "ops.json")

	ops, err := repo.Operations(ctx, cache)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ops.Names, []string{"CreateWidget", "GetGadget"}) || !ops.Has("GetGadget") || ops.Has("StartTime") {
		t.Errorf("operations = %+v", ops)
	}

	// The cache is used while the tip is unchanged.
	b, err := os.ReadFile(cache)
	if err != nil {
		t.Fatal(err)
	}
	var cached Operations
	if err := json.Unmarshal(b, &cached); err != nil {
		t.Fatal(err)
	}
	cached.Names = append(cached.Names, "FromCache")
	b, _ = json.Marshal(&cached)
	if err := os.WriteFile(cache, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if ops, err = repo.Operations(ctx, cache); err != nil || !ops.Has("FromCache") {
		t.Errorf("cached operations = %+v, %v", ops, err)
	}
}

type fakeTags map[string]string

func (f fakeTags) TagCommits(context.Context, string, string) (map[string]string, error) {
	return f, nil
}

func TestParseLsRemote(t *testing.T) {
	t.Parallel()

	got := parseLsRemote("aaa\trefs/tags/release-1\nbbb\trefs/tags/release-1^{}\nccc\trefs/tags/release-2\nddd\trefs/heads/main\n")
	if len(got) != 2 || got["release-1"] != "bbb" || got["release-2"] != "ccc" {
		t.Errorf("parseLsRemote = %v", got)
	}
}

// TestWriteFileAtomicConcurrent checks that concurrent writers of one cache file all succeed and leave no temporary
// files. A shared temporary name made all but one writer's rename fail.
func TestWriteFileAtomicConcurrent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "ops.json")
	const n = 50
	errs := make(chan error, n)
	start := make(chan struct{})
	for i := range n {
		go func() {
			<-start
			errs <- writeFileAtomic(path, []byte(strings.Repeat("x", 1<<16+i)))
		}()
	}
	close(start)
	for range n {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if m, _ := filepath.Glob(path + ".*.tmp"); len(m) > 0 {
		t.Errorf("leftover temporary files: %v", m)
	}
	if b, err := os.ReadFile(path); err != nil || len(b) < 1<<16 || strings.Trim(string(b), "x") != "" {
		t.Errorf("final file: %d bytes, %v", len(b), err)
	}
}

func TestOperationsHasConcurrent(t *testing.T) {
	t.Parallel()

	ops := &Operations{Names: []string{"CreateWidget", "DeleteWidget"}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if !ops.Has("CreateWidget") || ops.Has("StartTime") {
				t.Error("Has is wrong")
			}
		})
	}
	wg.Wait()
}

// TestPullLateModels checks that models committed after a release go to it when they change a module its notes list
// and the next release's don't, stay with the next release otherwise, and that a re-pull removes items it no longer
// produces.
func TestPullLateModels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	at := func(d, h, m int) time.Time { return time.Date(2026, 1, d, h, m, 0, 0, time.UTC) }
	widget := "models/widget/service/2020-01-01/widget-2020-01-01.json"
	gadget := "models/gadget/service/2020-01-01/gadget-2020-01-01.json"
	gizmo := "models/gizmo/service/2020-01-01/gizmo-2020-01-01.json"
	notes := func(mods ...string) string {
		b := "## Module Highlights\n"
		for _, m := range mods {
			b += "* `github.com/aws/aws-sdk-go-v2/service/" + m + "`: [v1.1.0](x)\n  * **Feature**: " + m + " change.\n"
		}
		return b
	}
	commits := []commitSpec{
		{at(1, 19, 0), map[string]string{widget: model("Widget", "CreateWidget"), gadget: model("Gadget", "GetGadget"),
			gizmo: model("Gizmo", "GetGizmo")}},
		// Day 2's release is at 20:00. A commit with no model change, then its widget models, land after it.
		{at(2, 20, 10), map[string]string{"README.md": "x"}},
		{at(2, 20, 30), map[string]string{widget: model("Widget", "CreateWidget", "TagWidget")}},
		// Gadget's models come before the 22:00 release that lists it; day 2's release lists gadget too, so they're
		// the 22:00 release's.
		{at(2, 21, 0), map[string]string{gadget: model("Gadget", "GetGadget", "PutGadget")}},
		{at(3, 19, 0), map[string]string{widget: model("Widget", "CreateWidget", "TagWidget", "UntagWidget")}},
		// Day 3 deprecates gizmo, and its model is removed after the release: removals don't count.
		{at(3, 20, 30), map[string]string{gizmo: ""}},
		// So the clone provably covers day 3.
		{at(4, 19, 0), map[string]string{"README.md": "y"}},
	}
	rels := fakeReleases{
		{TagName: "release-2026-01-01", PublishedAt: at(1, 20, 0)},
		{TagName: "release-2026-01-02", PublishedAt: at(2, 20, 0), Body: notes("widget", "gadget")},
		{TagName: "release-2026-01-02.2", PublishedAt: at(2, 22, 0), Body: notes("gadget")},
		{TagName: "release-2026-01-03", PublishedAt: at(3, 20, 0), Body: notes("widget", "gizmo")},
	}
	open := func() *items.Store {
		st, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		return st
	}
	noFetch := false
	src := &config.Source{Name: "sdk", Type: config.SourceAWSSDKGoV2, ModelsBranch: "main", Fetch: &noFetch, Start: "2026-01-01"}
	get := func(st *items.Store, id string) *Item {
		t.Helper()
		r, err := st.Get(ctx, id)
		if err != nil || r == nil {
			t.Fatalf("item %s: %v, %v", id, r, err)
		}
		var it Item
		if err := json.Unmarshal(r.Data, &it); err != nil {
			t.Fatal(err)
		}
		return &it
	}
	ops := func(it *Item) string {
		if it.Diff == nil {
			return ""
		}
		return strings.Join(it.Diff.AddedOperations, ",")
	}

	st := open()
	// A stale item from an earlier pull of day 2.
	if err := st.Put(ctx, []items.Record{{ID: "sdk/sprocket/release-2026-01-02", Source: "sdk", Service: "sprocket", Release: "release-2026-01-02",
		PublishedAt: at(2, 20, 0), Status: items.StatusNew, Data: []byte(`{}`)}}); err != nil {
		t.Fatal(err)
	}
	p := &Puller{Source: src, Models: &ModelsRepo{Dir: gitRepo(t, commits), Branch: "main"}, Releases: rels, Store: st}
	res, err := p.Pull(ctx, PullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Releases != 3 || !strings.Contains(strings.Join(res.Warnings, "\n"), "release-2026-01-02: removed 1 item(s) a re-pull no longer produces: sdk/sprocket/release-2026-01-02") {
		t.Errorf("result = %+v", res)
	}
	if w := get(st, "sdk/widget/release-2026-01-02"); w.Status != items.StatusNew || ops(w) != "TagWidget" {
		t.Errorf("day 2 widget = %s %q", w.Status, ops(w))
	}
	if g := get(st, "sdk/gadget/release-2026-01-02"); g.Status != items.StatusSkipped || g.SkipReason != SkipNoModel {
		t.Errorf("day 2 gadget = %s %s", g.Status, g.SkipReason)
	}
	if g := get(st, "sdk/gadget/release-2026-01-02.2"); g.Status != items.StatusNew || ops(g) != "PutGadget" {
		t.Errorf("day 2.2 gadget = %s %q", g.Status, ops(g))
	}
	if w := get(st, "sdk/widget/release-2026-01-03"); ops(w) != "UntagWidget" {
		t.Errorf("day 3 widget = %q", ops(w))
	}
	if g := get(st, "sdk/gizmo/release-2026-01-03"); g.SkipReason != SkipNoModel {
		t.Errorf("day 3 gizmo = %s %s", g.Status, g.SkipReason)
	}

	// Resuming from the checkpoint uses the stored head of the last release as the next one's base.
	p2 := &Puller{Source: src, Models: p.Models, Releases: append(rels, github.Release{TagName: "release-2026-01-04", PublishedAt: at(4, 20, 0), Body: notes("widget")}), Store: st}
	if p2.Pull(ctx, PullOptions{}); p2.heads["release-2026-01-03"].SHA != get(st, "sdk/widget/release-2026-01-03").Provenance.Head.SHA {
		t.Errorf("resumed base = %+v", p2.heads["release-2026-01-03"])
	}
}

// TestPullModelsPending checks that the newest release isn't pulled while its models may still be landing.
func TestPullModelsPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	at := func(d, h, m int) time.Time { return time.Date(2026, 1, d, h, m, 0, 0, time.UTC) }
	widget := "models/widget/service/2020-01-01/widget-2020-01-01.json"
	body := "## Module Highlights\n* `github.com/aws/aws-sdk-go-v2/service/widget`: [v1.1.0](x)\n  * **Feature**: Tagging.\n"
	rels := fakeReleases{
		{TagName: "release-2026-01-01", PublishedAt: at(1, 20, 0)},
		{TagName: "release-2026-01-02", PublishedAt: at(2, 20, 0), Body: body},
	}
	fetch := true
	for _, tc := range []struct {
		name    string
		commits []commitSpec
		now     time.Time
		want    string // warning; "" pulls
	}{
		{"models not yet landed", []commitSpec{{at(1, 19, 0), map[string]string{widget: model("Widget", "CreateWidget")}}},
			at(2, 20, 15), "stopped before release-2026-01-02: none of its listed modules has a model change yet"},
		{"models landed after it", []commitSpec{{at(1, 19, 0), map[string]string{widget: model("Widget", "CreateWidget")}},
			{at(2, 20, 30), map[string]string{widget: model("Widget", "CreateWidget", "TagWidget")}}},
			at(2, 20, 45), "stopped before release-2026-01-02: models committed after it"},
		{"models landed before it", []commitSpec{{at(1, 19, 0), map[string]string{widget: model("Widget", "CreateWidget")}},
			{at(2, 19, 30), map[string]string{widget: model("Widget", "CreateWidget", "TagWidget")}}},
			at(2, 20, 15), ""},
		{"lag passed", []commitSpec{{at(1, 19, 0), map[string]string{widget: model("Widget", "CreateWidget")}}},
			at(3, 1, 0), ""},
	} {
		st, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
		if err != nil {
			t.Fatal(err)
		}
		repo := gitRepo(t, tc.commits)
		src := &config.Source{Name: "sdk", Type: config.SourceAWSSDKGoV2, ModelsURL: repo, ModelsBranch: "main", Fetch: &fetch, Start: "2026-01-01"}
		p := &Puller{Source: src, Models: &ModelsRepo{Dir: repo, URL: repo, Branch: "main"}, Releases: rels, Store: st, Now: func() time.Time { return tc.now }}
		res, err := p.Pull(ctx, PullOptions{})
		st.Close()
		if err != nil {
			t.Fatal(err)
		}
		warns := strings.Join(res.Warnings, "\n")
		switch {
		case tc.want == "" && res.Releases != 1:
			t.Errorf("%s: result = %+v", tc.name, res)
		case tc.want != "" && (res.Releases != 0 || !strings.Contains(warns, tc.want)):
			t.Errorf("%s: result = %+v", tc.name, res)
		}
	}
}
