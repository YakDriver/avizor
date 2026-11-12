package eval

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/YakDriver/radar/internal/reconcile"
)

// Ks are the cutoffs reported for hit@k.
var Ks = []int{1, 3, 5}

// passK is the cutoff a case's expected candidates must fall within to pass.
const passK = 5

// Runner evaluates cases with a reconciler. Each case runs on a copy of Base with AsOf and NoCode set from the case.
type Runner struct {
	Base *reconcile.Reconciler
}

// CaseResult is the outcome of one case.
type CaseResult struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Input string `json:"input"`
	AsOf  string `json:"as_of,omitempty"`
	Pass  bool   `json:"pass"`
	// Error is set when the case couldn't be run; such cases fail.
	Error string `json:"error,omitempty"`

	Expected []ExpectResult `json:"expected"`
	// Hit maps k to whether any expected candidate is in the top k. Only set for cases with expectations.
	Hit map[int]bool `json:"hit,omitempty"`
	// Confident lists every candidate returned above "related", at any rank, with its verdict.
	Confident []Judged `json:"confident"`
	// Unmatched is the result's summary.unmatched; UnmatchedOK is set when the case has an expectation for it.
	Unmatched   bool  `json:"unmatched"`
	UnmatchedOK *bool `json:"unmatched_ok,omitempty"`
	// Top lists the first candidates returned, for inspecting failures.
	Top      []Returned `json:"top"`
	Failures []string   `json:"failures,omitempty"`
}

// ExpectResult is how one expected candidate fared.
type ExpectResult struct {
	Ref      string `json:"ref"`
	Want     string `json:"want_category,omitempty"`
	Rank     int    `json:"rank,omitempty"`
	Got      string `json:"got_category,omitempty"`
	Found    bool   `json:"found"`
	Category *bool  `json:"category_ok,omitempty"`
}

// Verdicts on confident candidates.
const (
	VerdictExpected  = "expected"
	VerdictSame      = "same"
	VerdictDifferent = "different"
	VerdictUnjudged  = "unjudged"
)

// Judged is a confident candidate and the corpus's verdict on it.
type Judged struct {
	Returned
	Verdict string `json:"verdict"`
}

// Returned is a candidate as returned, with its 1-based rank.
type Returned struct {
	Rank     int    `json:"rank"`
	Ref      string `json:"ref"`
	Category string `json:"category"`
	Title    string `json:"title,omitempty"`
}

// Metrics aggregate case results.
type Metrics struct {
	Cases  int `json:"cases"`
	Passed int `json:"passed"`
	Errors int `json:"errors"`

	// WithExpect is the number of cases with expected candidates; Hit and Recall are over them.
	WithExpect int `json:"cases_with_expect"`
	// Hit maps k to the fraction of those cases with an expected candidate in the top k.
	Hit map[int]float64 `json:"hit_at"`
	// Recall is the fraction of all expected candidates returned at any rank.
	Recall float64 `json:"recall"`
	// CategoryAccuracy is the fraction of expected candidates with a category label that were found with that category.
	CategoryLabeled  int     `json:"category_labeled"`
	CategoryAccuracy float64 `json:"category_accuracy"`
	// MissedUnmatched counts cases with expected non-related candidates whose result was summarized as unmatched.
	MissedUnmatched int `json:"missed_as_unmatched"`
	// UnmatchedLabeled and UnmatchedAccuracy cover cases that label summary.unmatched.
	UnmatchedLabeled  int     `json:"unmatched_labeled"`
	UnmatchedAccuracy float64 `json:"unmatched_accuracy"`
	// Confident counts candidates returned above "related". Each is correct (expected or labeled the same work),
	// wrong (labeled different work), or unjudged. Precision is correct / (correct + wrong): how often a confident
	// category is on the right work.
	Confident          int     `json:"confident"`
	ConfidentCorrect   int     `json:"confident_correct"`
	ConfidentWrong     int     `json:"confident_wrong"`
	ConfidentUnjudged  int     `json:"confident_unjudged"`
	ConfidentPrecision float64 `json:"confident_precision"`
}

// Report is the result of a run.
type Report struct {
	Target   string              `json:"target"`
	Corpus   string              `json:"corpus"`
	RunAt    time.Time           `json:"run_at"`
	Overall  Metrics             `json:"overall"`
	ByKind   map[string]*Metrics `json:"by_kind"`
	Cases    []CaseResult        `json:"cases"`
	Duration string              `json:"duration"`
}

