// Package draft renders a new issue for an SDK item from its evidence, enrichment, and reconciliation. The issue body
// holds only what the item shows. Inferred values (title, Terraform names, labels) and possible related issues are
// listed separately, with their basis, for the reviewer to confirm.
package draft

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"unicode"

	"github.com/YakDriver/radar/internal/awssdk"
	"github.com/YakDriver/radar/internal/enrich"
	"github.com/YakDriver/radar/internal/reconcile"
)

//go:embed default.md.tmpl
var defaultTemplate string

// Draft is a new issue to file, and what the reviewer should check.
type Draft struct {
	Item   string   `json:"item"`
	Repo   string   `json:"repo"`
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Body   string   `json:"body"`
	// GHArgs are `gh` arguments that create the issue, reading the body from standard input.
	GHArgs []string `json:"gh_args"`
	// Inferred lists every value in the draft that Radar guessed, with why.
	Inferred []Inferred `json:"inferred"`
	// Relates are related candidates the reviewer may want to link. They aren't in the body: they're weak matches.
	Relates []Relate `json:"relates"`
	// Warnings are reasons not to file the draft as is, such as existing work that may already cover the item.
	Warnings []string `json:"warnings"`
}

// Inferred is one guessed value.
type Inferred struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Basis string `json:"basis"`
}

// Relate is a candidate the reviewer may link with "Relates".
type Relate struct {
	Number int    `json:"number"`
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Basis  string `json:"basis"`
}

// Data is what the body template sees.
type Data struct {
	Item *awssdk.Item
	// Notes are the item's release notes, in full, without the "New AWS service client module" marker.
	Notes []string
	// Headline is the first sentence of the first note. Titles use it.
	Headline string
	// Release is the release name, such as "Release 2025-12-02".
	Release    string
	ReleaseURL string
	// BumpPRURL is the target PR that brought the SDK version in, if it's merged.
	BumpPRURL string
	Service   enrich.ServiceMatch
	Work      []enrich.Work
}

// Options configure rendering.
type Options struct {
	// Repo is the target repository, "owner/name".
	Repo string
	// Checkout is the target clone, read for resource annotations. Optional.
	Checkout string
	// Prefix is the Terraform type prefix, such as "aws_".
	Prefix string
	// LabelPrefix is the target's service label prefix, such as "service/".
	LabelPrefix string
	// Template is a body template file. Empty uses the built-in one.
	Template string
	// RelatesLimit caps suggested related candidates.
	RelatesLimit int
}

// New renders a draft.
func New(it *awssdk.Item, en *enrich.Result, rec *reconcile.Result, opts Options) (*Draft, error) {
	tmplText := defaultTemplate
	if opts.Template != "" {
		b, err := os.ReadFile(opts.Template)
		if err != nil {
			return nil, fmt.Errorf("reading draft template: %w", err)
		}
		tmplText = string(b)
	}
	tmpl, err := template.New("draft").Option("missingkey=error").Parse(tmplText)
	if err != nil {
		return nil, fmt.Errorf("parsing draft template: %w", err)
	}

	d := &Draft{Item: it.ID, Repo: opts.Repo, Inferred: []Inferred{}, Relates: []Relate{}, Warnings: []string{}}
	infer := func(field, value, basis string) { d.Inferred = append(d.Inferred, Inferred{field, value, basis}) }

	if it.Status != "new" {
		d.Warnings = append(d.Warnings, fmt.Sprintf("the item is %s: %s", it.Status, it.SkipReason))
	}

	data := Data{Item: it, Release: releaseName(it.Release.Tag), ReleaseURL: it.Release.CommitURL, Service: en.Service, Work: en.Work}
	if data.ReleaseURL == "" {
		data.ReleaseURL = it.Release.URL
	}
	for _, n := range it.Notes {
		if n.Text != NewModuleNote {
			data.Notes = append(data.Notes, n.Text)
		}
	}
	if len(data.Notes) > 0 {
		data.Headline = firstSentence(data.Notes[0])
		infer("headline", data.Headline, "first sentence of the release notes, used in the title; add the announcement headline to the body if there is one")
	}

	if av := en.Availability; av != nil {
		switch {
		case av.Status == enrich.Available && av.Bump != nil && av.Bump.PR != 0:
			data.BumpPRURL = fmt.Sprintf("https://github.com/%s/pull/%d", opts.Repo, av.Bump.PR)
		case av.Status == enrich.Pending:
			d.Warnings = append(d.Warnings, fmt.Sprintf("%s requires %s %s, before the item's %s; the change isn't available yet", opts.Repo, av.ModulePath, av.ProviderVersion, av.ItemVersion))
		}
	}

	d.Title, d.Labels = titleAndLabels(it, en, data.Headline, opts, infer)

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return nil, fmt.Errorf("rendering draft: %w", err)
	}
	d.Body = strings.TrimSpace(collapseBlank(body.String())) + "\n"

	for _, in := range d.Inferred {
		if in.Field == "terraform_name" {
			if p := docsPage(opts.Checkout, opts.Prefix, in.Value); p != "" {
				d.Warnings = append(d.Warnings, fmt.Sprintf("%s already exists: %s", in.Value, p))
			}
		}
	}
	if rec != nil {
		existingWork(rec, d)
		relates(rec, en, d, opts.RelatesLimit)
	}

	d.GHArgs = []string{"issue", "create", "--repo", opts.Repo, "--title", d.Title}
	for _, l := range d.Labels {
		d.GHArgs = append(d.GHArgs, "--label", l)
	}
	d.GHArgs = append(d.GHArgs, "--body-file", "-")
	return d, nil
}

