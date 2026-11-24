package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/YakDriver/avizor"
	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/draft"
	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/eval"
	"github.com/YakDriver/avizor/internal/gitcmd"
	"github.com/YakDriver/avizor/internal/queue"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func cmdEval(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	corpusPath := fs.String("corpus", "", "corpus file (default: the corpus built into avizor, "+avizor.DefaultCorpusName+")")
	fetch := fs.Bool("fetch", false, "fetch URL inputs to read their page titles (results then depend on the network)")
	failures := fs.Bool("failures", false, "in text output, list only failing cases")
	unjudged := fs.Bool("unjudged", false, "in text output, list confident candidates the corpus doesn't label, for adding same or different labels")
	jobs := fs.Int("jobs", 4, "cases to run at once")
	names, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	var corpus *eval.Corpus
	name := *corpusPath
	if name == "" {
		name = "built-in:" + avizor.DefaultCorpusName
		corpus, err = eval.Parse(avizor.DefaultCorpusName, avizor.DefaultCorpus)
	} else {
		corpus, err = eval.Load(name)
	}
	if err != nil {
		return err
	}
	if c.target == "" {
		c.target = corpus.Target
	}
	if corpus, err = corpus.Filter(names); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	t, st, err := c.open()
	if err != nil {
		return err
	}
	defer st.Close()

	// Eval runs offline: issues not in the index are errors, not fetched, so results depend only on the index.
	base := &reconcile.Reconciler{
		Store:    st,
		Target:   t,
		Services: loadServices(t, stderr),
		Options:  reconcile.Options{NoFetch: !*fetch, Version: version},
		Git:      &gitcmd.Cache{},
	}
	c.operations(ctx, t, base, stderr)
	if *fetch {
		base.HTTP = defaultHTTP()
	}
	items := &evalItems{c: &c, r: base, stderr: stderr, enriched: map[string]*evalItem{}, posts: map[string]*evalPost{}}
	rep, err := (&eval.Runner{Base: base, Items: items, Jobs: *jobs}).Run(ctx, corpus, name)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.format == "text" {
		if *unjudged {
			return writeUnjudged(stdout, rep)
		}
		return writeEvalText(stdout, rep, *failures)
	}
	return writeJSON(stdout, rep)
}

func writeEvalText(w io.Writer, rep *eval.Report, failuresOnly bool) error {
	for _, cr := range rep.Cases {
		if failuresOnly && cr.Pass {
			continue
		}
		status := "PASS"
		if !cr.Pass {
			status = "FAIL"
		}
		fmt.Fprintf(w, "%s  %-40s %-12s %s\n", status, cr.Name, cr.Kind, cr.Input)
		if cr.Pass {
			continue
		}
		for _, f := range cr.Failures {
			fmt.Fprintf(w, "      - %s\n", f)
		}
		var top []string
		for _, r := range cr.Top[:min(5, len(cr.Top))] {
			top = append(top, fmt.Sprintf("%d. %s (%s)", r.Rank, r.Ref, shortCategory(r.Category)))
		}
		if len(top) > 0 {
			fmt.Fprintf(w, "      top: %s\n", strings.Join(top, ", "))
		}
	}

	fmt.Fprintf(w, "\n%-18s %5s %5s %6s %6s %6s %6s %8s %7s %6s %9s %5s %8s\n", "kind", "cases", "pass", "hit@1", "hit@3", "hit@5",
		"recall", "category", "unmatch", "state", "precision", "wrong", "unjudged")
	row := func(name string, m *eval.Metrics) {
		fmt.Fprintf(w, "%-18s %5d %5d %6s %6s %6s %6s %8s %7s %6s %9s %5d %8d\n", name, m.Cases, m.Passed,
			pct(m.Hit[1], m.WithExpect), pct(m.Hit[3], m.WithExpect), pct(m.Hit[5], m.WithExpect),
			pct(m.Recall, m.WithExpect), pct(m.CategoryAccuracy, m.CategoryLabeled),
			pct(m.UnmatchedAccuracy, m.UnmatchedLabeled), pct(m.StateAccuracy, m.StateLabeled), pct(m.ConfidentPrecision, m.ConfidentCorrect+m.ConfidentWrong),
			m.ConfidentWrong, m.ConfidentUnjudged)
	}
	for _, k := range rep.Kinds() {
		row(k, rep.ByKind[k])
	}
	row("overall", &rep.Overall)
	if rep.Overall.Errors > 0 {
		fmt.Fprintf(w, "\n%d case(s) errored\n", rep.Overall.Errors)
	}
	if m := rep.Overall; m.WorkPosts+m.NoWorkPosts > 0 {
		fmt.Fprintf(w, "posts: %d of %d with work significant (recall %s), %d of %d without work significant; precision %s\n",
			m.WorkSignificant, m.WorkPosts, pct(m.SignificanceRecall, m.WorkPosts), m.NoWorkSignificant, m.NoWorkPosts,
			pct(m.SignificancePrecision, m.WorkSignificant+m.NoWorkSignificant))
	}
	if m := rep.Overall; m.ItemsLabeled > 0 {
		fmt.Fprintf(w, "post items: %d of %d posts list an expected item (%s), %d first (%s)\n", m.ItemsListed, m.ItemsLabeled,
			pct(float64(m.ItemsListed)/float64(m.ItemsLabeled), m.ItemsLabeled), m.ItemsFirst, pct(float64(m.ItemsFirst)/float64(m.ItemsLabeled), m.ItemsLabeled))
	}
	if rep.Overall.MissedUnmatched > 0 {
		fmt.Fprintf(w, "%d case(s) with existing work were summarized as unmatched\n", rep.Overall.MissedUnmatched)
	}
	if ix := rep.Index; ix != nil {
		fmt.Fprintf(w, "index: synced %s, %d issues, %d PRs, %d signals, %d comments with signals\n",
			cmp.Or(ix.SyncedAt, "never"), ix.Issues, ix.PRs, ix.Signals, ix.Comments)
	}
	return nil
}

