package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/YakDriver/avizor/internal/reconcile"
)

var categoryTitles = map[string]string{
	reconcile.CategoryImplemented: "already implemented",
	reconcile.CategoryTracked:     "already tracked",
	reconcile.CategoryConsidered:  "previously considered",
	reconcile.CategoryRelated:     "related",
}

func writeText(w io.Writer, res *reconcile.Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	p := func(format string, args ...any) { fmt.Fprintf(tw, format+"\n", args...) }

	in := res.Input
	switch {
	case in.Number != 0:
		p("input:\t#%d %s", in.Number, in.Title)
	case in.Title != "":
		p("input:\t%s (%s)", in.Title, in.URL)
	default:
		p("input:\t%s", in.Ref)
	}
	target := fmt.Sprintf("%s, %d issues and %d PRs indexed", res.Target.Repo, res.Target.Issues, res.Target.PRs)
	if res.Target.IndexSynced != "" {
		target += ", synced " + res.Target.IndexSynced
	}
	p("target:\t%s", target)
	if res.Target.CheckoutRef != "" {
		p("checkout:\t%s (%s)", res.Target.Checkout, res.Target.CheckoutRef)
	}
	if len(in.Keywords) > 0 {
		p("keywords:\t%s", strings.Join(in.Keywords, ", "))
	}
	for _, inf := range res.Inferences {
		p("inferred %s:\t%s (%s)", inf.Type, inf.Value, inf.Basis)
	}
	{
		var ev []string
		for _, e := range res.Evidence {
			if !e.Common {
				ev = append(ev, e.Type+":"+e.Value)
			}
		}
		if len(ev) > 0 {
			p("evidence:\t%s", strings.Join(ev, ", "))
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	for _, cat := range reconcile.Categories {
		var cs []reconcile.Candidate
		for _, c := range res.Candidates {
			if c.Category == cat {
				cs = append(cs, c)
			}
		}
		if len(cs) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n", categoryTitles[cat])
		for _, c := range cs {
			switch c.Kind {
			case reconcile.KindCode, reconcile.KindDoc:
				fmt.Fprintf(w, "  %-6s %s\n", c.Kind, c.Path)
			default:
				fmt.Fprintf(w, "  #%-6d %-6s %s\n", c.Number, stateLabel(c), c.Title)
			}
			fmt.Fprintf(w, "          matched: %s\n", matches(c.Signals))
		}
	}

	if res.Summary.Unmatched {
		fmt.Fprintln(w, "\nunmatched: no implemented, tracked, or previously considered work found")
	}
	if len(res.Warnings) > 0 {
		fmt.Fprintln(w, "\nwarnings")
		for _, wn := range res.Warnings {
			fmt.Fprintf(w, "  %s\n", wn)
		}
	}
	return nil
}

func stateLabel(c reconcile.Candidate) string {
	kind := ""
	if c.Kind == reconcile.KindPR {
		kind = "pr "
	}
	switch {
	case c.Merged:
		return kind + "merged"
	case c.StateReason != "" && c.State == "closed":
		return kind + strings.ReplaceAll(c.StateReason, "_", " ")
	default:
		return kind + c.State
	}
}

func matches(ms []reconcile.Match) string {
	parts := make([]string, 0, len(ms))
	for _, m := range ms {
		s := m.Type + ":" + m.Value
		if m.Where != "" && m.Type != "keyword" {
			s += " (" + m.Where + ")"
		}
		if m.Strength == reconcile.StrengthStrong {
			s = "*" + s
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}
