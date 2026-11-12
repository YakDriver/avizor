// Package enrich relates an SDK item to a target provider: which service package owns it, whether the provider's
// go.mod already includes the item's module version and which commit brought it in, and what kind of work the change
// suggests. Observations (go.mod contents, git history, code search) are evidence; work kinds are inferences.
package enrich

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/YakDriver/radar/internal/awssdk"
	"github.com/YakDriver/radar/internal/services"
	"github.com/YakDriver/radar/internal/smithy"
)

// Service statuses.
const (
	// ServiceImplemented means the provider has a service package for the module.
	ServiceImplemented = "implemented"
	// ServiceNotImplemented means the provider lists the service but marks it not implemented.
	ServiceNotImplemented = "not_implemented"
	// ServiceUnknown means the provider doesn't list the module.
	ServiceUnknown = "unknown"
)

// Availability statuses.
const (
	// Available means the provider's go.mod requires the item's module version or later.
	Available = "available"
	// Pending means the provider's go.mod requires an earlier version.
	Pending = "pending"
	// NotADependency means the provider's go.mod doesn't require the module.
	NotADependency = "not_a_dependency"
	// AvailabilityUnknown means there's no checkout or no version to compare.
	AvailabilityUnknown = "unknown"
)

// Work kinds, inferred from the model change.
const (
	WorkNewResource   = "new_resource"
	WorkNewDataSource = "new_data_source"
	WorkNewOperation  = "new_operation"
	WorkNewArgument   = "new_argument"
	WorkNewAttribute  = "new_attribute"
	WorkNewEnumValue  = "new_enum_value"
	WorkTypeChange    = "type_change"
	WorkRemoval       = "removal"
	WorkTagging       = "tagging"
)

// Result is an item's enrichment for one target.
type Result struct {
	Target       string        `json:"target"`
	Service      ServiceMatch  `json:"service"`
	Availability *Availability `json:"availability,omitempty"`
	// Work lists inferred kinds of provider work, one per operation group or changed shape member.
	Work     []Work   `json:"work"`
	Warnings []string `json:"warnings,omitempty"`
}

// ServiceMatch is the provider service package for the item's module. It's evidence from names_data.hcl.
type ServiceMatch struct {
	Status  string `json:"status"`
	Package string `json:"package,omitempty"`
	Label   string `json:"label,omitempty"`
	Basis   string `json:"basis"`
}

// Availability compares the item's module version with the provider's go.mod.
type Availability struct {
	Status          string `json:"status"`
	ModulePath      string `json:"module_path"`
	ItemVersion     string `json:"item_version,omitempty"`
	ProviderVersion string `json:"provider_version,omitempty"`
	// Source is where go.mod was read: "<checkout>@<commit> (<commit time>)".
	Source string `json:"source"`
	// Bump is the first-parent commit that first required the item's version or later, when Status is available.
	Bump *Bump `json:"bump,omitempty"`
}

// Bump is the commit that brought a module version into the provider.
type Bump struct {
	Commit  string    `json:"commit"`
	Time    time.Time `json:"time"`
	Subject string    `json:"subject"`
	// PR is the pull request number from the commit subject ("Merge pull request #N" or "(#N)"), or 0.
	PR      int    `json:"pr,omitempty"`
	Version string `json:"version"`
}

// Work is one inferred kind of provider work.
type Work struct {
	Kind string `json:"kind"`
	// Subject is what the work is about: a resource noun for operation groups, "Shape.Member" for members.
	Subject string `json:"subject"`
	// Operations are the new operations in the group, or the operations whose input or output reaches the shape.
	Operations []string `json:"operations,omitempty"`
	Basis      string   `json:"basis"`
	// UsedBy are provider files that call one of Operations, from a code search. It's evidence.
	UsedBy []string `json:"used_by,omitempty"`
}

// Enricher enriches items for one target.
type Enricher struct {
	Target   string
	Services *services.Map
	// LabelPrefix is the target's service label prefix, e.g. "service/".
	LabelPrefix string
	// Checkout is the provider clone. Without it, availability and code use are unknown.
	Checkout string
	// Ref is the checkout ref whose go.mod and history are read. Empty means HEAD.
	Ref string
	// Models loads the item's model at its head commit, to find which operations reach changed shapes. Nil skips that.
	Models *awssdk.ModelsRepo
	// NoCode skips searching the checkout for operation calls.
	NoCode bool

	rgPath string
}