func writeUnjudged(w io.Writer, rep *eval.Report) error {
	n := 0
	var errored []string
	for _, cr := range rep.Cases {
		if cr.Error != "" {
			// An errored case returned no candidates, so none of them could be judged.
			errored = append(errored, fmt.Sprintf("  %s: %s", cr.Name, cr.Error))
			continue
		}
		var lines []string
		for _, j := range cr.Confident {
			if j.Verdict == eval.VerdictUnjudged {
				lines = append(lines, fmt.Sprintf("  %3d. %-8s %-22s %s", j.Rank, j.Ref, j.Category, j.Title))
			}
		}
		if len(lines) == 0 {
			continue
		}
		n += len(lines)
		fmt.Fprintf(w, "%s  %s\n%s\n", cr.Name, cr.Input, strings.Join(lines, "\n"))
	}
	fmt.Fprintf(w, "\n%d unjudged confident candidate(s)\n", n)
	if len(errored) > 0 {
		fmt.Fprintf(w, "\n%d case(s) errored, so their candidates weren't checked:\n%s\n", len(errored), strings.Join(errored, "\n"))
	}
	return nil
}

func pct(v float64, n int) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", v*100)
}

func shortCategory(c string) string {
	return strings.TrimPrefix(strings.TrimPrefix(c, "already_"), "previously_")
}

// evalItems resolves item cases from pulled items, enriched for the target, and classifies them for the queue.
// Enrichment searches the checkout only for cases with code = true. It's safe for concurrent use, and each item is
// enriched once.
type evalItems struct {
	c *common
	// r supplies the target, service map, and git cache.
	r      *reconcile.Reconciler
	stderr io.Writer

	mu       sync.Mutex
	enriched map[string]*evalItem
	posts    map[string]*evalPost

	// idx ranks items for post cases, loaded once with every pulled item.
	idxOnce sync.Once
	idx     *enrich.ItemIndex
	idxErr  error
}

// evalPost is a blog post and its matched announcements, loaded once.
type evalPost struct {
	once sync.Once
	p    *whatsnew.Post
	anns []PostAnnouncement
	err  error
}

// evalItem is an item enriched once, by the first case that needs it.
type evalItem struct {
	once sync.Once
	it   *awssdk.Item
	en   *enrich.Result
	err  error
}

func (e *evalItems) enrich(ctx context.Context, id string, code bool) (*awssdk.Item, *enrich.Result, error) {
	key := fmt.Sprintf("%s %t", id, code)
	e.mu.Lock()
	ei := e.enriched[key]
	if ei == nil {
		ei = &evalItem{}
		e.enriched[key] = ei
	}
	e.mu.Unlock()
	ei.once.Do(func() {
		ei.it, ei.en, ei.err = e.c.enrichedItem(ctx, e.r, id, !code, e.stderr)
	})
	return ei.it, ei.en, ei.err
}

