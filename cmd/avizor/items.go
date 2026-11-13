package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/config"
	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/github"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/smithy"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func openItems(dir string) (*items.Store, error) {
	return items.Open(config.ItemsPath(dir))
}

func cmdPull(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	sinceFlag := fs.String("since", "", "pull releases or announcements published since a date, RFC 3339 time, or age (30d); doesn't move the checkpoint")
	noFetch := fs.Bool("no-fetch", false, "don't fetch the models clone first")
	quiet := fs.Bool("quiet", false, "suppress progress messages")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) > 1 {
		return fmt.Errorf("%w: pull takes at most one source name", errUsage)
	}
	var since time.Time
	if *sinceFlag != "" {
		if since, err = parseSince(*sinceFlag, time.Now()); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}

	cfg, dir, err := c.load()
	if err != nil {
		return err
	}
	// With no name, pull every source.
	srcs := make([]*config.Source, 0, len(cfg.Sources))
	if len(pos) == 1 {
		src, err := cfg.Source(pos[0])
		if err != nil {
			return err
		}
		srcs = append(srcs, src)
	} else {
		for i := range cfg.Sources {
			srcs = append(srcs, &cfg.Sources[i])
		}
	}
	st, err := openItems(dir)
	if err != nil {
		return err
	}
	defer st.Close()
	logf := func(format string, args ...any) {
		if !*quiet {
			fmt.Fprintf(stderr, format+"\n", args...)
		}
	}

	var results []any
	for _, src := range srcs {
		var res any
		switch src.Type {
		case config.SourceAWSWhatsNew:
			res, err = pullWhatsNew(ctx, src, st, since, logf)
		default:
			res, err = pullSDK(ctx, src, dir, st, since, *noFetch, logf)
		}
		if err != nil {
			return fmt.Errorf("source %s: %w", src.Name, err)
		}
		results = append(results, res)
		if c.format == "text" {
			switch r := res.(type) {
			case *awssdk.PullResult:
				fmt.Fprintf(stdout, "pulled %s since %s: %d release(s), %d new item(s), %d skipped, in %s\n",
					r.Source, r.Since, r.Releases, r.New, r.Skipped, r.Duration)
				for _, w := range r.Warnings {
					fmt.Fprintf(stdout, "warning: %s\n", w)
				}
			case *whatsNewResult:
				fmt.Fprintf(stdout, "pulled %s since %s: %d announcement(s), in %s\n", r.Source, r.Since, r.Announcements, r.Duration)
			}
		}
	}
	if c.format == "text" {
		return nil
	}
	if len(results) == 1 {
		return writeJSON(stdout, results[0])
	}
	return writeJSON(stdout, results)
}

func pullSDK(ctx context.Context, src *config.Source, dir string, st *items.Store, since time.Time, noFetch bool, logf func(string, ...any)) (*awssdk.PullResult, error) {
	gh, err := github.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	gh.Log = logf
	p := &awssdk.Puller{
		Source:   src,
		Models:   &awssdk.ModelsRepo{Dir: src.ModelsPath(dir), URL: src.ModelsURL, Branch: src.ModelsBranch},
		Releases: gh,
		Tags:     awssdk.GitTags{},
		Store:    st,
		Version:  version,
		Log:      logf,
	}
	res, err := p.Pull(ctx, awssdk.PullOptions{Since: since, NoFetch: noFetch})
	if err != nil && res != nil && res.Releases > 0 {
		logf("partial pull: %d release(s) stored before the error; progress is saved", res.Releases)
	}
	return res, err
}

type whatsNewResult struct {
	Source        string `json:"source"`
	Since         string `json:"since"`
	Announcements int    `json:"announcements"`
	Duration      string `json:"duration"`
}

// whatsNewOverlap is how far before the checkpoint a pull starts again, for posts published late or back-dated.
const whatsNewOverlap = 7 * 24 * time.Hour

func pullWhatsNew(ctx context.Context, src *config.Source, st *items.Store, since time.Time, logf func(string, ...any)) (*whatsNewResult, error) {
	start := time.Now()
	key := "checkpoint:" + src.Name
	explicit := !since.IsZero()
	if !explicit {
		v, err := st.Meta(ctx, key)
		if err != nil {
			return nil, err
		}
		if v != "" {
			cp, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return nil, fmt.Errorf("invalid checkpoint %q: %w", v, err)
			}
			since = cp.Add(-whatsNewOverlap)
		} else if since, err = config.ParseSince(src.Start, start); err != nil {
			return nil, err
		}
	}
	c := &whatsnew.Client{URL: src.URL, HTTP: &http.Client{Timeout: time.Minute}, UserAgent: "avizor/" + version}
	n, err := whatsnew.Pull(ctx, c, items.AnnouncementStore{Store: st, Source: src.Name}, since, logf)
	if err != nil {
		return nil, err
	}
	if !explicit {
		if err := st.SetMeta(ctx, key, start.UTC().Format(time.RFC3339)); err != nil {
			return nil, err
		}
	}
	return &whatsNewResult{Source: src.Name, Since: since.UTC().Format(time.RFC3339), Announcements: n,
		Duration: time.Since(start).Round(time.Millisecond).String()}, nil
}

