// Package reconcile finds existing issues, pull requests, and code that may correspond to an input, and explains why
// each candidate matched.
package reconcile

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/index"
	"github.com/YakDriver/radar/internal/services"
	"github.com/YakDriver/radar/internal/signal"
)

// IssueFetcher fetches an issue that isn't in the index.
type IssueFetcher interface {
	FetchIssue(ctx context.Context, repo string, number int) (*index.Issue, error)
}

// Options tune retrieval.
type Options struct {
	// NoFetch skips fetching URL inputs to read their title.
	NoFetch bool
	// NoCode skips searching the checkout.
	NoCode bool
	// RelatedLimit caps the number of "related" candidates. Zero means the default.
	RelatedLimit int
	// CommonThreshold skips exact signals found in more records than this, since they don't discriminate. Zero means the
	// default.
	CommonThreshold int
	// Version is recorded in provenance.
	Version string
}

const (
	defaultRelatedLimit    = 10
	defaultCommonThreshold = 40
	searchLimit            = 40
	keywordLimit           = 10
)

// Reconciler matches inputs against a target's index and checkout.
type Reconciler struct {
	Store    *index.Store
	Services *services.Map
	Target   *config.Target
	Issues   IssueFetcher
	HTTP     *http.Client
	Options  Options
	// AsOf, if set, reconciles against the index as it was at that time. See index.Store.AsOf. Code search reflects
	// the checkout as it is now, so callers usually set Options.NoCode with it.
	AsOf time.Time

	// view is Store, or Store.AsOf(AsOf), for the current run.
	view     *index.Store
	warnings []string
	rgPath   string
}