// Run binds the corpus to the base reconciler's target repo and evaluates every case, in order.
func (r *Runner) Run(ctx context.Context, c *Corpus, corpusPath string) (*Report, error) {
	if err := c.Bind(r.Base.Target.Repo); err != nil {
		return nil, err
	}
	start := time.Now()
	rep := &Report{Target: c.Target, Corpus: corpusPath, RunAt: start.UTC(), ByKind: map[string]*Metrics{}}
	for i := range c.Cases {
		if ctx.Err() != nil {
			break
		}
		rep.Cases = append(rep.Cases, r.runCase(ctx, &c.Cases[i]))
	}
	rep.Overall = aggregate(rep.Cases)
	kinds := map[string][]CaseResult{}
	for _, cr := range rep.Cases {
		kinds[cr.Kind] = append(kinds[cr.Kind], cr)
	}
	for k, crs := range kinds {
		m := aggregate(crs)
		rep.ByKind[k] = &m
	}
	rep.Duration = time.Since(start).Round(time.Millisecond).String()
	return rep, nil
}

func (r *Runner) runCase(ctx context.Context, cs *Case) CaseResult {
	cr := CaseResult{Name: cs.Name, Kind: cs.Kind, Input: cs.Input, Expected: []ExpectResult{}, Top: []Returned{}}
	if cs.Text != "" {
		cr.Input = "text: " + cs.Text
	}
	fail := func(err error) CaseResult {
		cr.Error = err.Error()
		cr.Failures = append(cr.Failures, "error: "+cr.Error)
		return cr
	}

	rc := *r.Base
	rc.Options.NoCode = !cs.Code
	rc.AsOf = cs.AsOfTime()
	if cs.AsOf == AsOfFiled {
		n := cs.issue
		is, err := r.Base.Store.Issue(ctx, n)
		if err != nil {
			return fail(err)
		}
		if is == nil {
			return fail(fmt.Errorf(`#%d is not in the index, so as_of = "filed" can't be resolved; sync further back`, n))
		}
		rc.AsOf = is.CreatedAt
	}
	if !rc.AsOf.IsZero() {
		cr.AsOf = rc.AsOf.UTC().Format(time.RFC3339)
	}

	res, err := rc.Run(ctx, cs.Input, cs.Text)
	if err != nil {
		return fail(err)
	}
	score(cs, res, &cr)
	return cr
}

func score(cs *Case, res *reconcile.Result, cr *CaseResult) {
	ranks := map[string]int{}
	cats := map[string]string{}
	for i, c := range res.Candidates {
		ref := candidateRef(c)
		if _, ok := ranks[ref]; ok {
			continue
		}
		ranks[ref] = i + 1
		cats[ref] = c.Category
		if i < 10 {
			cr.Top = append(cr.Top, Returned{Rank: i + 1, Ref: ref, Category: c.Category, Title: c.Title})
		}
	}

	best := 0
	for _, e := range cs.Expect {
		ref := expectRef(e)
		er := ExpectResult{Ref: ref, Want: e.Category}
		if rank, ok := ranks[ref]; ok {
			er.Found, er.Rank, er.Got = true, rank, cats[ref]
			if best == 0 || rank < best {
				best = rank
			}
			if rank > passK {
				cr.Failures = append(cr.Failures, fmt.Sprintf("%s ranked %d, beyond top %d", ref, rank, passK))
			}
		} else {
			cr.Failures = append(cr.Failures, ref+" not returned")
		}
		if e.Category != "" {
			ok := er.Found && er.Got == e.Category
			er.Category = &ok
			if er.Found && !ok {
				cr.Failures = append(cr.Failures, fmt.Sprintf("%s is %s, want %s", ref, er.Got, e.Category))
			}
		}
		cr.Expected = append(cr.Expected, er)
	}
	if len(cs.Expect) > 0 {
		cr.Hit = map[int]bool{}
		for _, k := range Ks {
			cr.Hit[k] = best > 0 && best <= k
		}
	}

	judge := cs.judgments()
	cr.Confident = []Judged{}
	for i, c := range res.Candidates {
		if c.Category == reconcile.CategoryRelated {
			continue
		}
		ref := candidateRef(c)
		if ranks[ref] != i+1 {
			continue // duplicate entry
		}
		v, ok := judge[ref]
		if !ok {
			v = VerdictUnjudged
		}
		cr.Confident = append(cr.Confident, Judged{Rank: i + 1, Ref: ref, Category: c.Category, Title: c.Title, Verdict: v})
		if v == VerdictDifferent {
			cr.Failures = append(cr.Failures, fmt.Sprintf("%s is %s, but is different work", ref, c.Category))
		}
	}

	cr.Unmatched = res.Summary.Unmatched
	if cs.Unmatched != nil {
		ok := res.Summary.Unmatched == *cs.Unmatched
		cr.UnmatchedOK = &ok
		if !ok {
			cr.Failures = append(cr.Failures, fmt.Sprintf("unmatched = %t, want %t", res.Summary.Unmatched, *cs.Unmatched))
		}
	}

	cr.Pass = len(cr.Failures) == 0
}

