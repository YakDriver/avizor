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

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/enrich"
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
	Launch   bool `json:"launch,omitempty"`
	NewsBlog bool `json:"news_blog,omitempty"`
	PostSignificance
	State string `json:"state"`
	// Numbers are the issues and PRs that cover the post: the decision's, or else the covering candidates'.
	Numbers  []int              `json:"numbers,omitempty"`
	Decision *items.Disposition `json:"decision,omitempty"`
	// Revisit is set for an open post whose decision's revisit date has passed.
	Revisit bool `json:"revisit,omitempty"`
	// Announcements are the What's New announcements matched to the post. Their URLs are searched with the post's.
	Announcements []PostAnnouncement `json:"announcements"`
	// Covering are open issues and PRs, and merged PRs, that a strong signal ties to the post. Related are the best
	// of the rest.
	Covering []PostCandidate `json:"covering"`
	Related  []PostCandidate `json:"related"`
	// Services are the provider services reconciliation inferred from the post.
	Services []string `json:"services,omitempty"`
	// Items are the pulled SDK items that best match the post, for drafting its issue with brief and draft.
	Items []enrich.PostItem `json:"items"`

	// CoveredBy is the SDK item whose created or duplicate decision covers the post: one of Items that matches one
	// of the post's announcements. It's set only for a post with no decision of its own.
	CoveredBy string `json:"covered_by,omitempty"`

	// byDecision is set when the decision sets the state.
	byDecision bool
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

// PostsSummary counts the posts in the window by state, and Significant counts the significant ones. Coverage is
// covered / (covered + open): the share of posts that need work and have it tracked. Posts decided as no work, and
// deferred ones, don't count.
type PostsSummary struct {
	PostCounts
	Significant PostCounts `json:"significant"`
}

// PostCounts counts posts by state, with their coverage.
type PostCounts struct {
	Posts    int            `json:"posts"`
	Counts   map[string]int `json:"counts"`
	Coverage float64        `json:"coverage"`
}

// add counts a post.
func (c *PostCounts) add(state string) {
	c.Posts++
	c.Counts[state]++
	if n := c.Counts[PostCovered] + c.Counts[PostOpen]; n > 0 {
		c.Coverage = float64(c.Counts[PostCovered]) / float64(n)
	}
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
	significant := fs.Bool("significant", false, "only show significant posts (the summary counts all posts and significant ones)")
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
	shownState := func(s string) bool { return (want == nil && (*all || s == PostOpen)) || slices.Contains(want, s) }
	shown := func(e PostEntry) bool { return shownState(e.State) && (!*significant || e.Significant) }
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
	if err != nil {
		ist.Close()
		return err
	}
	now := time.Now()
	ps, early := postsInWindow(ps, decisions, since, *launch, now)
	anns := announcements(c.dir, stderr)
	idx, err := loadItemIndex(ctx, ist, anns)
	ist.Close()
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		fmt.Fprintln(stderr, "avizor: warning: no blog posts in the window; pull them with `avizor pull aws-blogs`")
	}
	svcs := loadServices(t, stderr)
	var ops reconcile.Reconciler
	c.operations(ctx, t, &ops, stderr)
	results := make([]*reconcile.Result, len(ps))
	errs := make([]error, len(ps))
	matched := make([][]PostAnnouncement, len(ps))
	postItems := make([][]enrich.PostItem, len(ps))
	next := make(chan int)
	var wg sync.WaitGroup
	for range *jobs {
		wg.Go(func() {
			// Each worker has its own reconciler, which caches state per run.
			r := &reconcile.Reconciler{Store: st, Target: t, Services: svcs, Operations: ops.Operations, AsOf: asOf,
				OperationsSource: ops.OperationsSource, Options: reconcile.Options{NoFetch: true, NoCode: *noCode, Version: version,
					// postEntry drops code candidates and then applies --related.
					RelatedLimit: -1}}
			for i := range next {
				matched[i] = postAnnouncements(anns, ps[i])
				postItems[i] = rankPostItems(idx, ps[i], matched[i])
				results[i], errs[i] = r.Run(ctx, "", postText(ps[i], matched[i]))
			}
		})
	}
	for i := range ps {
		next <- i
	}
	close(next)
	wg.Wait()

	out := PostsResult{Posts: []PostEntry{}, Summary: PostsSummary{PostCounts: PostCounts{Counts: map[string]int{}},
		Significant: PostCounts{Counts: map[string]int{}}}}
	warned := map[string]bool{}
	for i, p := range ps {
		if errs[i] != nil {
			return fmt.Errorf("%s: %w", p.URL, errs[i])
		}
		// Warnings such as an unsynced index apply to every post; report each once.
		for _, w := range results[i].Warnings {
			if !warned[w] {
				warned[w] = true
				fmt.Fprintf(stderr, "avizor: warning: %s\n", w)
			}
		}
		var d *items.Disposition
		if dd, ok := decisions[p.URL]; ok {
			d = &dd
		}
		e := postEntry(p, matched[i], results[i], d, *related, now)
		e.Items = postItems[i]
		coverByItem(&e, p, decisions, now)
		if early[p.URL] && e.State != PostOpen {
			continue
		}
		out.Summary.add(e.State)
		if e.Significant {
			out.Summary.Significant.add(e.State)
		}
		if shown(e) {
			out.Posts = append(out.Posts, e)
		}
	}
	if c.format != "text" {
		return writeJSON(stdout, out)
	}
	return writePostsText(stdout, out, shownState, *significant)
}

