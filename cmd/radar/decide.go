package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/draft"
	"github.com/YakDriver/radar/internal/items"
	"github.com/YakDriver/radar/internal/reconcile"
)

func cmdDecide(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("decide", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	reason := fs.String("reason", "", "why (required for skipped)")
	revisitFlag := fs.String("revisit", "", "re-surface the item after an age (30d) or date; deferred defaults to 30d")
	by := fs.String("by", os.Getenv("USER"), "who decided")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) < 2 || len(pos) > 3 {
		return fmt.Errorf("%w: decide takes an item ID, a decision (%s), and for created or duplicate an issue number",
			errUsage, strings.Join(items.Decisions, ", "))
	}
	now := time.Now()
	d := items.Disposition{Item: pos[0], Decision: pos[1], Reason: strings.TrimSpace(*reason), DecidedAt: now, DecidedBy: *by}
	if !slices.Contains(items.Decisions, d.Decision) {
		return fmt.Errorf("%w: unknown decision %q; decisions are %s", errUsage, d.Decision, strings.Join(items.Decisions, ", "))
	}
	switch d.Decision {
	case items.DecisionCreated, items.DecisionDuplicate:
		if len(pos) != 3 {
			return fmt.Errorf("%w: %s needs the issue or PR number", errUsage, d.Decision)
		}
		if d.Number, err = strconv.Atoi(strings.TrimPrefix(pos[2], "#")); err != nil || d.Number <= 0 {
			return fmt.Errorf("%w: %q isn't an issue or PR number", errUsage, pos[2])
		}
	default:
		if len(pos) == 3 {
			return fmt.Errorf("%w: %s takes no issue number", errUsage, d.Decision)
		}
	}
	if d.Decision == items.DecisionSkipped && d.Reason == "" {
		return fmt.Errorf("%w: skipped needs --reason", errUsage)
	}
	if d.Decision == items.DecisionDeferred && *revisitFlag == "" {
		*revisitFlag = "30d"
	}
	if *revisitFlag != "" {
		if d.Decision == items.DecisionCleared {
			return fmt.Errorf("%w: cleared takes no --revisit", errUsage)
		}
		if d.RevisitAfter, err = parseRevisit(*revisitFlag, now); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}

	t, st, err := c.open()
	if err != nil {
		return err
	}
	defer st.Close()
	d.Target = t.Name
	if d.Number != 0 {
		if is, err := st.Issue(ctx, d.Number); err == nil && is == nil {
			fmt.Fprintf(stderr, "radar: warning: #%d isn't in the index yet; run `radar sync` to see it in candidates\n", d.Number)
		}
	}

	ist, err := openItems(c.dir)
	if err != nil {
		return err
	}
	defer ist.Close()
	rec, err := ist.Get(ctx, d.Item)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("no item %q; list items with `radar items`", d.Item)
	}
	if d.Decision != items.DecisionCleared {
		// Record what Radar found now, so the queue can notice when it changes.
		it, err := decodeItem(*rec)
		if err != nil {
			return err
		}
		svcs := loadServices(t, stderr)
		r := &reconcile.Reconciler{Store: st, Target: t, Services: svcs,
			Options: reconcile.Options{NoFetch: true, Version: version}}
		c.operations(ctx, t, r, stderr)
		e, err := queueEntry(ctx, it, queueEnricher(c.cfg, c.dir, t, svcs, it.Source, false), r, draft.Options{
			Repo: t.Repo, Checkout: t.Checkout, Prefix: t.TypePrefix, LabelPrefix: t.ServiceLabelPrefix, Template: t.DraftTemplate,
		})
		if err != nil {
			return err
		}
		if d.Evidence, err = json.Marshal(e.Evidence); err != nil {
			return err
		}
	}
	if err := ist.Decide(ctx, d); err != nil {
		return err
	}
	if c.format != "text" {
		return writeJSON(stdout, d)
	}
	fmt.Fprintf(stdout, "%s: %s\n", d.Item, decisionText(d))
	return nil
}

func cmdDecisions(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("decisions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	history := fs.Bool("history", false, "list every recorded decision, oldest first, not just current ones")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) > 1 {
		return fmt.Errorf("%w: decisions takes at most one item ID", errUsage)
	}
	cfg, dir, err := c.load()
	if err != nil {
		return err
	}
	t, err := cfg.Target(c.target)
	if err != nil {
		return err
	}
	ist, err := openItems(dir)
	if err != nil {
		return err
	}
	defer ist.Close()

	var list []items.Disposition
	item := ""
	if len(pos) == 1 {
		item = pos[0]
	}
	if *history {
		if list, err = ist.History(ctx, t.Name, item); err != nil {
			return err
		}
	} else {
		cur, err := ist.Current(ctx, t.Name)
		if err != nil {
			return err
		}
		for _, d := range cur {
			if item == "" || d.Item == item {
				list = append(list, d)
			}
		}
		slices.SortFunc(list, func(a, b items.Disposition) int { return b.DecidedAt.Compare(a.DecidedAt) })
	}
	if list == nil {
		list = []items.Disposition{}
	}
	if c.format != "text" {
		return writeJSON(stdout, list)
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for _, d := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", d.DecidedAt.Local().Format(time.DateOnly), d.Item, decisionText(d), d.DecidedBy)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d decision(s)\n", len(list))
	return nil
}

// decisionText is a decision in a few words: "duplicate of #5", "skipped (no provider work), revisit 2026-12-01".
func decisionText(d items.Disposition) string {
	var s string
	switch d.Decision {
	case items.DecisionCreated:
		s = fmt.Sprintf("created #%d", d.Number)
	case items.DecisionDuplicate:
		s = fmt.Sprintf("duplicate of #%d", d.Number)
	default:
		s = d.Decision
	}
	if d.Reason != "" {
		s += " (" + d.Reason + ")"
	}
	if !d.RevisitAfter.IsZero() {
		s += ", revisit " + d.RevisitAfter.UTC().Format(time.DateOnly)
	}
	return s
}

var revisitAgeRE = regexp.MustCompile(`^\d+[dh]$`)

// parseRevisit reads a revisit time: an age from now (30d, 12h) or a date or RFC 3339 time.
func parseRevisit(s string, now time.Time) (time.Time, error) {
	t, err := config.ParseSince(s, now)
	if err != nil {
		return time.Time{}, fmt.Errorf("--revisit: %w", err)
	}
	if revisitAgeRE.MatchString(s) {
		// ParseSince reads an age as that long ago.
		t = now.Add(now.Sub(t))
	}
	return t, nil
}
