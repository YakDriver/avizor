package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/YakDriver/radar/internal/awssdk"
	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/draft"
	"github.com/YakDriver/radar/internal/enrich"
	"github.com/YakDriver/radar/internal/index"
	"github.com/YakDriver/radar/internal/items"
	"github.com/YakDriver/radar/internal/queue"
	"github.com/YakDriver/radar/internal/reconcile"
	"github.com/YakDriver/radar/internal/services"
)

func cmdQueue(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("queue", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	sinceFlag := fs.String("since", "30d", "only releases published since a date, RFC 3339 time, or age (30d); empty for all")
	service := fs.String("service", "", "only this SDK service module (e.g. fsx)")
	source := fs.String("source", "", "only this source")
	all := fs.Bool("all", false, "also show items that are decided, covered, or probably need no work")
	states := fs.String("state", "", "only these states, comma-separated (overrides --all): "+strings.Join(queueStates, ", "))
	noCode := fs.Bool("no-code", false, "don't search the target checkout (no item is then no_code)")
	relates := fs.Int("relates", 3, "maximum related issues and PRs per item")
	jobs := fs.Int("jobs", 4, "items to process at once")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) > 0 {
		return fmt.Errorf("%w: queue takes no arguments", errUsage)
	}
	var want []string
	if *states != "" {
		for s := range strings.SplitSeq(*states, ",") {
			s = strings.TrimSpace(s)
			if !slices.Contains(queueStates, s) {
				return fmt.Errorf("%w: unknown state %q; states are %s", errUsage, s, strings.Join(queueStates, ", "))
			}
			want = append(want, s)
		}
	}
	f := items.Filter{Status: items.StatusNew, Service: *service, Source: *source}
	if *sinceFlag != "" {
		if f.Since, err = parseSince(*sinceFlag, time.Now()); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}
	*jobs = max(*jobs, 1)

	t, st, err := c.open()
	if err != nil {
		return err
	}
	defer st.Close()
	ist, err := openItems(c.dir)
	if err != nil {
		return err
	}
	recs, err := ist.List(ctx, f)
	if err != nil {
		ist.Close()
		return err
	}
	decisions, err := ist.Current(ctx, t.Name)
	ist.Close()
	if err != nil {
		return err
	}

	svcs := loadServices(t, stderr)
	// Without the service map, every item's service is unknown and would be hidden as unimplemented.
	switch {
	case t.Checkout == "":
		return fmt.Errorf("radar queue needs target %q's checkout for its service map; set checkout in the config", t.Name)
	case svcs == nil:
		return fmt.Errorf("radar queue needs the service map %s", t.NamesDataPath())
	}
	var ops reconcile.Reconciler
	c.operations(ctx, t, &ops, stderr)
	opts := draft.Options{Repo: t.Repo, Checkout: t.Checkout, Prefix: t.TypePrefix, LabelPrefix: t.ServiceLabelPrefix,
		Template: t.DraftTemplate, RelatesLimit: *relates}

	entries := make([]queue.Entry, len(recs))
	errs := make([]error, len(recs))
	next := make(chan int)
	var wg sync.WaitGroup
	for range *jobs {
		wg.Go(func() {
			// Each worker has its own enricher and reconciler: both cache state per run. The index store, service
			// map, and operation names are safe to share.
			r := &reconcile.Reconciler{Store: st, Target: t, Services: svcs, Operations: ops.Operations,
				OperationsSource: ops.OperationsSource, Options: reconcile.Options{NoFetch: true, NoCode: *noCode, Version: version}}
			enrichers := map[string]*enrich.Enricher{}
			for i := range next {
				it, err := decodeItem(recs[i])
				if err == nil {
					e := enrichers[it.Source]
					if e == nil {
						e = queueEnricher(c.cfg, c.dir, t, svcs, it.Source, *noCode)
						enrichers[it.Source] = e
					}
					entries[i], err = queueEntry(ctx, it, e, r, opts)
				}
				errs[i] = err
			}
		})
	}
	for i := range recs {
		next <- i
	}
	close(next)
	wg.Wait()

	now := time.Now()
	lookup := func(n int) *index.Issue {
		is, err := st.Issue(ctx, n)
		if err != nil {
			return nil
		}
		return is
	}
	counts := map[string]int{}
	var shown []queue.Entry
	for i, e := range entries {
		if errs[i] != nil {
			return fmt.Errorf("%s: %w", recs[i].ID, errs[i])
		}
		if d, ok := decisions[e.Item]; ok {
			queue.Resurface(&e, d, lookup, now)
		}
		g := e.Group()
		counts[g]++
		if (want == nil && (*all || queue.Shown(g))) || slices.Contains(want, g) {
			shown = append(shown, e)
		}
	}
	slices.SortStableFunc(shown, func(a, b queue.Entry) int {
		if c := strings.Compare(b.Published, a.Published); c != 0 {
			return c
		}
		return strings.Compare(a.Item, b.Item)
	})
	if shown == nil {
		shown = []queue.Entry{}
	}
	if c.format != "text" {
		return writeJSON(stdout, shown)
	}
	return writeQueueText(stdout, shown, counts, len(recs), *all || want != nil)
}

