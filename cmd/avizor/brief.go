package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/draft"
	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/gitcmd"
	"github.com/YakDriver/avizor/internal/index"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/queue"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

// Brief is everything needed to decide an item, in one place: the release, availability, the queue state, the
// inferred work with its spec and provider facts, the candidates, and the announcements posted near the release,
// ranked, for the reader to pick from.
type Brief struct {
	Item      string         `json:"item"`
	Service   string         `json:"service"`
	SDKID     string         `json:"sdk_id,omitempty"`
	Release   awssdk.Release `json:"release"`
	Notes     []string       `json:"notes"`
	Module    string         `json:"module"`
	Version   string         `json:"version,omitempty"`
	Published string         `json:"published"`
	// Availability is whether the target's go.mod requires the item's SDK version. It's reported apart from the
	// state: StateIfAvailable is the state the item would have if it did.
	Availability     *enrich.Availability `json:"availability,omitempty"`
	State            string               `json:"state"`
	Reason           string               `json:"reason"`
	StateIfAvailable string               `json:"state_if_available"`
	// Queue is where the queue lists the item: its state, or decided or resurfaced when it has a decision.
	Queue string `json:"queue"`
	// Decision is the item's current decision; Resurfaced says why it needs another look.
	Decision   *items.Disposition `json:"decision,omitempty"`
	Resurfaced string             `json:"resurfaced,omitempty"`
	// Title and Labels are the draft's.
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	// Service package facts from the target.
	ServiceMatch enrich.ServiceMatch   `json:"service_match"`
	Work         []enrich.Work         `json:"work"`
	Candidates   []reconcile.Candidate `json:"candidates"`
	// Announcement is the matcher's pick, if it made one. Announcements are every post within the window, best first.
	Announcement  *whatsnew.Match     `json:"announcement,omitempty"`
	Announcements []BriefAnnouncement `json:"announcements"`
	// BlogPosts are the blog posts created from 3 days before the release to 21 days after, best first. An issue
	// with a related post should link it; the reader picks.
	BlogPosts []BriefPost `json:"blog_posts"`
	// Warnings are the enrichment's and reconciliation's, such as a failed model load, an unsynced index, or a
	// limited search, and a note when no announcements were loaded.
	Warnings []string `json:"warnings,omitempty"`

	// entry is the queue entry, with the evidence a decision is compared against.
	entry queue.Entry
}

// BriefAnnouncement is a post near the release, scored against the item. Matched marks the matcher's pick.
// MentionsService is set when the post's text has every word of the service's name.
type BriefAnnouncement struct {
	Headline        string  `json:"headline"`
	URL             string  `json:"url"`
	Posted          string  `json:"posted"`
	Days            float64 `json:"days"`
	Score           float64 `json:"score"`
	Matched         bool    `json:"matched,omitempty"`
	MentionsService bool    `json:"mentions_service,omitempty"`
	// RegionExpansion is set for a post about availability in more Regions, which the matcher never picks.
	RegionExpansion bool `json:"region_expansion,omitempty"`
}

// maxRelatedText is how many related candidates text output shows; JSON has all.
const maxRelatedText = 5

// BriefPost is a blog post near the release, scored against the item and the matched announcement's headline. Launch
// is set for a post tagged as a launch announcement, and TaggedService for one whose product tags name the service.
type BriefPost struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Posted        string  `json:"posted"`
	Days          float64 `json:"days"`
	Score         float64 `json:"score"`
	Launch        bool    `json:"launch,omitempty"`
	TaggedService bool    `json:"tagged_service,omitempty"`
}

// Blog posts often come days after the release, so brief looks further after it than for announcements.
const (
	blogBefore = 3 * 24 * time.Hour
	blogAfter  = 21 * 24 * time.Hour
)

// briefWindow is how far before and after a release brief looks for announcements.
const briefWindow = 7 * 24 * time.Hour