func (e *evalItems) Item(ctx context.Context, id string, code bool) (*reconcile.Item, error) {
	it, en, err := e.enrich(ctx, id, code)
	if err != nil {
		return nil, err
	}
	return enrich.ReconcileItem(it, en), nil
}

func (e *evalItems) State(ctx context.Context, id string, code bool, rec *reconcile.Result, texts map[int]string) (string, string, error) {
	it, en, err := e.enrich(ctx, id, code)
	if err != nil {
		return "", "", err
	}
	t := e.r.Target
	d, err := draft.New(it, en, rec, draft.Options{Repo: t.Repo, Checkout: t.Checkout, Prefix: t.TypePrefix, LabelPrefix: t.ServiceLabelPrefix})
	if err != nil {
		return "", "", err
	}
	q := queue.Classify(it, en, rec, d, texts)
	return q.State, q.Reason, nil
}

// post loads a blog post and matches its announcements, as posts does.
func (e *evalItems) post(ctx context.Context, url string) (*evalPost, error) {
	e.mu.Lock()
	ep := e.posts[url]
	if ep == nil {
		ep = &evalPost{}
		e.posts[url] = ep
	}
	e.mu.Unlock()
	ep.once.Do(func() {
		ist, err := openItems(e.c.dir)
		if err != nil {
			ep.err = err
			return
		}
		defer ist.Close()
		if ep.p, ep.err = ist.Post(ctx, url); ep.err == nil && ep.p == nil {
			ep.err = fmt.Errorf("no blog post %q; pull them with `avizor pull aws-blogs`", url)
		}
		if ep.err == nil {
			ep.anns = postAnnouncements(announcements(e.c.dir, e.stderr), *ep.p)
		}
	})
	return ep, ep.err
}

// PostText is what a post is reconciled from, and the operations new for it, as posts does.
func (e *evalItems) PostText(ctx context.Context, url string) (string, reconcile.OperationSet, error) {
	ep, err := e.post(ctx, url)
	if err != nil {
		return "", nil, err
	}
	idx, err := e.itemIndex(ctx)
	if err != nil {
		return "", nil, err
	}
	return postText(*ep.p, ep.anns), idx.NewOperations(*ep.p), nil
}

// itemIndex loads every pulled item once, for post cases.
func (e *evalItems) itemIndex(ctx context.Context) (*enrich.ItemIndex, error) {
	e.idxOnce.Do(func() {
		ist, err := openItems(e.c.dir)
		if err != nil {
			e.idxErr = err
			return
		}
		defer ist.Close()
		e.idx, e.idxErr = loadItemIndex(ctx, ist, announcements(e.c.dir, e.stderr))
	})
	return e.idx, e.idxErr
}

// PostState classifies a reconciled post as posts does, without a decision.
func (e *evalItems) PostState(ctx context.Context, url string, res *reconcile.Result) (eval.PostState, error) {
	ep, err := e.post(ctx, url)
	if err != nil {
		return eval.PostState{}, err
	}
	pe := postEntry(*ep.p, ep.anns, res, nil, 0, time.Now())
	idx, err := e.itemIndex(ctx)
	if err != nil {
		return eval.PostState{}, err
	}
	var ids []string
	for _, it := range rankPostItems(idx, *ep.p, ep.anns) {
		ids = append(ids, it.ID)
	}
	return eval.PostState{State: pe.State, Numbers: pe.Numbers, Significant: pe.Significant, Signals: pe.Signals, Items: ids}, nil
}

// BlogPosts ranks the blog posts near an item's release, as brief does.
func (e *evalItems) BlogPosts(ctx context.Context, id string) ([]eval.Returned, error) {
	it, en, err := e.enrich(ctx, id, false)
	if err != nil {
		return nil, err
	}
	posts := blogPosts(e.c.dir, e.stderr)
	if posts == nil {
		return nil, errors.New("no blog posts loaded; run `avizor pull aws-blogs --since 365d`")
	}
	var out []eval.Returned
	for i, p := range enrich.BlogPosts(posts, it, en.Announcement, 0) {
		out = append(out, eval.Returned{Rank: i + 1, Ref: p.URL, Title: p.Title})
	}
	return out, nil
}
