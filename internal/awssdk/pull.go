package awssdk

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/YakDriver/avizor/internal/config"
	"github.com/YakDriver/avizor/internal/github"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/smithy"
)

// Skip reasons.
const (
	SkipNoAPIChange  = "no API change: the model changed only in documentation, traits, or rules"
	SkipNoModel      = "release note without a model change"
	SkipModelRemoved = "service model removed"
)

// Item is one service's change in one SDK release. Every field is evidence: observed in a release or a model, with
// where it came from in Provenance.
type Item struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Status string `json:"status"`
	// SkipReason is set when Status is skipped.
	SkipReason string `json:"skip_reason,omitempty"`

	Service Service      `json:"service"`
	Release Release      `json:"release"`
	Notes   []Note       `json:"notes"`
	Diff    *smithy.Diff `json:"diff,omitempty"`

	Provenance Provenance `json:"provenance"`
}

// Service identifies the service.
type Service struct {
	// SDKID is the model's SDK ID, e.g. "FSx". Empty for items from release notes without a model change.
	SDKID string `json:"sdk_id,omitempty"`
	// Module is the aws-sdk-go-v2 service module, e.g. "fsx".
	Module string `json:"module"`
	// ModulePath is the Go module path.
	ModulePath string `json:"module_path"`
	// ModuleVersion is the module's version in the release, from the release notes.
	ModuleVersion string `json:"module_version,omitempty"`
}