const NewModuleNote = "New AWS service client module"

// titleAndLabels infers the title and labels from the work.
func titleAndLabels(it *awssdk.Item, en *enrich.Result, headline string, opts Options, infer func(field, value, basis string)) (string, []string) {
	labels := []string{"enhancement"}
	addLabel := func(l, basis string) {
		if !slices.Contains(labels, l) {
			labels = append(labels, l)
			infer("label", l, basis)
		}
	}
	if slices.ContainsFunc(it.Notes, func(n awssdk.Note) bool { return n.Text == NewModuleNote }) {
		addLabel("new-service", "the release notes say \""+NewModuleNote+"\"")
	}
	pkg := en.Service.Package
	switch en.Service.Status {
	case enrich.ServiceImplemented:
		addLabel(en.Service.Label, en.Service.Basis)
	default:
		// A service the provider doesn't implement yet; its package would most likely be named after the module.
		pkg = it.Service.Module
		addLabel("new-service", "the target has no implemented service for module "+it.Service.Module)
		addLabel(cmp(opts.LabelPrefix, "service/")+pkg, "the new service's package would most likely be named after its SDK module")
	}

	var newTypes, newDataSources, argTypes []string
	for _, w := range en.Work {
		switch w.Kind {
		case enrich.WorkNewResource:
			addLabel("new-resource", w.Basis)
			newTypes = append(newTypes, opts.Prefix+pkg+"_"+snake(w.Subject))
		case enrich.WorkNewDataSource:
			// Not labeled: new read operations are usually statuses or sub-resource listings, not data sources.
			newDataSources = append(newDataSources, opts.Prefix+pkg+"_"+snake(w.Subject))
		case enrich.WorkNewArgument:
			for _, n := range resourceNames(opts.Checkout, w.UsedBy) {
				if !slices.Contains(argTypes, n) {
					argTypes = append(argTypes, n)
				}
			}
		}
	}

	subject := strings.TrimSuffix(headline, ".")
	service := cmp(it.Service.SDKID, en.Service.Package, it.Service.Module)
	switch {
	case len(newTypes) == 1:
		title := fmt.Sprintf("New resource: `%s`", newTypes[0])
		infer("title", title, "one new resource inferred from the new operations")
		infer("terraform_name", newTypes[0], "provider prefix, service package, and the new operations' noun in snake case")
		return title, labels
	case len(newTypes) > 1:
		for _, n := range newTypes {
			infer("terraform_name", n, "provider prefix, service package, and the new operations' noun in snake case")
		}
	case len(newDataSources) == 1:
		title := fmt.Sprintf("New data source: `%s`", newDataSources[0])
		infer("title", title, "one new data source inferred from new read-only operations")
		infer("terraform_name", newDataSources[0], "provider prefix, service package, and the new operations' noun in snake case")
		return title, labels
	case len(argTypes) == 1 && subject != "":
		title := fmt.Sprintf("r/%s: %s", argTypes[0], subject)
		infer("title", title, "new input members of operations called only by "+argTypes[0])
		return title, labels
	}
	title := subject
	if title == "" {
		title = fmt.Sprintf("%s changes in %s", it.Service.SDKID, releaseName(it.Release.Tag))
	} else if !strings.Contains(strings.ToLower(title), strings.ToLower(service)) {
		title = service + ": " + title
	}
	infer("title", title, "release notes; no single resource is affected")
	return title, labels
}

