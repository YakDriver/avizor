package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

// Post states.
const (
	// PostOpen: nothing covers the post and it has no decision. These are the posts to work.
	PostOpen = "open"
	// PostCovered: a decision names the covering issues, or a confident issue or PR candidate covers the post.
	PostCovered = "covered"
	// PostNoWork: the post was decided as having no Terraform work (skipped).
	PostNoWork = "no_work"
	// PostDeferred: the post was deferred, and its revisit date hasn't passed.
	PostDeferred = "deferred"
)

// postStates are the post states in the order they're reported. Only open posts are shown by default.
var postStates = []string{PostOpen, PostCovered, PostNoWork, PostDeferred}

// Announcements matched to a post: posted from postAnnBefore before it to postAnnAfter after, scoring at least
// postAnnMin and postAnnRatio of the best, postAnnMax at most. A post often comes weeks after the What's New
// announcement, and AWS sometimes posts two announcements for one launch, so there's no single pick.
const (
	postAnnBefore = 45 * 24 * time.Hour
	postAnnAfter  = 3 * 24 * time.Hour
	postAnnMin    = 0.35
	postAnnRatio  = 0.75
	postAnnMax    = 3
)

// newsBlogPrefix is the AWS News Blog's URL prefix. Its posts cover the bigger launches.
const newsBlogPrefix = "https://aws.amazon.com/blogs/aws/"

// PostEntry is a blog post and whether existing work covers it.
type PostEntry struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Posted string `json:"posted"`
	// Launch is set for a post tagged as a launch announcement, and NewsBlog for a post on the AWS News Blog.
	Launch   bool   `json:"launch,omitempty"`
	NewsBlog bool   `json:"news_blog,omitempty"`
	State    string `json:"state"`
	// Numbers are the issues and PRs that cover the post: the decision's, or else the covering candidates'.
	Numbers  []int              `json:"numbers,omitempty"`
	Decision *items.Disposition `json:"decision,omitempty"`
	// Revisit is set for a deferred post whose revisit date has passed, which makes it open again.
	Revisit bool `json:"revisit,omitempty"`
	// Announcements are the What's New announcements matched to the post. Their URLs are searched with the post's.
	Announcements []PostAnnouncement `json:"announcements"`
	// Covering are open issues and PRs, and merged PRs, that a strong signal ties to the post. Related are the best
	// of the rest.
	Covering []PostCandidate `json:"covering"`
	Related  []PostCandidate `json:"related"`
}

// PostAnnouncement is a What's New announcement matched to a post.
type PostAnnouncement struct {
	Headline string  `json:"headline"`
	URL      string  `json:"url"`
	Posted   string  `json:"posted"`
	Days     float64 `json:"days"`
	Score    float64 `json:"score"`
}