// postAnnouncements returns the announcements matched to a post, best first.
func postAnnouncements(m *whatsnew.Matcher, p whatsnew.Post) []PostAnnouncement {
	out := []PostAnnouncement{}
	if m == nil {
		return out
	}
	// Rank all, since region expansions are ranked too and are skipped here. Scores are in descending order.
	for _, r := range m.Rank(p.Title+"\n"+p.Excerpt, "", p.PostedAt, postAnnBefore, postAnnAfter, 0) {
		if r.RegionExpansion {
			continue
		}
		if r.Score < postAnnMin || (len(out) > 0 && r.Score < postAnnRatio*out[0].Score) {
			break
		}
		out = append(out, PostAnnouncement{Headline: r.Announcement.Headline, URL: r.Announcement.URL,
			Posted: r.Announcement.PostedAt.Format(time.DateOnly), Days: r.Days, Score: r.Score})
		if len(out) == postAnnMax {
			break
		}
	}
	return out
}

// postItemsMax is how many SDK items are listed for a post.
const postItemsMax = 3

// loadItemIndex indexes every pulled item, matched to announcements. Term weights come from the whole set, so a post's
// items don't depend on the posts listed with it, and posts and eval rank the same way; Rank applies the release
// window. Skipped items have no API change, so they aren't loaded.
func loadItemIndex(ctx context.Context, ist *items.Store, anns *whatsnew.Matcher) (*enrich.ItemIndex, error) {
	recs, err := ist.List(ctx, items.Filter{Status: items.StatusNew})
	if err != nil {
		return nil, err
	}
	its := make([]*awssdk.Item, 0, len(recs))
	for _, r := range recs {
		it, err := decodeItem(r)
		if err != nil {
			return nil, err
		}
		its = append(its, it)
	}
	return enrich.NewItemIndex(its, anns), nil
}