// Enrich relates it to the target.
func (e *Enricher) Enrich(ctx context.Context, it *awssdk.Item) *Result {
	res := &Result{Target: e.Target, Work: []Work{}}
	warnf := func(format string, args ...any) { res.Warnings = append(res.Warnings, fmt.Sprintf(format, args...)) }

	res.Service = e.service(it)
	if e.Checkout != "" {
		av, err := e.availability(ctx, it)
		if err != nil {
			warnf("availability: %v", err)
		} else {
			res.Availability = av
		}
	}

	var usages map[string]*smithy.Usage
	if it.Diff != nil && e.Models != nil && it.Provenance.ModelPath != "" && it.Provenance.Head.SHA != "" {
		m, err := e.model(ctx, it)
		if err != nil {
			warnf("loading model: %v", err)
		} else {
			usages = m.Usages()
		}
	}
	res.Work = work(it.Diff, usages)

	if !e.NoCode && e.Checkout != "" && res.Service.Status == ServiceImplemented {
		if err := e.usedBy(ctx, res.Service.Package, res.Work); err != nil {
			warnf("code search: %v", err)
		}
	}
	return res
}

func (e *Enricher) service(it *awssdk.Item) ServiceMatch {
	if e.Services == nil {
		return ServiceMatch{Status: ServiceUnknown, Basis: "no service map (names_data.hcl) for the target"}
	}
	svc := e.Services.ByModule(it.Service.Module)
	if svc == nil {
		return ServiceMatch{Status: ServiceUnknown, Basis: fmt.Sprintf("no service in names_data.hcl uses module %s", it.Service.Module)}
	}
	m := ServiceMatch{Status: ServiceImplemented, Package: svc.Package, Label: e.LabelPrefix + svc.Package,
		Basis: fmt.Sprintf("names_data.hcl service %q uses module %s", svc.Package, it.Service.Module)}
	if svc.NotImplemented {
		m.Status, m.Label = ServiceNotImplemented, ""
		m.Basis += " and is marked not_implemented"
	}
	return m
}

func (e *Enricher) ref() string {
	if e.Ref == "" {
		return "HEAD"
	}
	return e.Ref
}

func (e *Enricher) availability(ctx context.Context, it *awssdk.Item) (*Availability, error) {
	commit, err := git(ctx, e.Checkout, "log", "-1", "--format=%H %cI", e.ref(), "--")
	if err != nil {
		return nil, err
	}
	sha, when, _ := strings.Cut(strings.TrimSpace(commit), " ")
	gomod, err := git(ctx, e.Checkout, "show", sha+":go.mod")
	if err != nil {
		return nil, err
	}
	av := &Availability{
		Status:      AvailabilityUnknown,
		ModulePath:  it.Service.ModulePath,
		ItemVersion: it.Service.ModuleVersion,
		Source:      fmt.Sprintf("%s go.mod at %s (%s)", e.Checkout, sha, when),
	}
	av.ProviderVersion = requiredVersion(gomod, it.Service.ModulePath)
	switch {
	case av.ProviderVersion == "":
		av.Status = NotADependency
		return av, nil
	case av.ItemVersion == "":
		return av, nil
	}
	cmp, ok := compareVersions(av.ProviderVersion, av.ItemVersion)
	if !ok {
		return av, nil
	}
	if cmp < 0 {
		av.Status = Pending
		return av, nil
	}
	av.Status = Available
	bump, err := e.bump(ctx, sha, it)
	if err != nil {
		return nil, err
	}
	av.Bump = bump
	return av, nil
}

var prNumberRE = regexp.MustCompile(`(?:Merge pull request #|\(#)(\d+)`)

// bump finds the first first-parent commit at or after the item's release whose go.mod requires the item's version
// or later.
func (e *Enricher) bump(ctx context.Context, head string, it *awssdk.Item) (*Bump, error) {
	args := []string{"log", "--first-parent", "--reverse", "--format=%H%x09%cI%x09%s", "-G", regexp.QuoteMeta(it.Service.ModulePath) + " v"}
	if !it.Release.PublishedAt.IsZero() {
		args = append(args, "--since="+it.Release.PublishedAt.Add(-24*time.Hour).UTC().Format(time.RFC3339))
	}
	args = append(args, head, "--", "go.mod")
	out, err := git(ctx, e.Checkout, args...)
	if err != nil {
		return nil, err
	}
	for line := range strings.Lines(out) {
		f := strings.SplitN(strings.TrimSpace(line), "\t", 3)
		if len(f) != 3 {
			continue
		}
		gomod, err := git(ctx, e.Checkout, "show", f[0]+":go.mod")
		if err != nil {
			return nil, err
		}
		v := requiredVersion(gomod, it.Service.ModulePath)
		if c, ok := compareVersions(v, it.Service.ModuleVersion); !ok || c < 0 {
			continue
		}
		b := &Bump{Commit: f[0], Subject: f[2], Version: v}
		b.Time, _ = time.Parse(time.RFC3339, f[1])
		if m := prNumberRE.FindStringSubmatch(f[2]); m != nil {
			b.PR, _ = strconv.Atoi(m[1])
		}
		return b, nil
	}
	return nil, nil
}