func cmp(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

var annotationRE = regexp.MustCompile(`(?m)^// @(?:SDK|Framework)Resource\("([a-z0-9_]+)"`)

// resourceNames returns the Terraform resource types registered in files of the checkout.
func resourceNames(checkout string, files []string) []string {
	var out []string
	if checkout == "" {
		return out
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(checkout, f))
		if err != nil {
			continue
		}
		for _, m := range annotationRE.FindAllSubmatch(b, -1) {
			if n := string(m[1]); !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	slices.Sort(out)
	return out
}

// docsPage returns the checkout's docs page for a Terraform type, relative to the checkout, or "".
func docsPage(checkout, prefix, name string) string {
	if checkout == "" {
		return ""
	}
	short := strings.TrimPrefix(name, prefix)
	for _, dir := range []string{"r", "d"} {
		rel := filepath.Join("website", "docs", dir, short+".html.markdown")
		if _, err := os.Stat(filepath.Join(checkout, rel)); err == nil {
			return rel
		}
	}
	return ""
}

// existingWork warns about confident candidates: work that may already cover the item.
func existingWork(rec *reconcile.Result, d *Draft) {
	for _, c := range rec.Candidates {
		if c.Category == reconcile.CategoryRelated {
			continue
		}
		ref := c.Path
		if c.Number != 0 {
			ref = fmt.Sprintf("#%d %s", c.Number, c.Title)
		}
		d.Warnings = append(d.Warnings, fmt.Sprintf("%s: %s", strings.ReplaceAll(c.Category, "_", " "), ref))
	}
}

// relates suggests related issues and PRs in the item's service that share title keywords. The SDK bump PR is skipped,
// since every item in its release shares it. Bug reports are skipped: they share words like a service's resource
// names without being about a capability.
func relates(rec *reconcile.Result, en *enrich.Result, d *Draft, limit int) {
	if limit <= 0 {
		limit = 3
	}
	for _, c := range rec.Candidates {
		if len(d.Relates) == limit {
			break
		}
		if c.Category != reconcile.CategoryRelated || c.Number == 0 || isBug(c) || isBump(c) {
			continue
		}
		var label bool
		var kws []string
		for _, m := range c.Signals {
			switch m.Type {
			case "label":
				label = label || m.Value == en.Service.Label
			case "keyword":
				kws = append(kws, m.Value)
			}
		}
		if !label || len(kws) < 2 {
			continue
		}
		d.Relates = append(d.Relates, Relate{Number: c.Number, Kind: c.Kind, URL: c.URL, Title: c.Title,
			Basis: fmt.Sprintf("service label and title keywords %s", strings.Join(kws, ", "))})
	}
}

// isBump reports whether c is the SDK bump PR, which every item in a release shares.
func isBump(c reconcile.Candidate) bool {
	return slices.ContainsFunc(c.Signals, func(m reconcile.Match) bool { return m.Type == reconcile.MatchBump })
}

func isBug(c reconcile.Candidate) bool {
	return slices.Contains(c.Labels, "bug") || strings.HasPrefix(strings.ToLower(c.Title), "[bug]")
}

// releaseName turns "release-2025-12-02" into "Release 2025-12-02".
func releaseName(tag string) string {
	if rest, ok := strings.CutPrefix(tag, "release-"); ok {
		return "Release " + rest
	}
	return tag
}

var sentenceEndRE = regexp.MustCompile(`[.!?](\s|$)`)

// firstSentence returns text up to the end of its first sentence.
func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	if loc := sentenceEndRE.FindStringIndex(text); loc != nil {
		return strings.TrimSpace(text[:loc[0]+1])
	}
	return text
}

// snake turns a PascalCase noun into snake_case: S3AccessPoint -> s3_access_point, KMSKey -> kms_key.
func snake(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		if unicode.IsUpper(r) && i > 0 {
			prevLower := unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1])
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if prevLower || (unicode.IsUpper(rs[i-1]) && nextLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

var blankRunRE = regexp.MustCompile(`\n{3,}`)

func collapseBlank(s string) string {
	return blankRunRE.ReplaceAllString(s, "\n\n")
}