func (r *Reconciler) warnf(format string, args ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

// Run reconciles arg (an issue number, issue URL, or other URL) or free text.
func (r *Reconciler) Run(ctx context.Context, arg, text string) (*Result, error) {
	r.warnings = nil
	r.view = r.Store
	if !r.AsOf.IsZero() {
		r.view = r.Store.AsOf(r.AsOf)
	}
	relatedLimit := cmp.Or(r.Options.RelatedLimit, defaultRelatedLimit)
	commonThreshold := cmp.Or(r.Options.CommonThreshold, defaultCommonThreshold)

	in, err := r.resolve(ctx, arg, text)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Input:      in.info,
		Evidence:   []Evidence{},
		Inferences: []Inference{},
		Candidates: []Candidate{},
		Provenance: Provenance{Tool: "radar", Version: r.Options.Version, GeneratedAt: time.Now().UTC()},
	}
	if err := r.fillTarget(ctx, &res.Target); err != nil {
		return nil, err
	}

	sigs := r.inputSignals(in, res)
	focus := map[signal.Signal]bool{}
	for _, sg := range signal.Extract(in.focusText, r.Target.Repo) {
		focus[sg] = true
	}
	pkgs := r.inferServices(in, sigs, focus, res)
	request := inferRequest(in.focusText, res)

	res.Input.Keywords = signal.Keywords(in.focusText, keywordLimit)
	if res.Input.Keywords == nil {
		res.Input.Keywords = []string{}
	}

	cands := map[int]*Candidate{}
	get := func(n int) *Candidate {
		c, ok := cands[n]
		if !ok {
			c = &Candidate{}
			cands[n] = c
		}
		return c
	}

	// Exact signals.
	var common []string
	for _, sg := range sigs {
		switch sg.Kind {
		case signal.KindURL, signal.KindOp, signal.KindTFName:
		case signal.KindIssueRef:
			if n, err := strconv.Atoi(sg.Value); err == nil && n != in.info.Number {
				get(n).addSignal(Match{Type: matchReferencedByInput, Value: "#" + sg.Value, Strength: StrengthWeak})
			}
			continue
		default:
			continue
		}
		count, err := r.view.SignalCount(ctx, sg.Kind, sg.Value)
		if err != nil {
			return nil, err
		}
		if count > commonThreshold {
			common = append(common, fmt.Sprintf("%s:%s (%d)", sg.Kind, sg.Value, count))
			markCommon(res.Evidence, sg)
			continue
		}
		hits, err := r.view.SignalHits(ctx, sg.Kind, sg.Value)
		if err != nil {
			return nil, err
		}
		for _, h := range hits {
			if h.Number == in.info.Number {
				continue
			}
			get(h.Number).addSignal(Match{Type: h.Kind, Value: h.Value, Where: h.Where, Strength: signalStrength(h, focus, request)})
		}
	}
	if len(common) > 0 {
		r.warnf("skipped signals found in more than %d records: %s", commonThreshold, strings.Join(common, ", "))
	}

	// Records that reference the input issue.
	if in.info.Number != 0 {
		hits, err := r.view.SignalHits(ctx, signal.KindIssueRef, strconv.Itoa(in.info.Number))
		if err != nil {
			return nil, err
		}
		if len(hits) <= commonThreshold {
			for _, h := range hits {
				get(h.Number).addSignal(Match{Type: matchReferencesInput, Value: "#" + strconv.Itoa(in.info.Number), Where: h.Where, Strength: StrengthWeak})
			}
		}
	}

	// Full-text retrieval on keywords.
	searchHits, err := r.view.Search(ctx, res.Input.Keywords, searchLimit)
	if err != nil {
		return nil, err
	}
	ftsRank := map[int]float64{}
	for _, h := range searchHits {
		if h.Number != in.info.Number {
			ftsRank[h.Number] = h.Rank
		}
	}

	// Load candidate records, including FTS hits, then annotate and filter.
	numbers := make([]int, 0, len(cands)+len(ftsRank))
	for n := range cands {
		numbers = append(numbers, n)
	}
	for n := range ftsRank {
		if _, ok := cands[n]; !ok {
			numbers = append(numbers, n)
		}
	}
	records, err := r.view.Issues(ctx, numbers)
	if err != nil {
		return nil, err
	}

	serviceLabels := map[string]bool{}
	for _, p := range pkgs {
		serviceLabels[r.Target.ServiceLabelPrefix+p] = true
	}

	var out []Candidate
	for _, n := range numbers {
		is := records[n]
		if is == nil {
			// Referenced but not indexed (e.g. a sliced sync). Keep cross-references so the reader can follow them.
			if c, ok := cands[n]; ok {
				c.Kind, c.Number, c.Category = KindIssue, n, CategoryRelated
				c.URL = fmt.Sprintf("https://github.com/%s/issues/%d", r.Target.Repo, n)
				c.Title = "(not in index)"
				out = append(out, *c)
			}
			continue
		}

		_, exact := cands[n]
		c := get(n)
		fillFromIssue(c, is)

		labelMatch := false
		for _, l := range is.Labels {
			if serviceLabels[l] {
				c.addSignal(Match{Type: "label", Value: l, Strength: StrengthWeak})
				labelMatch = true
			}
		}

		rank, fromFTS := ftsRank[n]
		if fromFTS {
			c.rank = rank
			kw := titleKeywords(is.Title, res.Input.Keywords)
			for _, k := range kw {
				c.addSignal(Match{Type: "keyword", Value: k, Where: index.WhereTitle, Strength: StrengthWeak})
			}
			// A full-text hit alone is kept only if its title shares enough keywords, or one keyword and the service.
			if !exact && !enoughKeywords(len(kw), len(res.Input.Keywords), labelMatch) {
				continue
			}
		}

		c.Category = categorize(c, is)
		out = append(out, *c)
	}

	if !r.Options.NoCode {
		var tfNames, ops []string
		for _, sg := range sigs {
			if sg.Kind == signal.KindTFName && focus[sg] {
				tfNames = append(tfNames, sg.Value)
			}
		}
		for _, sg := range sigs {
			if sg.Kind == signal.KindOp {
				ops = append(ops, sg.Value)
			}
		}
		code := r.searchCode(ctx, ops, tfNames, pkgs)
		// Existing code for a named type answers a request for that type, but is only context for a change to it.
		if request != RequestNewType {
			for i := range code {
				code[i].Category = CategoryRelated
			}
		}
		out = append(out, code...)
	}

	sortCandidates(out)
	res.Candidates = limitRelated(out, relatedLimit)
	res.Summary = summarize(res.Candidates)
	res.Warnings = r.warnings
	return res, nil
}