// requiredVersion returns the version a require directive in go.mod gives path, or "". Lines in exclude, replace, and
// retract directives are ignored.
func requiredVersion(gomod, path string) string {
	block := "" // the directive of the enclosing ( ... ) block
	for line := range strings.Lines(gomod) {
		line, _, _ = strings.Cut(line, "//")
		f := strings.Fields(line)
		switch {
		case len(f) == 0:
			continue
		case block != "":
			if f[0] == ")" {
				block = ""
				continue
			}
		case len(f) == 2 && f[1] == "(":
			block = f[0]
			continue
		default:
			// A single-line directive: "require path version".
			if f[0] != "require" {
				continue
			}
			f = f[1:]
			block = ""
			if len(f) >= 2 && f[0] == path {
				return f[1]
			}
			continue
		}
		if block == "require" && len(f) >= 2 && f[0] == path {
			return f[1]
		}
	}
	return ""
}

// compareVersions compares vMAJOR.MINOR.PATCH versions. Pre-release and build suffixes aren't supported; ok is false
// for them.
func compareVersions(a, b string) (int, bool) {
	pa, ok1 := parseVersion(a)
	pb, ok2 := parseVersion(b)
	if !ok1 || !ok2 {
		return 0, false
	}
	return slices.Compare(pa, pb), true
}

func parseVersion(v string) ([]int, bool) {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 || !strings.HasPrefix(v, "v") {
		return nil, false
	}
	out := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

func (e *Enricher) model(ctx context.Context, it *awssdk.Item) (*smithy.Model, error) {
	id, err := e.Models.BlobID(ctx, it.Provenance.Head.SHA, it.Provenance.ModelPath)
	if err != nil {
		return nil, err
	}
	b, err := e.Models.Blob(ctx, id)
	if err != nil {
		return nil, err
	}
	return smithy.Parse(b)
}

// usedBy records, for each work entry, the provider files in the service package that call one of its operations.
func (e *Enricher) usedBy(ctx context.Context, pkg string, ws []Work) error {
	var ops []string
	for _, w := range ws {
		ops = append(ops, w.Operations...)
	}
	slices.Sort(ops)
	ops = slices.Compact(ops)
	if len(ops) == 0 {
		return nil
	}
	if e.rgPath == "" {
		p, err := exec.LookPath("rg")
		if err != nil {
			return fmt.Errorf("ripgrep (rg) not found")
		}
		e.rgPath = p
	}
	dir := filepath.Join(e.Checkout, "internal", "service", pkg)
	// Matches SDK calls such as conn.CreateWidget(ctx, ...) and paginator constructors, in non-test files.
	pattern := `\.(` + strings.Join(ops, "|") + `)\(|New(` + strings.Join(ops, "|") + `)Paginator\(`
	cmd := exec.CommandContext(ctx, e.rgPath, "--no-heading", "--with-filename", "--no-line-number", "--only-matching", "--glob", "*.go", "--glob", "!*_test.go", "-e", pattern, dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("rg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	files := map[string][]string{} // op -> files
	for line := range strings.Lines(string(out)) {
		// "<path>:<match>"
		path, match, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		op := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(match, "("), "."), "Paginator")
		op = strings.TrimPrefix(op, "New")
		if !slices.Contains(ops, op) {
			continue
		}
		rel, err := filepath.Rel(e.Checkout, path)
		if err != nil {
			rel = path
		}
		if !slices.Contains(files[op], rel) {
			files[op] = append(files[op], rel)
		}
	}
	for i := range ws {
		for _, op := range ws[i].Operations {
			for _, f := range files[op] {
				if !slices.Contains(ws[i].UsedBy, f) {
					ws[i].UsedBy = append(ws[i].UsedBy, f)
				}
			}
		}
		slices.Sort(ws[i].UsedBy)
	}
	return nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
