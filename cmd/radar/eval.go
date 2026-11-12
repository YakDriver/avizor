package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/YakDriver/radar"
	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/enrich"
	"github.com/YakDriver/radar/internal/eval"
	"github.com/YakDriver/radar/internal/reconcile"
)

func cmdEval(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	corpusPath := fs.String("corpus", "", "corpus file (default: the corpus built into radar, "+radar.DefaultCorpusName+")")
	fetch := fs.Bool("fetch", false, "fetch URL inputs to read their page titles (results then depend on the network)")
	failures := fs.Bool("failures", false, "in text output, list only failing cases")
	unjudged := fs.Bool("unjudged", false, "in text output, list confident candidates the corpus doesn't label, for adding same or different labels")
	names, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	var corpus *eval.Corpus
	name := *corpusPath
	if name == "" {
		name = "built-in:" + radar.DefaultCorpusName
		corpus, err = eval.Parse(radar.DefaultCorpusName, radar.DefaultCorpus)
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
	}
	c.operations(ctx, t, base, stderr)
	if *fetch {
		base.HTTP = defaultHTTP()
	}
	rep, err := (&eval.Runner{Base: base, Items: evalItems{c: &c, t: t, stderr: stderr}}).Run(ctx, corpus, name)
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

	fmt.Fprintf(w, "\n%-18s %5s %5s %6s %6s %6s %6s %8s %7s %9s %5s %8s\n", "kind", "cases", "pass", "hit@1", "hit@3", "hit@5",
		"recall", "category", "unmatch", "precision", "wrong", "unjudged")
	row := func(name string, m *eval.Metrics) {
		fmt.Fprintf(w, "%-18s %5d %5d %6s %6s %6s %6s %8s %7s %9s %5d %8d\n", name, m.Cases, m.Passed,
			pct(m.Hit[1], m.WithExpect), pct(m.Hit[3], m.WithExpect), pct(m.Hit[5], m.WithExpect),
			pct(m.Recall, m.WithExpect), pct(m.CategoryAccuracy, m.CategoryLabeled),
			pct(m.UnmatchedAccuracy, m.UnmatchedLabeled), pct(m.ConfidentPrecision, m.ConfidentCorrect+m.ConfidentWrong),
			m.ConfidentWrong, m.ConfidentUnjudged)
	}
	for _, k := range rep.Kinds() {
		row(k, rep.ByKind[k])
	}
	row("overall", &rep.Overall)
	if rep.Overall.Errors > 0 {
		fmt.Fprintf(w, "\n%d case(s) errored\n", rep.Overall.Errors)
	}
	if rep.Overall.MissedUnmatched > 0 {
		fmt.Fprintf(w, "%d case(s) with existing work were summarized as unmatched\n", rep.Overall.MissedUnmatched)
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

// evalItems resolves item cases from pulled items, enriched for the target without searching code.
type evalItems struct {
	c      *common
	t      *config.Target
	stderr io.Writer
}

func (e evalItems) Item(ctx context.Context, id string) (*reconcile.Item, error) {
	it, en, err := e.c.enrichedItem(ctx, e.t, id, true, e.stderr)
	if err != nil {
		return nil, err
	}
	return enrich.ReconcileItem(it, en), nil
}