func (r *Reconciler) fillTarget(ctx context.Context, t *TargetInfo) error {
	t.Name = r.Target.Name
	t.Repo = r.Target.Repo
	t.Checkout = r.Target.Checkout
	synced, err := r.Store.Meta(ctx, "last_sync")
	if err != nil {
		return err
	}
	t.IndexSynced = synced
	if synced == "" {
		r.warnf("the index for %s has never been synced; run `radar sync`", r.Target.Name)
	}
	st, err := r.Store.Stats(ctx)
	if err != nil {
		return err
	}
	t.Issues, t.PRs = st.Issues, st.PRs
	if !r.AsOf.IsZero() {
		t.AsOf = r.AsOf.UTC().Format(time.RFC3339)
	}
	if t.Checkout != "" && !r.Options.NoCode {
		t.CheckoutRef = checkoutRef(ctx, t.Checkout)
	}
	return nil
}

// inputSignals collects distinct signals from the input and records them as evidence.
func (r *Reconciler) inputSignals(in *input, res *Result) []signal.Signal {
	var out []signal.Signal
	seen := map[signal.Signal]bool{}
	add := func(sg signal.Signal, source string) {
		if seen[sg] {
			return
		}
		seen[sg] = true
		out = append(out, sg)
		res.Evidence = append(res.Evidence, Evidence{Type: sg.Kind, Value: sg.Value, Source: source})
	}

	source := "input." + in.info.Kind
	switch in.info.Kind {
	case InputIssue:
		for _, sg := range signal.Extract(in.info.Title, r.Target.Repo) {
			add(sg, "input.title")
		}
		source = "input.body"
	case InputURL:
		if in.pageTitle != "" {
			res.Evidence = append(res.Evidence, Evidence{Type: "page_title", Value: in.pageTitle, Source: in.info.Ref})
		}
	}
	for _, sg := range signal.Extract(in.signalText, r.Target.Repo) {
		add(sg, source)
	}
	for _, h := range in.indexed {
		if h.Where == index.WhereComment {
			add(signal.Signal{Kind: h.Kind, Value: h.Value}, "input.comments")
		}
	}
	return out
}

// inferServices guesses which provider service packages the input concerns, recording each guess and its basis.
func (r *Reconciler) inferServices(in *input, sigs []signal.Signal, focus map[signal.Signal]bool, res *Result) []string {
	var pkgs []string
	add := func(pkg, basis string) {
		if slices.Contains(pkgs, pkg) {
			return
		}
		pkgs = append(pkgs, pkg)
		res.Inferences = append(res.Inferences, Inference{Type: "service", Value: pkg, Basis: basis})
	}

	for _, l := range in.info.Labels {
		if pkg, ok := strings.CutPrefix(l, r.Target.ServiceLabelPrefix); ok {
			add(pkg, "label "+l)
		}
	}
	if r.Services == nil {
		return pkgs
	}
	for _, sg := range sigs {
		if sg.Kind != signal.KindTFName || !focus[sg] {
			continue
		}
		if svc := r.Services.ByTFName(sg.Value); svc != nil {
			add(svc.Package, "terraform name "+sg.Value)
		}
	}
	for _, m := range r.Services.DetectInText(in.focusText) {
		add(m.Service.Package, fmt.Sprintf("text mentions %q", m.Name))
	}
	return pkgs
}

// signalStrength rates an exact match.
//
//   - A Terraform name is strong only for a request for a new type, and only if it's in the input's focus text (its
//     title) and the candidate's title. Bodies mention resource names as context constantly, and many issues and PRs
//     change an existing resource without being about the same change.
//   - GitHub URLs are weak. They are usually SDK release commits or other repositories' issues, which many unrelated
//     features share.
func signalStrength(h index.SignalHit, focus map[signal.Signal]bool, request string) string {
	switch h.Kind {
	case signal.KindTFName:
		if request != RequestNewType || h.Where != index.WhereTitle || !focus[signal.Signal{Kind: h.Kind, Value: h.Value}] {
			return StrengthWeak
		}
	case signal.KindURL:
		if strings.HasPrefix(h.Value, "https://github.com/") {
			return StrengthWeak
		}
	}
	return StrengthStrong
}

func markCommon(ev []Evidence, sg signal.Signal) {
	for i := range ev {
		if ev[i].Type == sg.Kind && ev[i].Value == sg.Value {
			ev[i].Common = true
		}
	}
}

func categorize(c *Candidate, is *index.Issue) string {
	if c.strongCount() == 0 {
		return CategoryRelated
	}
	switch {
	case is.Kind == index.KindPR && is.Merged:
		return CategoryImplemented
	case is.State == "open":
		return CategoryTracked
	default:
		return CategoryConsidered
	}
}