// PostCandidate is an issue or PR found for a post.
type PostCandidate struct {
	Number   int    `json:"number"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	State    string `json:"state,omitempty"`
	Title    string `json:"title"`
	// Signals are the strong signals that tie a covering candidate to the post.
	Signals []string `json:"signals,omitempty"`
}

// PostsSummary counts the posts in the window by state. Coverage is covered / (covered + open): the share of posts
// that need work and have it tracked. Posts decided as no work, and deferred ones, don't count.
type PostsSummary struct {
	Posts    int            `json:"posts"`
	Counts   map[string]int `json:"counts"`
	Coverage float64        `json:"coverage"`
}

// PostsResult is the output of avizor posts.
type PostsResult struct {
	Posts   []PostEntry  `json:"posts"`
	Summary PostsSummary `json:"summary"`
}

func cmdPosts(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("posts", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	sinceFlag := fs.String("since", "30d", "only posts created since a date, RFC 3339 time, or age (30d)")
	launch := fs.Bool("launch", false, "only posts tagged as launch announcements")
	all := fs.Bool("all", false, "also show covered, no-work, and deferred posts")
	states := fs.String("state", "", "only these states, comma-separated (overrides --all): "+strings.Join(postStates, ", "))
	noCode := fs.Bool("no-code", false, "don't search the target checkout")
	related := fs.Int("related", 3, "maximum related issues and PRs per post")
	jobs := fs.Int("jobs", 4, "posts to process at once")
	asOfFlag := fs.String("as-of", "", "reconcile against the index as it was at a date (2026-01-02, end of day) or RFC 3339 time")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) > 0 {
		return fmt.Errorf("%w: posts takes no arguments", errUsage)
	}
	var want []string
	if *states != "" {
		for s := range strings.SplitSeq(*states, ",") {
			s = strings.TrimSpace(s)
			if !slices.Contains(postStates, s) {
				return fmt.Errorf("%w: unknown state %q; states are %s", errUsage, s, strings.Join(postStates, ", "))
			}
			want = append(want, s)
		}
	}
	shown := func(s string) bool { return (want == nil && (*all || s == PostOpen)) || slices.Contains(want, s) }
	since, err := parseSince(*sinceFlag, time.Now())
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	*jobs = max(*jobs, 1)
	var asOf time.Time
	if *asOfFlag != "" {
		if asOf, err = parseAsOf(*asOfFlag); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}

	t, st, err := c.open()
	if err != nil {
		return err
	}
	defer st.Close()
	ist, err := openItems(c.dir)
	if err != nil {
		return err
	}
	ps, err := ist.Posts(ctx)
	if err != nil {
		ist.Close()
		return err
	}
	decisions, err := ist.Current(ctx, t.Name)
	ist.Close()
	if err != nil {
		return err
	}
	ps = slices.DeleteFunc(ps, func(p whatsnew.Post) bool {
		return p.PostedAt.Before(since) || (*launch && !slices.Contains(p.Tags, whatsnew.LaunchTag))
	})
	if len(ps) == 0 {
		fmt.Fprintln(stderr, "avizor: warning: no blog posts in the window; pull them with `avizor pull aws-blogs`")
	}
	anns := announcements(c.dir, stderr)

	svcs := loadServices(t, stderr)
	var ops reconcile.Reconciler
	c.operations(ctx, t, &ops, stderr)
	results := make([]*reconcile.Result, len(ps))
	errs := make([]error, len(ps))
	matched := make([][]PostAnnouncement, len(ps))
	next := make(chan int)
	var wg sync.WaitGroup
	for range *jobs {
		wg.Go(func() {
			// Each worker has its own reconciler, which caches state per run.
			r := &reconcile.Reconciler{Store: st, Target: t, Services: svcs, Operations: ops.Operations, AsOf: asOf,
				OperationsSource: ops.OperationsSource, Options: reconcile.Options{NoFetch: true, NoCode: *noCode, Version: version}}
			for i := range next {
				matched[i] = postAnnouncements(anns, ps[i])
				results[i], errs[i] = r.Run(ctx, "", postText(ps[i], matched[i]))
			}
		})
	}
	for i := range ps {
		next <- i
	}
	close(next)
	wg.Wait()

	now := time.Now()
	out := PostsResult{Posts: []PostEntry{}, Summary: PostsSummary{Posts: len(ps), Counts: map[string]int{}}}
	for i, p := range ps {
		if errs[i] != nil {
			return fmt.Errorf("%s: %w", p.URL, errs[i])
		}
		var d *items.Disposition
		if dd, ok := decisions[p.URL]; ok {
			d = &dd
		}
		e := postEntry(p, matched[i], results[i], d, *related, now)
		out.Summary.Counts[e.State]++
		if shown(e.State) {
			out.Posts = append(out.Posts, e)
		}
	}
	if n := out.Summary.Counts[PostCovered] + out.Summary.Counts[PostOpen]; n > 0 {
		out.Summary.Coverage = float64(out.Summary.Counts[PostCovered]) / float64(n)
	}
	if c.format != "text" {
		return writeJSON(stdout, out)
	}
	return writePostsText(stdout, out, shown)
}

// postAnnouncements returns the announcements matched to a post, best first.
func postAnnouncements(m *whatsnew.Matcher, p whatsnew.Post) []PostAnnouncement {
	out := []PostAnnouncement{}
	if m == nil {
		return out
	}
	for _, r := range m.Rank(p.Title+"\n"+p.Excerpt, "", p.PostedAt, postAnnBefore, postAnnAfter, postAnnMax) {
		if r.RegionExpansion || r.Score < postAnnMin || (len(out) > 0 && r.Score < postAnnRatio*out[0].Score) {
			continue
		}
		out = append(out, PostAnnouncement{Headline: r.Announcement.Headline, URL: r.Announcement.URL,
			Posted: r.Announcement.PostedAt.Format(time.DateOnly), Days: r.Days, Score: r.Score})
	}
	return out
}

// postText is what a post is reconciled from: its title and excerpt, its URL, and its announcements' URLs. An issue
// that links the post or an announcement matches on the URL, a strong signal.
func postText(p whatsnew.Post, anns []PostAnnouncement) string {
	var b strings.Builder
	b.WriteString(p.Title)
	if p.Excerpt != "" {
		b.WriteString(". " + p.Excerpt)
	}
	b.WriteString("\n" + p.URL)
	for _, a := range anns {
		b.WriteString("\n" + a.URL)
	}
	return b.String()
}

// postEntry decides a post's state. A decision comes first: created and duplicate cover it, skipped means no work, and
// deferred holds it until the revisit date. Without one, a confident issue or PR candidate covers it.
func postEntry(p whatsnew.Post, anns []PostAnnouncement, res *reconcile.Result, d *items.Disposition, relatedLimit int, now time.Time) PostEntry {
	e := PostEntry{URL: p.URL, Title: p.Title, Posted: p.PostedAt.Format(time.DateOnly), Launch: slices.Contains(p.Tags, whatsnew.LaunchTag),
		NewsBlog: strings.HasPrefix(p.URL, newsBlogPrefix), Decision: d, Announcements: anns, Covering: []PostCandidate{},
		Related: []PostCandidate{}}
	for _, c := range res.Candidates {
		if c.Kind != reconcile.KindIssue && c.Kind != reconcile.KindPR {
			continue
		}
		pc := PostCandidate{Number: c.Number, Kind: c.Kind, Category: c.Category, State: c.State, Title: c.Title}
		switch c.Category {
		case reconcile.CategoryTracked, reconcile.CategoryImplemented:
			for _, m := range c.Signals {
				if m.Strength == reconcile.StrengthStrong {
					pc.Signals = append(pc.Signals, m.Type+":"+m.Value)
				}
			}
			e.Covering = append(e.Covering, pc)
		default:
			if len(e.Related) < relatedLimit {
				e.Related = append(e.Related, pc)
			}
		}
	}
	switch {
	case d != nil && (d.Decision == items.DecisionCreated || d.Decision == items.DecisionDuplicate):
		e.State, e.Numbers = PostCovered, d.Numbers()
	case d != nil && d.Decision == items.DecisionSkipped:
		e.State = PostNoWork
	case d != nil && d.Decision == items.DecisionDeferred && now.Before(d.RevisitAfter):
		e.State = PostDeferred
	case len(e.Covering) > 0:
		e.State = PostCovered
		for _, c := range e.Covering {
			e.Numbers = append(e.Numbers, c.Number)
		}
	default:
		e.State = PostOpen
		e.Revisit = d != nil && d.Decision == items.DecisionDeferred
	}
	return e
}

// writePostsText writes the posts and a summary line, which counts the states shown and hidden.
func writePostsText(w io.Writer, res PostsResult, shownState func(string) bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, e := range res.Posts {
		var flags []string
		if e.NewsBlog {
			flags = append(flags, "news")
		}
		if e.Launch {
			flags = append(flags, "launch")
		}
		note := ""
		switch {
		case e.Revisit:
			note = decisionText(*e.Decision) + "; revisit due"
		case e.Decision != nil && (e.Decision.Decision != items.DecisionDeferred || e.State == PostDeferred):
			note = decisionText(*e.Decision)
		case e.State == PostCovered:
			note = "covered by " + issueList(e.Numbers)
		case len(e.Related) > 0:
			rel := make([]string, 0, len(e.Related))
			for _, r := range e.Related {
				rel = append(rel, fmt.Sprintf("#%d%s", r.Number, prTag(r.Kind)))
			}
			note = "related " + strings.Join(rel, ", ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.Posted, e.State, cmp.Or(strings.Join(flags, ","), "-"), truncate(e.Title, 80), note)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	var shown, hidden []string
	for _, s := range postStates {
		n := res.Summary.Counts[s]
		if n == 0 {
			continue
		}
		p := fmt.Sprintf("%s %d", strings.ReplaceAll(s, "_", " "), n)
		if shownState(s) {
			shown = append(shown, p)
		} else {
			hidden = append(hidden, p)
		}
	}
	fmt.Fprintf(w, "%d of %d post(s) shown", len(res.Posts), res.Summary.Posts)
	if len(shown) > 0 {
		fmt.Fprintf(w, "; %s", strings.Join(shown, ", "))
	}
	if len(hidden) > 0 {
		fmt.Fprintf(w, "; hidden: %s", strings.Join(hidden, ", "))
	}
	if n := res.Summary.Counts[PostCovered] + res.Summary.Counts[PostOpen]; n > 0 {
		fmt.Fprintf(w, "; coverage %.0f%% (%d of %d)", 100*res.Summary.Coverage, res.Summary.Counts[PostCovered], n)
	}
	fmt.Fprintln(w)
	return nil
}