func cmdBrief(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("brief", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	noCode := fs.Bool("no-code", false, "don't search the target checkout")
	maxAnns := fs.Int("announcements", 10, "maximum announcements per item, best first; 0 for all in the window")
	maxPosts := fs.Int("blog-posts", 5, "maximum blog posts per item, best first; 0 for all in the window")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) == 0 {
		return fmt.Errorf("%w: brief takes one or more item IDs", errUsage)
	}
	if *maxAnns < 0 || *maxPosts < 0 {
		return fmt.Errorf("%w: --announcements and --blog-posts must be 0 or more", errUsage)
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
	defer ist.Close()
	decisions, err := ist.Current(ctx, t.Name)
	if err != nil {
		return err
	}
	var recs []items.Record
	for _, id := range pos {
		r, err := ist.Get(ctx, id)
		if err != nil {
			return err
		}
		if r == nil {
			return fmt.Errorf("no item %q; list items with `avizor items`", id)
		}
		recs = append(recs, *r)
	}

	svcs := loadServices(t, stderr)
	// As for queue: without the service map, every service is unknown and would be reported unimplemented.
	switch {
	case t.Checkout == "":
		return fmt.Errorf("avizor brief needs target %q's checkout for its service map; set checkout in the config", t.Name)
	case svcs == nil:
		return fmt.Errorf("avizor brief needs the service map %s", t.NamesDataPath())
	}
	git := &gitcmd.Cache{}
	r := &reconcile.Reconciler{Store: st, Target: t, Services: svcs, Git: git,
		Options: reconcile.Options{NoFetch: true, NoCode: *noCode, Version: version}}
	c.operations(ctx, t, r, stderr)
	opts := draft.Options{Repo: t.Repo, Checkout: t.Checkout, Prefix: t.TypePrefix, LabelPrefix: t.ServiceLabelPrefix,
		Template: t.DraftTemplate}
	lookup := func(n int) *index.Issue {
		is, err := st.Issue(ctx, n)
		if err != nil {
			return nil
		}
		return is
	}
	enrichers := map[string]*enrich.Enricher{}
	var out []Brief
	for _, rec := range recs {
		it, err := decodeItem(rec)
		if err != nil {
			return err
		}
		e := enrichers[it.Source]
		if e == nil {
			e = queueEnricher(c.cfg, c.dir, t, svcs, git, it.Source, *noCode)
			enrichers[it.Source] = e
		}
		b, err := brief(ctx, it, e, r, opts, *maxAnns)
		if err == nil {
			b.rankPosts(it, blogPosts(c.dir, stderr), *maxPosts)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", it.ID, err)
		}
		if d, ok := decisions[it.ID]; ok {
			b.decide(d, lookup, time.Now())
		}
		out = append(out, b)
	}
	if c.format != "text" {
		return writeJSON(stdout, out)
	}
	for i, b := range out {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		writeBriefText(stdout, b)
	}
	return nil
}

// brief enriches, reconciles, drafts, and classifies an item as the queue does, and ranks the announcements near it.
func brief(ctx context.Context, it *awssdk.Item, e *enrich.Enricher, r *reconcile.Reconciler, opts draft.Options, maxAnns int) (Brief, error) {
	en := e.Enrich(ctx, it)
	rec, err := r.RunItem(ctx, enrich.ReconcileItem(it, en))
	if err != nil {
		return Brief{}, err
	}
	texts, err := candidateTexts(ctx, r.Store, rec)
	if err != nil {
		return Brief{}, err
	}
	d, err := queue.Draft(it, en, rec, texts, opts)
	if err != nil {
		return Brief{}, err
	}
	q := queue.Classify(it, en, rec, d, texts)
	b := Brief{
		Item: it.ID, Service: q.Service, SDKID: it.Service.SDKID, Release: it.Release, Module: it.Service.ModulePath,
		Version: it.Service.ModuleVersion, Published: q.Published, Availability: en.Availability, State: q.State, Reason: q.Reason,
		StateIfAvailable: q.State, Queue: q.State, Title: q.Title, Labels: q.Labels, ServiceMatch: en.Service, Work: en.Work,
		Candidates: rec.Candidates, Announcement: en.Announcement, Announcements: []BriefAnnouncement{}, BlogPosts: []BriefPost{}, entry: q,
		Warnings: slices.Concat(en.Warnings, rec.Warnings),
	}
	if b.State == queue.Pending {
		// Pending is a gap waiting on the SDK version; Classify checks every other state first.
		b.StateIfAvailable = queue.Gap
	}
	for _, n := range it.Notes {
		b.Notes = append(b.Notes, n.Text)
	}
	if b.Candidates == nil {
		b.Candidates = []reconcile.Candidate{}
	}
	if e.Announcements == nil {
		b.Warnings = append(b.Warnings, "no announcements loaded, so none are listed; run `avizor pull aws-whats-new`")
	} else {
		text, service := enrich.AnnouncementQuery(it)
		svcWords := serviceWords(service)
		for _, a := range e.Announcements.Rank(text, service, it.Release.PublishedAt, briefWindow, briefWindow, maxAnns) {
			ba := BriefAnnouncement{Headline: a.Announcement.Headline, URL: a.Announcement.URL,
				Posted: a.Announcement.PostedAt.Format(time.DateOnly), Days: a.Days, Score: a.Score,
				Matched:         en.Announcement != nil && en.Announcement.Announcement.URL == a.Announcement.URL,
				MentionsService: mentions(a.Announcement, svcWords), RegionExpansion: a.RegionExpansion}
			b.Announcements = append(b.Announcements, ba)
		}
	}
	return b, nil
}

// rankPosts lists the blog posts near the release, scored against the item's text and the matched announcement's
// headline.
func (b *Brief) rankPosts(it *awssdk.Item, posts *whatsnew.Matcher, n int) {
	if posts == nil {
		b.Warnings = append(b.Warnings, "no blog posts loaded, so none are listed; run `avizor pull aws-blogs`")
		return
	}
	text, service := enrich.AnnouncementQuery(it)
	if b.Announcement != nil {
		text += "\n" + b.Announcement.Announcement.Headline
	}
	key := strings.Join(serviceWords(service), "")
	for _, r := range posts.Rank(text, service, it.Release.PublishedAt, blogBefore, blogAfter, n) {
		p := BriefPost{Title: r.Announcement.Headline, URL: r.Announcement.URL, Posted: r.Announcement.PostedAt.Format(time.DateOnly),
			Days: r.Days, Score: r.Score, Launch: slices.Contains(r.Announcement.Tags, whatsnew.LaunchTag),
			TaggedService: taggedService(r.Announcement.Tags, key)}
		b.BlogPosts = append(b.BlogPosts, p)
	}
}

// taggedService reports whether a blog post's product category tags name a service, by its identifying words run
// together: "dsql" in blog-posts#category-database-amazon-aurora#dsql, "sagemaker" in ...#amazon-sagemaker-ai.
func taggedService(tags []string, key string) bool {
	if key == "" {
		return false
	}
	return slices.ContainsFunc(tags, func(t string) bool {
		if !strings.HasPrefix(t, "blog-posts#category") {
			return false
		}
		leaf := t[strings.LastIndex(t, "#")+1:]
		return strings.Contains(strings.ReplaceAll(leaf, "-", ""), key)
	})
}

// decide attaches a decision as the queue does, comparing it with the entry's evidence, which can put the item in
// the decided or resurfaced group.
func (b *Brief) decide(d items.Disposition, lookup queue.IssueLookup, now time.Time) {
	e := b.entry
	queue.Resurface(&e, d, lookup, now)
	b.Decision, b.Resurfaced, b.Queue = e.Decision, e.Resurfaced, e.Group()
}

func cmpOr(s, dflt string) string {
	if s != "" {
		return s
	}
	return dflt
}

// genericWords are words of service names that don't identify the service.
var genericWords = []string{"amazon", "aws", "service", "services"}

// serviceWords returns the words of a service name that identify it: "SageMaker" → sage, maker.
func serviceWords(service string) []string {
	return slices.DeleteFunc(whatsnew.Tokens(service), func(w string) bool { return slices.Contains(genericWords, w) })
}

// mentions reports whether an announcement's headline or body has every one of words.
func mentions(a whatsnew.Announcement, words []string) bool {
	if len(words) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, w := range whatsnew.Tokens(a.Headline + " " + a.Body) {
		have[w] = true
	}
	return !slices.ContainsFunc(words, func(w string) bool { return !have[w] })
}

func writeBriefText(w io.Writer, b Brief) {
	p := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }
	p("item:         %s", b.Item)
	p("release:      %s (%s) %s", b.Release.Tag, b.Published, b.Release.URL)
	for _, n := range b.Notes {
		p("note:         %s", n)
	}
	p("module:       %s %s", b.Module, b.Version)
	switch a := b.Availability; {
	case a == nil:
		p("availability: unknown")
	case a.ProviderVersion != "":
		p("availability: %s (target requires %s)", a.Status, a.ProviderVersion)
	default:
		p("availability: %s", a.Status)
	}
	p("service:      %s (%s)", b.ServiceMatch.Status, cmpOr(b.ServiceMatch.Package, b.ServiceMatch.Basis))
	if b.StateIfAvailable != b.State {
		p("state:        %s: %s (%s once the SDK version is available)", b.State, b.Reason, b.StateIfAvailable)
	} else {
		p("state:        %s: %s", b.State, b.Reason)
	}
	if d := b.Decision; d != nil {
		line := decisionText(*d)
		if b.Resurfaced != "" {
			line += "; resurfaced: " + b.Resurfaced
		}
		p("decision:     %s (queue: %s)", line, b.Queue)
	}
	p("draft title:  %s [%s]", b.Title, strings.Join(b.Labels, ", "))

	var shown []reconcile.Candidate
	related := 0
	for _, c := range b.Candidates {
		if c.Category == reconcile.CategoryRelated {
			if related == maxRelatedText {
				continue
			}
			related++
		}
		shown = append(shown, c)
	}
	if len(shown) > 0 {
		p("\ncandidates")
		for _, c := range shown {
			ref := c.Path
			if c.Number != 0 {
				ref = fmt.Sprintf("#%d %s %s", c.Number, stateLabel(c), c.Title)
			}
			p("  %-9s %s", c.Category, ref)
			p("            %s", matches(c.Signals))
		}
	}
	p("\nannouncements (posted within 7 days of the release, best first)")
	if len(b.Announcements) == 0 {
		p("  none")
	}
	for _, a := range b.Announcements {
		var flags []string
		if a.Matched {
			flags = append(flags, "matched")
		}
		if a.MentionsService {
			flags = append(flags, "names the service")
		}
		if a.RegionExpansion {
			flags = append(flags, "region expansion")
		}
		tail := ""
		if len(flags) > 0 {
			tail = " [" + strings.Join(flags, ", ") + "]"
		}
		p("  %.2f  %+5.1fd  %s%s", a.Score, a.Days, a.Headline, tail)
		p("               %s", a.URL)
	}
	p("\nblog posts (created 3 days before to 21 days after the release, best first)")
	if len(b.BlogPosts) == 0 {
		p("  none")
	}
	for _, bp := range b.BlogPosts {
		var flags []string
		if bp.Launch {
			flags = append(flags, "launch")
		}
		if bp.TaggedService {
			flags = append(flags, "tagged with the service")
		}
		tail := ""
		if len(flags) > 0 {
			tail = " [" + strings.Join(flags, ", ") + "]"
		}
		p("  %.2f  %+5.1fd  %s%s", bp.Score, bp.Days, bp.Title, tail)
		p("               %s", bp.URL)
	}
	if len(b.Work) > 0 {
		p("\nwork (inferred)")
		writeWorkText(w, b.Work)
	}
	for _, warn := range b.Warnings {
		p("warning: %s", warn)
	}
}