func fillFromIssue(c *Candidate, is *index.Issue) {
	c.Kind = KindIssue
	if is.Kind == index.KindPR {
		c.Kind = KindPR
	}
	c.Number = is.Number
	c.Title = is.Title
	c.State = is.State
	c.StateReason = is.StateReason
	c.Merged = is.Merged
	c.URL = is.URL
	c.Labels = is.Labels
	c.UpdatedAt = is.UpdatedAt
}

// titleKeywords returns the keywords present in title, comparing simple stems.
func titleKeywords(title string, keywords []string) []string {
	words := map[string]bool{}
	for _, w := range signal.Keywords(title, 0) {
		words[stem(w)] = true
	}
	var out []string
	for _, k := range keywords {
		if words[stem(k)] {
			out = append(out, k)
		}
	}
	return out
}

func stem(w string) string {
	for _, suf := range []string{"ies", "es", "s", "ing", "ed"} {
		if s, ok := strings.CutSuffix(w, suf); ok && len(s) >= 3 {
			if suf == "ies" {
				return s + "y"
			}
			return s
		}
	}
	return w
}

func enoughKeywords(matched, total int, labelMatch bool) bool {
	if labelMatch && matched >= 1 {
		return true
	}
	return matched >= min(2, total) && matched > 0
}

func sortCandidates(cs []Candidate) {
	order := map[string]int{}
	for i, c := range Categories {
		order[c] = i
	}
	slices.SortStableFunc(cs, func(a, b Candidate) int {
		if d := order[a.Category] - order[b.Category]; d != 0 {
			return d
		}
		if d := b.strongCount() - a.strongCount(); d != 0 {
			return d
		}
		if d := boolInt(b.crossReferenced()) - boolInt(a.crossReferenced()); d != 0 {
			return d
		}
		if d := len(b.Signals) - len(a.Signals); d != 0 {
			return d
		}
		if a.rank != b.rank {
			if a.rank < b.rank {
				return -1
			}
			return 1
		}
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
}

func limitRelated(cs []Candidate, limit int) []Candidate {
	out := make([]Candidate, 0, len(cs))
	n := 0
	for _, c := range cs {
		if c.Category == CategoryRelated {
			if n == limit {
				continue
			}
			n++
		}
		out = append(out, c)
	}
	return out
}

func summarize(cs []Candidate) Summary {
	s := Summary{Counts: map[string]int{}}
	for _, c := range Categories {
		s.Counts[c] = 0
	}
	for _, c := range cs {
		s.Counts[c.Category]++
	}
	s.Unmatched = s.Counts[CategoryImplemented]+s.Counts[CategoryTracked]+s.Counts[CategoryConsidered] == 0
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Request kinds inferred from the input's title or text.
const (
	RequestNewType = "new_type"
	RequestChange  = "change"
	RequestUnknown = "unknown"
)

var (
	newTypeRE = regexp.MustCompile(`(?i)\bnew (?:resource|data[ -]source|ephemeral resource|list resource|action)\b`)
	changeRE  = regexp.MustCompile(`(?i)\b(?:add|adds|adding|support|supports|argument|attribute|block|bug|fix|crash|panic|error|diff|drift|deprecat\w*|missing|update)\b`)
	// typeTitleRE matches the provider's "r/aws_x: ..." convention for a request about one type.
	typeTitleRE = regexp.MustCompile(`^(?:\[[^\]]*\]:?\s*)?(?:[a-z]+/)?aws_[a-z0-9_]+:`)
)

// inferRequest guesses whether the input asks for a new Terraform type or a change to an existing one.
func inferRequest(text string, res *Result) string {
	kind, basis := RequestUnknown, "no request wording recognized"
	switch {
	case newTypeRE.MatchString(text):
		kind, basis = RequestNewType, fmt.Sprintf("text says %q", newTypeRE.FindString(text))
	case changeRE.MatchString(text):
		kind, basis = RequestChange, fmt.Sprintf("text says %q", changeRE.FindString(text))
	case typeTitleRE.MatchString(text):
		kind, basis = RequestNewType, "title names one type without change wording"
	}
	res.Inferences = append(res.Inferences, Inference{Type: "request", Value: kind, Basis: basis})
	return kind
}