func cmdItems(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("items", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	status := fs.String("status", items.StatusNew, "status to list: new, skipped, or all")
	service := fs.String("service", "", "only this SDK service module (e.g. fsx)")
	source := fs.String("source", "", "only this source")
	sinceFlag := fs.String("since", "", "only releases published since a date, RFC 3339 time, or age (30d)")
	limit := fs.Int("limit", 50, "maximum items (0 for no limit)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) > 0 {
		return fmt.Errorf("%w: items takes no arguments", errUsage)
	}
	f := items.Filter{Source: *source, Service: *service, Limit: *limit}
	switch *status {
	case "all":
	case items.StatusNew, items.StatusSkipped:
		f.Status = *status
	default:
		return fmt.Errorf("%w: --status must be new, skipped, or all", errUsage)
	}
	if *sinceFlag != "" {
		if f.Since, err = parseSince(*sinceFlag, time.Now()); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}

	_, dir, err := c.load()
	if err != nil {
		return err
	}
	st, err := openItems(dir)
	if err != nil {
		return err
	}
	defer st.Close()

	recs, err := st.List(context.Background(), f)
	if err != nil {
		return err
	}
	list := make([]*awssdk.Item, 0, len(recs))
	for _, r := range recs {
		it, err := decodeItem(r)
		if err != nil {
			return err
		}
		list = append(list, it)
	}
	if c.format != "text" {
		return writeJSON(stdout, list)
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for _, it := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", it.ID, it.Status, it.Summary())
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d item(s)\n", len(list))
	return nil
}

func cmdInspect(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	noEnrich := fs.Bool("no-enrich", false, "show only the item, without relating it to the target")
	noCode := fs.Bool("no-code", false, "don't search the target checkout for operation calls")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) != 1 {
		return fmt.Errorf("%w: inspect takes one item ID", errUsage)
	}

	cfg, dir, err := c.load()
	if err != nil {
		return err
	}
	st, err := openItems(dir)
	if err != nil {
		return err
	}
	defer st.Close()

	r, err := st.Get(ctx, pos[0])
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("no item %q; list items with `avizor items`", pos[0])
	}
	it, err := decodeItem(*r)
	if err != nil {
		return err
	}
	out := inspected{Item: it}
	if !*noEnrich {
		t, err := cfg.Target(c.target)
		if err != nil {
			return err
		}
		out.Enrichment = newEnricher(cfg, dir, t, it, *noCode, stderr).Enrich(ctx, it)
	}
	if c.format != "text" {
		return writeJSON(stdout, out)
	}
	if err := writeItemText(stdout, it); err != nil {
		return err
	}
	if out.Enrichment != nil {
		return writeEnrichmentText(stdout, out.Enrichment)
	}
	return nil
}

// inspected is an item and, optionally, its enrichment for a target.
type inspected struct {
	*awssdk.Item
	Enrichment *enrich.Result `json:"enrichment,omitempty"`
}

// newEnricher builds an enricher for t, reading models from the item's source clone if it exists.
func newEnricher(cfg *config.Config, dir string, t *config.Target, it *awssdk.Item, noCode bool, stderr io.Writer) *enrich.Enricher {
	e := &enrich.Enricher{
		Target:      t.Name,
		Services:    loadServices(t, stderr),
		LabelPrefix: t.ServiceLabelPrefix,
		Checkout:    t.Checkout,
		Ref:         t.Ref,
		NoCode:      noCode,
	}
	if src, err := cfg.Source(it.Source); err == nil {
		if p := src.ModelsPath(dir); dirExists(p) {
			e.Models = &awssdk.ModelsRepo{Dir: p, URL: src.ModelsURL, Branch: src.ModelsBranch}
		}
	}
	e.Announcements = announcements(dir, stderr)
	return e
}

var (
	matchersMu sync.Mutex
	matchers   = map[string]*whatsnew.Matcher{}
)