func aggregate(crs []CaseResult) Metrics {
	m := Metrics{Hit: map[int]float64{}}
	hits := map[int]int{}
	var expects, found, catOK, unmatchedOK int
	for _, cr := range crs {
		m.Cases++
		if cr.Pass {
			m.Passed++
		}
		if cr.Error != "" {
			m.Errors++
		}
		if len(cr.Expected) > 0 && cr.Error == "" {
			m.WithExpect++
			for _, k := range Ks {
				if cr.Hit[k] {
					hits[k]++
				}
			}
		}
		strongExpected := false
		for _, e := range cr.Expected {
			expects++
			if e.Found {
				found++
			}
			if e.Category != nil {
				m.CategoryLabeled++
				if *e.Category {
					catOK++
				}
			}
			if e.Want != "" && e.Want != reconcile.CategoryRelated {
				strongExpected = true
			}
		}
		if strongExpected && cr.Error == "" && cr.Unmatched {
			m.MissedUnmatched++
		}
		if cr.UnmatchedOK != nil {
			m.UnmatchedLabeled++
			if *cr.UnmatchedOK {
				unmatchedOK++
			}
		}
		for _, j := range cr.Confident {
			m.Confident++
			switch j.Verdict {
			case VerdictExpected, VerdictSame:
				m.ConfidentCorrect++
			case VerdictDifferent:
				m.ConfidentWrong++
			default:
				m.ConfidentUnjudged++
			}
		}
	}
	for _, k := range Ks {
		m.Hit[k] = ratio(hits[k], m.WithExpect)
	}
	m.Recall = ratio(found, expects)
	m.CategoryAccuracy = ratio(catOK, m.CategoryLabeled)
	m.UnmatchedAccuracy = ratio(unmatchedOK, m.UnmatchedLabeled)
	m.ConfidentPrecision = ratio(m.ConfidentCorrect, m.ConfidentCorrect+m.ConfidentWrong)
	return m
}

// Kinds returns the report's case kinds, sorted.
func (rep *Report) Kinds() []string {
	ks := make([]string, 0, len(rep.ByKind))
	for k := range rep.ByKind {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// judgments maps candidate refs to the case's verdict on them.
func (cs *Case) judgments() map[string]string {
	out := map[string]string{}
	for _, e := range cs.Expect {
		out[expectRef(e)] = VerdictExpected
	}
	for _, r := range numberRefs(cs.Same) {
		out[r] = VerdictSame
	}
	for _, r := range cs.SamePaths {
		out[r] = VerdictSame
	}
	for _, r := range numberRefs(cs.Different) {
		out[r] = VerdictDifferent
	}
	for _, r := range cs.DifferentPaths {
		out[r] = VerdictDifferent
	}
	return out
}

func candidateRef(c reconcile.Candidate) string {
	if c.Path != "" {
		return c.Path
	}
	return "#" + strconv.Itoa(c.Number)
}

func expectRef(e Expect) string {
	if e.Path != "" {
		return e.Path
	}
	return "#" + strconv.Itoa(e.Number)
}

// Filter returns a copy of the corpus with only the named cases, in corpus order.
func (c *Corpus) Filter(names []string) (*Corpus, error) {
	if len(names) == 0 {
		return c, nil
	}
	out := &Corpus{Target: c.Target}
	for _, cs := range c.Cases {
		if slices.Contains(names, cs.Name) {
			out.Cases = append(out.Cases, cs)
		}
	}
	for _, n := range names {
		if !slices.ContainsFunc(out.Cases, func(cs Case) bool { return cs.Name == n }) {
			return nil, fmt.Errorf("no case named %q", n)
		}
	}
	return out, nil
}