// Release identifies the SDK release.
type Release struct {
	Tag         string    `json:"tag"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	// Commit is the commit the release tag points to, and CommitURL its GitHub page. Maintainers cite it in tracking
	// issues. Empty if the tags couldn't be listed.
	Commit    string `json:"commit,omitempty"`
	CommitURL string `json:"commit_url,omitempty"`
}

// Provenance records where an item's facts came from.
type Provenance struct {
	ReleasesRepo string `json:"releases_repo"`
	ModelsRepo   string `json:"models_repo"`
	// ModelPath is the model file in the models repo.
	ModelPath string `json:"model_path,omitempty"`
	// Base and Head are the models repo commits compared: the last commit at or before the previous and this release.
	Base     Commit    `json:"models_base"`
	Head     Commit    `json:"models_head"`
	PulledAt time.Time `json:"pulled_at"`
	PulledBy string    `json:"pulled_by"`
}

// Releases lists SDK releases. *github.Client implements it.
type Releases interface {
	ListReleases(ctx context.Context, repo string, since time.Time) ([]github.Release, error)
}

// TagCommits lists the commits a repo's tags point to, peeling annotated tags.
type TagCommits interface {
	TagCommits(ctx context.Context, repo, pattern string) (map[string]string, error)
}

// GitTags implements TagCommits with git ls-remote against GitHub.
type GitTags struct{}

// TagCommits runs git ls-remote for tags matching pattern (such as "release-*") in the GitHub repo.
func (GitTags) TagCommits(ctx context.Context, repo, pattern string) (map[string]string, error) {
	out, err := run(ctx, "", "ls-remote", "--tags", "https://github.com/"+repo+".git", "refs/tags/"+pattern)
	if err != nil {
		return nil, err
	}
	return parseLsRemote(out), nil
}

// parseLsRemote maps tag names to commits. A peeled entry ("<tag>^{}") is the commit an annotated tag points to, and
// wins over the tag object.
func parseLsRemote(out string) map[string]string {
	tags := map[string]string{}
	for line := range strings.Lines(out) {
		sha, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		name, ok := strings.CutPrefix(ref, "refs/tags/")
		if !ok {
			continue
		}
		if base, peeled := strings.CutSuffix(name, "^{}"); peeled {
			tags[base] = sha
		} else if _, seen := tags[name]; !seen {
			tags[name] = sha
		}
	}
	return tags
}

// Puller turns SDK releases into items.
type Puller struct {
	Source   *config.Source
	Models   *ModelsRepo
	Releases Releases
	// Tags, if set, resolves release tags to commits.
	Tags  TagCommits
	Store *items.Store
	// Version is recorded in provenance.
	Version string
	Log     func(format string, args ...any)

	cache   map[string]*smithy.Model
	commits map[string]string
}

// PullOptions control one pull.
type PullOptions struct {
	// Since, if set, pulls releases published at or after it, without reading or moving the checkpoint.
	Since time.Time
	// NoFetch skips fetching the models clone.
	NoFetch bool
}

// PullResult summarizes a pull.
type PullResult struct {
	Source   string   `json:"source"`
	Since    string   `json:"since"`
	Releases int      `json:"releases"`
	New      int      `json:"new"`
	Skipped  int      `json:"skipped"`
	Warnings []string `json:"warnings,omitempty"`
	// Latest is the newest release pulled.
	Latest   string `json:"latest_release,omitempty"`
	Duration string `json:"duration"`
}

func (p *Puller) checkpointKey() string {
	return p.Source.Name + ":last_release_published_at"
}

func (p *Puller) logf(format string, args ...any) {
	if p.Log != nil {
		p.Log(format, args...)
	}
}

// Pull fetches releases after the checkpoint (or since opts.Since), computes each one's model changes, and stores one
// item per service.
func (p *Puller) Pull(ctx context.Context, opts PullOptions) (*PullResult, error) {
	start := time.Now()
	res := &PullResult{Source: p.Source.Name}

	since, inclusive, err := p.since(ctx, opts.Since)
	if err != nil {
		return nil, err
	}
	res.Since = since.UTC().Format(time.RFC3339)

	current, err := p.Models.Ensure(ctx, p.Source.FetchModels() && !opts.NoFetch, p.logf)
	if err != nil {
		return nil, err
	}
	// Without a fetch, the clone may be missing models for recent releases, and their changes would look like no change.
	// Only releases the clone provably covers, those published before its newest commit, are pulled then.
	var coveredUntil time.Time
	if !current {
		tip, err := p.Models.Tip(ctx)
		if err != nil {
			return nil, err
		}
		coveredUntil = tip.Time
	}

	rels, err := p.Releases.ListReleases(ctx, p.Source.ReleasesRepo, since)
	if err != nil {
		return nil, fmt.Errorf("listing %s releases: %w", p.Source.ReleasesRepo, err)
	}
	// Oldest first. The oldest may precede the window; it's only the baseline for the next one.
	slices.Reverse(rels)

	p.cache = map[string]*smithy.Model{}
	if p.Tags != nil {
		if p.commits, err = p.Tags.TagCommits(ctx, p.Source.ReleasesRepo, "release-*"); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("release commits unavailable: %v", err))
		}
	}
	for i, rel := range rels {
		if rel.PublishedAt.Before(since) || (!inclusive && rel.PublishedAt.Equal(since)) {
			continue
		}
		if i == 0 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: no earlier release to compare with; skipped", rel.TagName))
			continue
		}
		if !coveredUntil.IsZero() && !rel.PublishedAt.Before(coveredUntil) {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"stopped before %s: the models clone wasn't fetched and its newest commit (%s) may predate this release's models; pull again with fetch enabled",
				rel.TagName, coveredUntil.Format(time.RFC3339)))
			break
		}
		recs, warn, err := p.pullRelease(ctx, rels[i-1], rel)
		if err != nil {
			return res, fmt.Errorf("%s: %w", rel.TagName, err)
		}
		if warn != "" {
			res.Warnings = append(res.Warnings, warn)
		}
		if err := p.Store.Put(ctx, recs); err != nil {
			return res, err
		}
		for _, r := range recs {
			if r.Status == items.StatusNew {
				res.New++
			} else {
				res.Skipped++
			}
		}
		res.Releases++
		res.Latest = rel.TagName
		if opts.Since.IsZero() {
			if err := p.Store.SetMeta(ctx, p.checkpointKey(), rel.PublishedAt.UTC().Format(time.RFC3339)); err != nil {
				return res, err
			}
		}
		p.logf("%s: %d item(s)", rel.TagName, len(recs))
	}
	res.Duration = time.Since(start).Round(time.Millisecond).String()
	return res, nil
}

// since returns where the pull starts: the override, the checkpoint, or the source's start. It reports whether a
// release published exactly at that time is included: yes for an override or start, no for the checkpoint, which is
// the last release already pulled.
func (p *Puller) since(ctx context.Context, override time.Time) (time.Time, bool, error) {
	if !override.IsZero() {
		return override, true, nil
	}
	v, err := p.Store.Meta(ctx, p.checkpointKey())
	if err != nil {
		return time.Time{}, false, err
	}
	if v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid checkpoint %q: %w", v, err)
		}
		return t, false, nil
	}
	t, err := config.ParseSince(p.Source.Start, time.Now())
	return t, true, err
}

func (p *Puller) pullRelease(ctx context.Context, prev, rel github.Release) ([]items.Record, string, error) {
	base, err := p.Models.CommitAt(ctx, prev.PublishedAt)
	if err != nil {
		return nil, "", err
	}
	head, err := p.Models.CommitAt(ctx, rel.PublishedAt)
	if err != nil {
		return nil, "", err
	}
	if base.SHA == "" {
		return nil, fmt.Sprintf("%s: models repository has no history before %s; skipped", rel.TagName, prev.PublishedAt.Format(time.DateOnly)), nil
	}

	notes := ParseNotes(rel.Body)
	changes, err := p.Models.ChangedModels(ctx, base.SHA, head.SHA)
	if err != nil {
		return nil, "", err
	}

	prov := Provenance{
		ReleasesRepo: p.Source.ReleasesRepo,
		ModelsRepo:   p.Source.ModelsURL,
		Base:         base,
		Head:         head,
		PulledAt:     time.Now().UTC(),
		PulledBy:     "avizor " + p.Version,
	}
	release := Release{Tag: rel.TagName, URL: rel.HTMLURL, PublishedAt: rel.PublishedAt.UTC()}
	if sha := p.commits[rel.TagName]; sha != "" {
		release.Commit = sha
		release.CommitURL = "https://github.com/" + p.Source.ReleasesRepo + "/commit/" + sha
	}

	byModule := map[string]*Item{}
	nextCache := map[string]*smithy.Model{}
	for _, ch := range changes {
		it, err := p.modelItem(ctx, ch, base.SHA, head.SHA, nextCache)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", ch.Path, err)
		}
		if it == nil {
			continue
		}
		it.Release = release
		it.Provenance = prov
		it.Provenance.ModelPath = ch.Path
		byModule[it.Service.Module] = it
	}
	p.cache = nextCache

	for mod, sn := range notes {
		it, ok := byModule[mod]
		if !ok {
			it = &Item{Status: items.StatusSkipped, SkipReason: SkipNoModel, Service: Service{Module: mod}, Release: release, Provenance: prov}
			byModule[mod] = it
		}
		it.Service.ModuleVersion = sn.Version
		it.Notes = sn.Notes
	}

	var recs []items.Record
	for _, mod := range sortedKeys(byModule) {
		if !p.wanted(mod) {
			continue
		}
		it := byModule[mod]
		it.Source = p.Source.Name
		it.ID = ItemID(p.Source.Name, mod, rel.TagName)
		it.Service.Module = mod
		it.Service.ModulePath = "github.com/aws/aws-sdk-go-v2/service/" + mod
		if it.Notes == nil {
			it.Notes = []Note{}
		}
		data, err := json.Marshal(it)
		if err != nil {
			return nil, "", err
		}
		recs = append(recs, items.Record{
			ID: it.ID, Source: it.Source, Service: mod, Release: rel.TagName, PublishedAt: release.PublishedAt,
			Status: it.Status, PulledAt: prov.PulledAt, Data: data,
		})
	}
	return recs, "", nil
}

// modelItem diffs one changed model file. Parsed head models are added to next, to serve as the following release's
// base without parsing them again.
func (p *Puller) modelItem(ctx context.Context, ch ModelChange, base, head string, next map[string]*smithy.Model) (*Item, error) {
	var oldM, newM *smithy.Model
	if ch.Status != "A" {
		m, err := p.model(ctx, base, ch.Path, nil)
		if err != nil {
			return nil, err
		}
		oldM = m
	}
	if ch.Status == "D" {
		return &Item{Status: items.StatusSkipped, SkipReason: SkipModelRemoved,
			Service: Service{SDKID: oldM.SDKID, Module: ModuleName(oldM.SDKID)}}, nil
	}
	m, err := p.model(ctx, head, ch.Path, next)
	if err != nil {
		return nil, err
	}
	newM = m

	d := smithy.Compare(oldM, newM)
	it := &Item{Status: items.StatusNew, Diff: d, Service: Service{SDKID: newM.SDKID, Module: ModuleName(newM.SDKID)}}
	if d.Empty() {
		it.Status, it.SkipReason = items.StatusSkipped, SkipNoAPIChange
		// Keep retargeted members as evidence: they can carry constraint changes.
		if len(d.RetargetedMembers) == 0 {
			it.Diff = nil
		}
	}
	return it, nil
}

func (p *Puller) model(ctx context.Context, commit, path string, next map[string]*smithy.Model) (*smithy.Model, error) {
	id, err := p.Models.BlobID(ctx, commit, path)
	if err != nil {
		return nil, err
	}
	m, ok := p.cache[id]
	if !ok {
		data, err := p.Models.Blob(ctx, id)
		if err != nil {
			return nil, err
		}
		if m, err = smithy.Parse(data); err != nil {
			return nil, err
		}
	}
	if next != nil {
		next[id] = m
	}
	return m, nil
}

func (p *Puller) wanted(mod string) bool {
	if len(p.Source.Services) > 0 && !slices.Contains(p.Source.Services, mod) {
		return false
	}
	return !slices.Contains(p.Source.ExcludeServices, mod)
}

// ItemID is "<source>/<module>/<release tag>".
func ItemID(source, module, tag string) string {
	return source + "/" + module + "/" + tag
}

// Summary is a one-line description of an item's change.
func (it *Item) Summary() string {
	if it.Status == items.StatusSkipped {
		return it.SkipReason
	}
	d := it.Diff
	var parts []string
	add := func(n int, what string) {
		if n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+what)
		}
	}
	add(len(d.AddedOperations), "new operations")
	add(len(d.AddedMembers), "new members")
	add(len(d.AddedEnumValues), "new enum values")
	add(len(d.AddedShapes), "new shapes")
	add(len(d.ChangedMembers)+len(d.ChangedEnumValues), "changed members")
	add(len(d.RemovedOperations)+len(d.RemovedMembers)+len(d.RemovedEnumValues)+len(d.RemovedShapes), "removals")
	return strings.Join(parts, ", ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