// announcements returns a matcher over the announcements stored in the state directory, built once per process, or
// nil if there are none.
func announcements(dir string, stderr io.Writer) *whatsnew.Matcher {
	matchersMu.Lock()
	defer matchersMu.Unlock()
	if m, ok := matchers[dir]; ok {
		return m
	}
	var m *whatsnew.Matcher
	st, err := openItems(dir)
	if err == nil {
		var as []whatsnew.Announcement
		if as, err = st.Announcements(context.Background()); err == nil && len(as) > 0 {
			m = whatsnew.NewMatcher(as)
		}
		st.Close()
	}
	if err != nil {
		fmt.Fprintf(stderr, "avizor: warning: announcements unavailable: %v\n", err)
	}
	matchers[dir] = m
	return m
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func writeEnrichmentText(w io.Writer, e *enrich.Result) error {
	fmt.Fprintf(w, "\ntarget %s\n", e.Target)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	p := func(format string, args ...any) { fmt.Fprintf(tw, "  "+format+"\n", args...) }
	svc := e.Service.Status
	if e.Service.Package != "" {
		svc = fmt.Sprintf("%s %s", e.Service.Package, e.Service.Status)
	}
	p("service:\t%s (%s)", svc, e.Service.Basis)
	if a := e.Availability; a != nil {
		p("availability:\t%s: item %s, provider %s", a.Status, cmp.Or(a.ItemVersion, "?"), cmp.Or(a.ProviderVersion, "none"))
		p("go.mod:\t%s", a.Source)
		if b := a.Bump; b != nil {
			pr := ""
			if b.PR != 0 {
				pr = fmt.Sprintf("#%d ", b.PR)
			}
			p("bumped by:\t%s%s %s (%s, %s)", pr, short(b.Commit), b.Subject, b.Version, b.Time.Format(time.DateOnly))
		}
	}
	switch m := e.Announcement; {
	case m != nil:
		p("announcement:\t%s (inferred: %s)", m.Announcement.Headline, m.Basis)
		p("\t%s", m.Announcement.URL)
	case e.AnnouncementsMatched:
		p("announcement:\tnone clearly matches")
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(e.Work) > 0 {
		fmt.Fprintf(w, "\n  work (inferred)\n")
		for _, wk := range e.Work {
			fmt.Fprintf(w, "    %s: %s (%s)\n", wk.Kind, wk.Subject, wk.Basis)
			if len(wk.UsedBy) > 0 {
				fmt.Fprintf(w, "      provider calls these operations in: %s\n", strings.Join(wk.UsedBy, ", "))
			}
			switch {
			case wk.Member != "" && len(wk.FoundIn) > 0:
				fmt.Fprintf(w, "      %s is used in: %s\n", wk.Member, strings.Join(wk.FoundIn, ", "))
			case wk.Member != "":
				fmt.Fprintf(w, "      %s isn't used in those files\n", wk.Member)
			}
		}
	}
	for _, warn := range e.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", warn)
	}
	return nil
}

func decodeItem(r items.Record) (*awssdk.Item, error) {
	var it awssdk.Item
	if err := json.Unmarshal(r.Data, &it); err != nil {
		return nil, fmt.Errorf("decoding item %s: %w", r.ID, err)
	}
	return &it, nil
}

func writeItemText(w io.Writer, it *awssdk.Item) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	p := func(format string, args ...any) { fmt.Fprintf(tw, format+"\n", args...) }
	p("item:\t%s", it.ID)
	p("status:\t%s", it.Status)
	if it.SkipReason != "" {
		p("skip reason:\t%s", it.SkipReason)
	}
	svc := it.Service.Module
	if it.Service.SDKID != "" {
		svc = fmt.Sprintf("%s (%s)", it.Service.SDKID, it.Service.Module)
	}
	p("service:\t%s", svc)
	mod := it.Service.ModulePath
	if it.Service.ModuleVersion != "" {
		mod += " " + it.Service.ModuleVersion
	}
	p("module:\t%s", mod)
	p("release:\t%s (%s)", it.Release.Tag, it.Release.PublishedAt.Format(time.RFC3339))
	p("release url:\t%s", it.Release.URL)
	if it.Release.CommitURL != "" {
		p("release commit:\t%s", it.Release.CommitURL)
	}
	for _, n := range it.Notes {
		p("note:\t%s: %s", n.Kind, n.Text)
	}
	pv := it.Provenance
	if pv.ModelPath != "" {
		p("model:\t%s", pv.ModelPath)
	}
	p("models compared:\t%s %s..%s", pv.ModelsRepo, short(pv.Base.SHA), short(pv.Head.SHA))
	p("pulled:\t%s by %s", pv.PulledAt.Format(time.RFC3339), pv.PulledBy)
	if err := tw.Flush(); err != nil {
		return err
	}

	d := it.Diff
	if d == nil {
		return nil
	}
	section := func(title string, lines []string) {
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(w, "\n%s\n", title)
		for _, l := range lines {
			fmt.Fprintf(w, "  %s\n", l)
		}
	}
	section("new operations", d.AddedOperations)
	section("removed operations", d.RemovedOperations)
	section("new members", members(d.AddedMembers))
	section("removed members", members(d.RemovedMembers))
	section("new enum values", members(d.AddedEnumValues))
	section("removed enum values", members(d.RemovedEnumValues))
	section("changed members (new type)", changes(d.ChangedMembers))
	section("changed enum values", changes(d.ChangedEnumValues))
	section("new shapes", shapes(d.AddedShapes))
	section("removed shapes", shapes(d.RemovedShapes))
	section("retargeted members (same type, not counted as API changes)", changes(d.RetargetedMembers))
	return nil
}

func members(ms []smithy.MemberChange) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		line := m.Shape + "." + m.Member
		if m.Value != "" && m.Value != m.Member {
			line += " (" + m.Value + ")"
		}
		out = append(out, line)
	}
	return out
}

func changes(ms []smithy.MemberChange) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, fmt.Sprintf("%s.%s: %s -> %s", m.Shape, m.Member, m.OldValue, m.Value))
	}
	return out
}

func shapes(ss []smithy.ShapeRef) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Name+" ("+s.Type+")")
	}
	return out
}

func short(sha string) string {
	return sha[:min(len(sha), 10)]
}
