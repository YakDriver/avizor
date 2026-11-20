package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/draft"
	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/queue"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func cmdDraft(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("draft", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	noCode := fs.Bool("no-code", false, "don't search the target checkout")
	relates := fs.Int("relates", 3, "maximum related issues and PRs to suggest")
	useAnn := fs.Bool("announcement", false, "link the matched What's New announcement in the body, below the release notes, "+
		"with the blog posts that match it")
	var blogs repeated
	fs.Var(&blogs, "blog", "link this blog post in the body, by URL (repeatable); `avizor brief` lists the posts near the release")
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

	it, en, err := c.enrichedItem(ctx, r, pos[0], *noCode, stderr)
	if err != nil {
		return err
	}
	rec, err := r.RunItem(ctx, enrich.ReconcileItem(it, en))
	if err != nil {
		return err
	}
	var posts []draft.BlogPost
	for _, u := range blogs {
		p, err := blogPost(ctx, c.dir, u)
		if err != nil {
			return err
		}
		posts = append(posts, draft.BlogPost{Post: *p})
	}
	if *useAnn && en.Announcement != nil {
		ps, err := announcedPosts(ctx, c.dir, it, en, stderr)
		if err != nil {
			return err
		}
		posts = append(posts, ps...)
	}
	texts, err := candidateTexts(ctx, st, rec)
	if err != nil {
		return err
	}
	d, err := queue.Draft(it, en, rec, texts, draft.Options{
		Repo: t.Repo, Checkout: t.Checkout, Prefix: t.TypePrefix, LabelPrefix: t.ServiceLabelPrefix, Template: t.DraftTemplate,
		RelatesLimit: *relates, Announcement: *useAnn, BlogPosts: posts,
	})
	if err != nil {
		return err
	}

	switch {
	case *bodyOnly:
		// stdout is for piping to gh, so warnings go to stderr.
		for _, w := range d.Warnings {
			fmt.Fprintf(stderr, "avizor: warning: %s\n", w)
		}
		_, err := io.WriteString(stdout, d.Body)
		return err
	case c.format == "text":
		return writeDraftText(stdout, d, regenArgs(fs, d.Item, t.Name))
	}
	return writeJSON(stdout, d)
}

// blogPost returns the stored blog post with a URL, or, for a URL that isn't stored, a post titled with the URL.
func blogPost(ctx context.Context, dir, u string) (*whatsnew.Post, error) {
	st, err := openItems(dir)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	ps, err := st.Posts(ctx)
	if err != nil {
		return nil, err
	}
	norm := func(s string) string { return strings.TrimSuffix(s, "/") }
	if i := slices.IndexFunc(ps, func(p whatsnew.Post) bool { return norm(p.URL) == norm(u) }); i >= 0 {
		return &ps[i], nil
	}
	return &whatsnew.Post{URL: u, Title: u}, nil
}

// announcedPostsMax caps the blog posts linked for a matched announcement. On the 51 blog eval cases, 35 with an
// announcement, linking the best 2 links an expected post for 30 cases, and 31 of 40 links are expected. Linking only
// the best hits 28 cases. Labels list the posts that issues linked, so some unexpected links are on topic.
const announcedPostsMax = 2

// announcedPostsCandidates is how many of the posts near the release, best first, are checked against the
// announcement. Checking a post ranks the announcements for it, about 2ms, and a window holds 400 to 700 posts. On the
// blog eval cases, checking 5, 10, 20, or all links the same posts; 10 takes 0.60s per draft and all 1.13s.
const announcedPostsCandidates = 10

// unlinkedKinds are the kinds of post not linked for a matched announcement. Event and developer tools posts are
// linked: News Blog launch posts during re:Invent have the event tag, and posts about CloudFormation and CDK features
// the developer tools tag, and issues link both.
var unlinkedKinds = []string{KindRoundup, KindCustomerStory}

// announcedPosts returns the blog posts near the item's release that match its announcement, best first: of the best
// posts brief lists, those whose own matched announcements, as posts finds them, include the item's.
func announcedPosts(ctx context.Context, dir string, it *awssdk.Item, en *enrich.Result, stderr io.Writer) ([]draft.BlogPost, error) {
	m, anns := blogPosts(dir, stderr), announcements(dir, stderr)
	if m == nil || anns == nil {
		return nil, nil
	}
	st, err := openItems(dir)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	var lookupErr error
	out := linkPosts(m, anns, it, en.Announcement, func(u string) *whatsnew.Post {
		p, err := st.Post(ctx, u)
		if err != nil && lookupErr == nil {
			lookupErr = err
		}
		return p
	})
	return out, lookupErr
}

// linkPosts is announcedPosts over posts and announcements, with post looking up a stored post by URL.
func linkPosts(posts, anns *whatsnew.Matcher, it *awssdk.Item, ann *whatsnew.Match, post func(string) *whatsnew.Post) []draft.BlogPost {
	want := ann.Announcement.URL
	var out []draft.BlogPost
	for _, bp := range enrich.BlogPosts(posts, it, ann, announcedPostsCandidates) {
		p := post(bp.URL)
		if p == nil {
			continue
		}
		pa := postAnnouncements(anns, *p)
		if !slices.ContainsFunc(pa, func(a PostAnnouncement) bool { return a.URL == want }) || slices.ContainsFunc(unlinkedKinds, func(k string) bool { return isKind(*p, k) }) {
			continue
		}
		out = append(out, draft.BlogPost{Post: *p, Basis: "matches the item's announcement, " + want})
		if len(out) == announcedPostsMax {
			break
		}
	}
	return out
}

// repeated is a flag that can be given more than once, collecting its values in order.
type repeated []string

func (r *repeated) String() string { return strings.Join(*r, ",") }

func (r *repeated) Set(v string) error {
	*r = append(*r, v)
	return nil
}

// regenArgs is the command that prints this draft's body again: the same item, with the flags that were set (config,
// state directory, target, and options) and the target it resolved to.
func regenArgs(fs *flag.FlagSet, item, target string) []string {
	args := []string{"avizor", "draft", item}
	hasTarget := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "format", "body":
			return
		case "target":
			hasTarget = true
		}
		if r, ok := f.Value.(*repeated); ok {
			for _, v := range *r {
				args = append(args, "--"+f.Name+"="+v)
			}
			return
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
	if len(d.Decide) > 0 {
		fmt.Fprintln(w, "\nto decide (marked <!-- decide: ... --> in the body):")
		for _, c := range d.Decide {
			fmt.Fprintf(w, "  %s\n", c)
		}
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