// rankPostItems ranks the SDK items for a post. The query adds its matched announcements' headlines, which name the
// launch the way release notes do.
func rankPostItems(idx *enrich.ItemIndex, p whatsnew.Post, anns []PostAnnouncement) []enrich.PostItem {
	heads := make([]string, len(anns))
	urls := make([]string, len(anns))
	for i, a := range anns {
		heads[i] = a.Headline
		urls[i] = a.URL
	}
	return idx.Rank(p, urls, strings.Join(heads, "\n"), postItemsMax)
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

// postsInWindow keeps the posts created since a time, only launch announcements if launch is set. A post before then
// stays in when its decision's revisit date has passed, so a deferral that comes due after the post leaves the window
// is still seen; early marks those posts, which are listed only if they reopen.
func postsInWindow(ps []whatsnew.Post, decisions map[string]items.Disposition, since time.Time, launch bool, now time.Time) ([]whatsnew.Post, map[string]bool) {
	early := map[string]bool{}
	ps = slices.DeleteFunc(ps, func(p whatsnew.Post) bool {
		if launch && !slices.Contains(p.Tags, whatsnew.LaunchTag) {
			return true
		}
		if !p.PostedAt.Before(since) {
			return false
		}
		if d, ok := decisions[p.URL]; ok && revisitDue(d, now) {
			early[p.URL] = true
			return false
		}
		return true
	})
	return ps, early
}

// revisitDue reports whether a decision's revisit date has passed.
func revisitDue(d items.Disposition, now time.Time) bool {
	return !d.RevisitAfter.IsZero() && !now.Before(d.RevisitAfter)
}

// postEntry decides a post's state. A decision comes first: created and duplicate cover it, skipped means no work, and
// deferred holds it. Once a decision's revisit date passes, it no longer sets the state. Without one, a confident
// issue or PR candidate covers the post.
// coverByItemMin is the score the announcement a post shares with an item needs for the item's decision to cover the
// post. A weaker match is often another launch of the same service: "Rapid game server iteration on Amazon GameLift
// Servers" matched GameLift's DDoS protection announcement at 0.45. See coverByItem.
const coverByItemMin = 0.5

// coverByItem covers an open post with no decision of its own by a created or duplicate decision on one of its items,
// so filing from the item doesn't need a second decision on the post. The item must match one of the post's
// announcements, scoring at least coverByItemMin against the post, and the post can't be a roundup or a customer
// story, which share a launch's announcement without being about it. The first such item, in the post's order,
// covers it.
func coverByItem(e *PostEntry, p whatsnew.Post, decisions map[string]items.Disposition, now time.Time) {
	// A post can be of several kinds, and Kind is only the first, so each kind is checked.
	if e.State != PostOpen || e.Decision != nil && !revisitDue(*e.Decision, now) || isKind(p, KindRoundup) || isKind(p, KindCustomerStory) {
		return
	}
	for _, it := range e.Items {
		d, ok := decisions[it.ID]
		if !it.Announcement || !ok || revisitDue(d, now) || d.Decision != items.DecisionCreated && d.Decision != items.DecisionDuplicate {
			continue
		}
		if !slices.ContainsFunc(e.Announcements, func(a PostAnnouncement) bool { return a.URL == it.AnnouncementURL && a.Score >= coverByItemMin }) {
			continue
		}
		e.State, e.Numbers, e.CoveredBy, e.Revisit = PostCovered, d.Numbers(), it.ID, false
		return
	}
}

func postEntry(p whatsnew.Post, anns []PostAnnouncement, res *reconcile.Result, d *items.Disposition, relatedLimit int, now time.Time) PostEntry {
	e := PostEntry{URL: p.URL, Title: p.Title, Posted: p.PostedAt.Format(time.DateOnly), Launch: slices.Contains(p.Tags, whatsnew.LaunchTag),
		NewsBlog: strings.HasPrefix(p.URL, newsBlogPrefix), PostSignificance: postSignificance(p, anns), Decision: d,
		Announcements: anns, Covering: []PostCandidate{}, Related: []PostCandidate{}, Items: []enrich.PostItem{}}
	for _, in := range res.Inferences {
		if in.Type == "service" && !slices.Contains(e.Services, in.Value) {
			e.Services = append(e.Services, in.Value)
		}
	}
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
	due := d != nil && revisitDue(*d, now)
	switch {
	case d != nil && !due && (d.Decision == items.DecisionCreated || d.Decision == items.DecisionDuplicate):
		e.State, e.Numbers, e.byDecision = PostCovered, d.Numbers(), true
	case d != nil && !due && d.Decision == items.DecisionSkipped:
		e.State, e.byDecision = PostNoWork, true
	case d != nil && !due && d.Decision == items.DecisionDeferred:
		e.State, e.byDecision = PostDeferred, true
	case len(e.Covering) > 0:
		e.State = PostCovered
		for _, c := range e.Covering {
			e.Numbers = append(e.Numbers, c.Number)
		}
	default:
		e.State = PostOpen
		e.Revisit = due
	}
	return e
}

// writePostsText writes the posts and a summary line, which counts the states shown and hidden, then the significant
// posts' coverage.
func writePostsText(w io.Writer, res PostsResult, shownState func(string) bool, significantOnly bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, e := range res.Posts {
		var flags []string
		if e.NewsBlog {
			flags = append(flags, "news")
		}
		if e.Launch {
			flags = append(flags, "launch")
		}
		if e.Significant {
			flags = append(flags, "sig")
		}
		note := ""
		switch {
		case e.Revisit:
			note = decisionText(*e.Decision) + "; revisit due"
		case e.byDecision:
			note = decisionText(*e.Decision)
		case e.CoveredBy != "":
			note = "covered by " + issueList(e.Numbers) + " (item " + e.CoveredBy + ")"
		case e.State == PostCovered:
			note = "covered by " + issueList(e.Numbers)
		case len(e.Related) > 0:
			rel := make([]string, 0, len(e.Related))
			for _, r := range e.Related {
				rel = append(rel, fmt.Sprintf("#%d%s", r.Number, prTag(r.Kind)))
			}
			note = "related " + strings.Join(rel, ", ")
		}
		if e.State == PostOpen && len(e.Items) > 0 {
			ids := make([]string, len(e.Items))
			for i, it := range e.Items {
				ids[i] = it.ID
			}
			note = strings.TrimPrefix(note+"; items "+strings.Join(ids, ", "), "; ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Posted, e.State, cmp.Or(strings.Join(flags, ","), "-"), truncate(e.Title, 60), e.URL, note)
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
	if significantOnly {
		fmt.Fprint(w, " (significant only)")
	}
	if len(shown) > 0 {
		fmt.Fprintf(w, "; %s", strings.Join(shown, ", "))
	}
	if len(hidden) > 0 {
		fmt.Fprintf(w, "; hidden: %s", strings.Join(hidden, ", "))
	}
	if n := res.Summary.Counts[PostCovered] + res.Summary.Counts[PostOpen]; n > 0 {
		fmt.Fprintf(w, "; coverage %.0f%% (%d of %d)", 100*res.Summary.Coverage, res.Summary.Counts[PostCovered], n)
	}
	sig := res.Summary.Significant
	fmt.Fprintf(w, "\nsignificant: %d post(s)", sig.Posts)
	for _, s := range postStates {
		if n := sig.Counts[s]; n > 0 {
			fmt.Fprintf(w, ", %s %d", strings.ReplaceAll(s, "_", " "), n)
		}
	}
	if n := sig.Counts[PostCovered] + sig.Counts[PostOpen]; n > 0 {
		fmt.Fprintf(w, "; coverage %.0f%% (%d of %d)", 100*sig.Coverage, sig.Counts[PostCovered], n)
	}
	fmt.Fprintln(w)
	return nil
}
