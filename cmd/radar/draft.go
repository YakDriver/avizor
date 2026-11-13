package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/YakDriver/radar/internal/draft"
	"github.com/YakDriver/radar/internal/enrich"
	"github.com/YakDriver/radar/internal/reconcile"
)

func cmdDraft(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("draft", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	noCode := fs.Bool("no-code", false, "don't search the target checkout")
	relates := fs.Int("relates", 3, "maximum related issues and PRs to suggest")
	useAnn := fs.Bool("announcement", false, "put the matched What's New announcement in the body instead of the release notes")
	bodyOnly := fs.Bool("body", false, "print only the issue body, for piping to `gh issue create --body-file -`")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) != 1 {
		return fmt.Errorf("%w: draft takes one item ID", errUsage)
	}

	t, st, err := c.open()
	if err != nil {
		return err
	}
	defer st.Close()

	r := &reconcile.Reconciler{
		Store:    st,
		Target:   t,
		Services: loadServices(t, stderr),
		Options:  reconcile.Options{NoFetch: true, NoCode: *noCode, Version: version},
	}
	c.operations(ctx, t, r, stderr)

	it, en, err := c.enrichedItem(ctx, t, pos[0], *noCode, stderr)
	if err != nil {
		return err
	}
	rec, err := r.RunItem(ctx, enrich.ReconcileItem(it, en))
	if err != nil {
		return err
	}
	d, err := draft.New(it, en, rec, draft.Options{
		Repo: t.Repo, Checkout: t.Checkout, Prefix: t.TypePrefix, LabelPrefix: t.ServiceLabelPrefix, Template: t.DraftTemplate,
		RelatesLimit: *relates, Announcement: *useAnn,
	})
	if err != nil {
		return err
	}

	switch {
	case *bodyOnly:
		_, err := io.WriteString(stdout, d.Body)
		return err
	case c.format == "text":
		return writeDraftText(stdout, d, regenArgs(fs, d.Item, t.Name))
	}
	return writeJSON(stdout, d)
}

// regenArgs is the command that prints this draft's body again: the same item, with the flags that were set (config,
// state directory, target, and options) and the target it resolved to.
func regenArgs(fs *flag.FlagSet, item, target string) []string {
	args := []string{"radar", "draft", item}
	hasTarget := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "format", "body":
			return
		case "target":
			hasTarget = true
		}
		args = append(args, "--"+f.Name+"="+f.Value.String())
	})
	if !hasTarget {
		args = append(args, "--target="+target)
	}
	return append(args, "--body")
}

func writeDraftText(w io.Writer, d *draft.Draft, regen []string) error {
	fmt.Fprintf(w, "title:   %s\nlabels:  %s\n\n%s\n", d.Title, strings.Join(d.Labels, ", "), d.Body)
	fmt.Fprintln(w, "--- for the reviewer (not part of the issue)")
	for _, warn := range d.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
	if len(d.Inferred) > 0 {
		fmt.Fprintln(w, "\ninferred:")
		for _, in := range d.Inferred {
			fmt.Fprintf(w, "  %s: %s (%s)\n", in.Field, in.Value, in.Basis)
		}
	}
	if len(d.Relates) > 0 {
		fmt.Fprintln(w, "\npossibly related (add \"Relates <url>.\" if so):")
		for _, r := range d.Relates {
			fmt.Fprintf(w, "  #%d%s %s (%s)\n", r.Number, prTag(r.Kind), r.Title, r.Basis)
		}
	}
	fmt.Fprintf(w, "\nto file it:\n  %s | gh %s\n", shellJoin(regen), shellJoin(d.GHArgs))
	return nil
}

// shellJoin quotes arguments for a POSIX shell.
func shellJoin(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if a != "" && strings.IndexFunc(a, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./:=@", r))
		}) < 0 {
			out[i] = a
			continue
		}
		out[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(out, " ")
}

// prTag marks a pull request in a list of issue numbers.
func prTag(kind string) string {
	if kind == reconcile.KindPR {
		return " (PR)"
	}
	return ""
}
