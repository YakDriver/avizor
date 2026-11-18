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

	"github.com/YakDriver/avizor/internal/config"
	"github.com/YakDriver/avizor/internal/draft"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/reconcile"
)

func cmdDecide(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("decide", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	reason := fs.String("reason", "", "why (required for skipped)")
	revisitFlag := fs.String("revisit", "", "re-surface the item after an age (30d) or date; deferred defaults to 30d")
	by := fs.String("by", os.Getenv("USER"), "who decided")
	var comments stringList
	fs.Var(&comments, "comment", "URL of a comment made on an existing issue or PR for the item (repeatable)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) < 2 {
		return fmt.Errorf("%w: decide takes an item ID, a decision (%s), and for created or duplicate one or more issue numbers",
			errUsage, strings.Join(items.Decisions, ", "))
	}
	now := time.Now()
	d := items.Disposition{Item: pos[0], Decision: pos[1], Reason: strings.TrimSpace(*reason), DecidedAt: now, DecidedBy: *by}
	if !slices.Contains(items.Decisions, d.Decision) {
		return fmt.Errorf("%w: unknown decision %q; decisions are %s", errUsage, d.Decision, strings.Join(items.Decisions, ", "))
	}
	numbers := pos[2:]
	switch d.Decision {
	case items.DecisionCreated, items.DecisionDuplicate:
		if len(numbers) == 0 {
			return fmt.Errorf("%w: %s needs the issue or PR number", errUsage, d.Decision)
		}
		seen := map[int]bool{}
		for _, a := range numbers {
			n, err := strconv.Atoi(strings.TrimPrefix(a, "#"))
			if err != nil || n <= 0 {
				return fmt.Errorf("%w: %q isn't an issue or PR number", errUsage, a)
			}
			if seen[n] {
				return fmt.Errorf("%w: #%d is listed twice", errUsage, n)
			}
			seen[n] = true
			if d.Number == 0 {
				d.Number = n
			} else {
				d.Also = append(d.Also, n)
			}
		}
	default:
		if len(numbers) > 0 {
			return fmt.Errorf("%w: %s takes no issue number", errUsage, d.Decision)
		}
	}
	if d.Decision == items.DecisionSkipped && d.Reason == "" {
		return fmt.Errorf("%w: skipped needs --reason", errUsage)
	}
	if len(comments) > 0 && d.Decision == items.DecisionCleared {
		return fmt.Errorf("%w: cleared takes no --comment", errUsage)
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
	for _, u := range comments {
		if !isCommentURL(u, t.Repo) {
			return fmt.Errorf("%w: --comment %q isn't a comment URL in %s (https://github.com/%s/issues/N#issuecomment-M)",
				errUsage, u, t.Repo, t.Repo)
		}
		if !slices.Contains(d.Comments, u) {
			d.Comments = append(d.Comments, u)
		}
	}
	for _, n := range d.Numbers() {
		if is, err := st.Issue(ctx, n); err == nil && is == nil {
			fmt.Fprintf(stderr, "avizor: warning: #%d isn't in the index yet; run `avizor sync` to see it in candidates\n", n)
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
		return fmt.Errorf("no item %q; list items with `avizor items`", d.Item)
	}
	if d.Decision != items.DecisionCleared {
		// Record what Avizor found now, so the queue can notice when it changes.
		it, err := decodeItem(*rec)
		if err != nil {
			return err
		}
		svcs := loadServices(t, stderr)
		r := &reconcile.Reconciler{Store: st, Target: t, Services: svcs,
			Options: reconcile.Options{NoFetch: true, Version: version}}
		c.operations(ctx, t, r, stderr)
		e, err := queueEntry(ctx, it, queueEnricher(c.cfg, c.dir, t, svcs, nil, it.Source, false), r, draft.Options{
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

// decisionText is a decision in a few words: "duplicate of #5", "created #7, also #8", "skipped (no provider work),
// revisit 2026-12-01".
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
	if len(d.Also) > 0 {
		s += ", also " + issueList(d.Also)
	}
	if len(d.Comments) > 0 {
		var ns []string
		for _, u := range d.Comments {
			ns = append(ns, "#"+commentURLRE.FindStringSubmatch(u)[2])
		}
		s += ", commented on " + strings.Join(ns, ", ")
	}
	if d.Reason != "" {
		s += " (" + d.Reason + ")"
	}
	if !d.RevisitAfter.IsZero() {
		s += ", revisit " + d.RevisitAfter.UTC().Format(time.DateOnly)
	}
	return s
}

func issueList(ns []int) string {
	var out []string
	for _, n := range ns {
		out = append(out, fmt.Sprintf("#%d", n))
	}
	return strings.Join(out, ", ")
}

// commentURLRE matches a GitHub issue or PR comment URL, including a PR review comment (#discussion_r…). The groups are
// the repository and the issue or PR number.
var commentURLRE = regexp.MustCompile(`^https://github\.com/([^/]+/[^/]+)/(?:issues|pull)/(\d+)#(?:issuecomment-|discussion_r)\d+$`)

// isCommentURL reports whether u is a comment URL on an issue or PR in repo.
func isCommentURL(u, repo string) bool {
	m := commentURLRE.FindStringSubmatch(u)
	if m == nil || !strings.EqualFold(m[1], repo) {
		return false
	}
	// Review comments exist only on PRs.
	return !strings.Contains(u, "#discussion_r") || strings.Contains(u, "/pull/")
}

// stringList is a repeatable string flag.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(s string) error {
	*l = append(*l, strings.TrimSpace(s))
	return nil
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