// queueStates are the queue states in the order they're reported.
var queueStates = []string{queue.Resurfaced, queue.Gap, queue.Pending, queue.NewService, queue.Decided, queue.Covered, queue.NoCode, queue.Unimplemented, queue.NoWork}

// queueEnricher is newEnricher with a shared service map, for many items from one source.
func queueEnricher(cfg *config.Config, dir string, t *config.Target, svcs *services.Map, source string, noCode bool) *enrich.Enricher {
	e := &enrich.Enricher{Target: t.Name, Services: svcs, LabelPrefix: t.ServiceLabelPrefix, Checkout: t.Checkout, Ref: t.Ref, NoCode: noCode}
	if src, err := cfg.Source(source); err == nil {
		if p := src.ModelsPath(dir); dirExists(p) {
			e.Models = &awssdk.ModelsRepo{Dir: p, URL: src.ModelsURL, Branch: src.ModelsBranch}
		}
	}
	return e
}

func queueEntry(ctx context.Context, it *awssdk.Item, e *enrich.Enricher, r *reconcile.Reconciler, opts draft.Options) (queue.Entry, error) {
	en := e.Enrich(ctx, it)
	rec, err := r.RunItem(ctx, enrich.ReconcileItem(it, en))
	if err != nil {
		return queue.Entry{}, err
	}
	d, err := draft.New(it, en, rec, opts)
	if err != nil {
		return queue.Entry{}, err
	}
	return queue.Classify(it, en, rec, d), nil
}

func writeQueueText(w io.Writer, shown []queue.Entry, counts map[string]int, total int, all bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, e := range shown {
		rel := make([]string, 0, len(e.Relates))
		for _, r := range e.Relates {
			rel = append(rel, fmt.Sprintf("#%d%s", r.Number, prTag(r.Kind)))
		}
		note := ""
		switch {
		case e.Decision != nil && e.Resurfaced != "":
			note = decisionText(*e.Decision) + "; resurfaced: " + e.Resurfaced
		case e.Decision != nil:
			note = decisionText(*e.Decision)
		case e.State == queue.Covered:
			note = e.Reason
		case len(rel) > 0:
			note = "related " + strings.Join(rel, ", ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Published, e.Item, e.Group(), workSummary(e.Work), truncate(e.Title, 80), note)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	var parts, hidden []string
	for _, s := range queueStates {
		if counts[s] == 0 {
			continue
		}
		p := fmt.Sprintf("%s %d", strings.ReplaceAll(s, "_", " "), counts[s])
		if queue.Shown(s) {
			parts = append(parts, p)
		} else {
			hidden = append(hidden, p)
		}
	}
	fmt.Fprintf(w, "%d of %d item(s) shown", len(shown), total)
	if len(parts) > 0 {
		fmt.Fprintf(w, "; %s", strings.Join(parts, ", "))
	}
	if len(hidden) > 0 {
		verb := "hidden"
		if all {
			verb = "usually hidden"
		}
		fmt.Fprintf(w, "; %s: %s", verb, strings.Join(hidden, ", "))
	}
	fmt.Fprintln(w)
	return nil
}

// workKinds are short names for work kinds, in the order they're summarized.
var workKinds = []struct{ kind, one, many string }{
	{enrich.WorkNewResource, "resource", "resources"},
	{enrich.WorkNewDataSource, "data source", "data sources"},
	{enrich.WorkNewOperation, "operation", "operations"},
	{enrich.WorkNewArgument, "argument", "arguments"},
	{enrich.WorkNewAttribute, "attribute", "attributes"},
	{enrich.WorkNewEnumValue, "enum value", "enum values"},
	{enrich.WorkTypeChange, "type change", "type changes"},
	{enrich.WorkRemoval, "removal", "removals"},
	{enrich.WorkTagging, "tagging", "tagging"},
}

func workSummary(work map[string]int) string {
	var parts []string
	for _, k := range workKinds {
		switch n := work[k.kind]; {
		case n == 1:
			parts = append(parts, "1 "+k.one)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, k.many))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
