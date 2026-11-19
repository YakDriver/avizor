package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestPostAnnouncements(t *testing.T) {
	t.Parallel()

	posted := time.Date(2026, 11, 5, 16, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	m := whatsnew.NewMatcher([]whatsnew.Announcement{
		{URL: "u-posture", Headline: "AWS Client VPN now supports device posture assessment", PostedAt: posted.Add(-31 * day),
			Body: "Client VPN evaluates device posture from CrowdStrike, Jamf, or JumpCloud with a Cedar policy."},
		{URL: "u-old", Headline: "AWS Client VPN now supports device posture assessment checks", PostedAt: posted.Add(-46 * day)},
		{URL: "u-other", Headline: "Amazon S3 adds annotations", PostedAt: posted.Add(-day)},
		// Ranked first, but region expansions aren't matched. Three of them fill what Rank would return for 3.
		{URL: "u-region", Headline: "AWS Client VPN device posture assessment with CrowdStrike, Jamf, and JumpCloud is now available in more Regions",
			PostedAt: posted.Add(-2 * day), Body: "Register a device trust provider, write a Cedar policy, and Client VPN evaluates every connection."},
		{URL: "u-region2", Headline: "AWS Client VPN device posture assessment with CrowdStrike, Jamf, and JumpCloud is now available in additional Regions",
			PostedAt: posted.Add(-2 * day), Body: "Register a device trust provider, write a Cedar policy, and Client VPN evaluates every connection."},
		{URL: "u-region3", Headline: "AWS Client VPN device posture assessment with CrowdStrike, Jamf, and JumpCloud expands to more Regions",
			PostedAt: posted.Add(-2 * day), Body: "Register a device trust provider, write a Cedar policy, and Client VPN evaluates every connection."},
		// Unrelated announcements, so the words they don't share weigh more.
		{URL: "u-f1", Headline: "Amazon EC2 adds instance types", PostedAt: posted.Add(-3 * day)},
		{URL: "u-f2", Headline: "Amazon RDS supports a new engine version", PostedAt: posted.Add(-4 * day)},
		{URL: "u-f3", Headline: "AWS Lambda adds a runtime", PostedAt: posted.Add(-5 * day)},
		{URL: "u-f4", Headline: "Amazon DynamoDB adds bulk loading", PostedAt: posted.Add(-6 * day)},
	})
	p := whatsnew.Post{URL: "u-post", Title: "Introducing device posture assessment for AWS Client VPN", PostedAt: posted,
		Excerpt: "Register a device trust provider (CrowdStrike, Jamf, or JumpCloud), write a Cedar policy, and Client VPN evaluates every connection."}
	got := postAnnouncements(m, p)
	if len(got) != 1 || got[0].URL != "u-posture" || got[0].Days != -31 {
		t.Errorf("announcements = %+v; want only u-posture, 31 days before (u-old is outside 45 days, u-other doesn't match, u-region is a region expansion)", got)
	}
	if got := postAnnouncements(nil, p); got == nil || len(got) != 0 {
		t.Errorf("announcements without a matcher = %#v; want empty", got)
	}
	if text := postText(p, got); !strings.HasPrefix(text, p.Title+". Register") || !strings.HasSuffix(text, "\nu-post\nu-posture") {
		t.Errorf("postText = %q", text)
	}
}

func TestPostEntry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	p := whatsnew.Post{URL: newsBlogPrefix + "x/", Title: "X", PostedAt: now.Add(-time.Hour), Tags: []string{whatsnew.LaunchTag}}
	strong := []reconcile.Match{{Type: "url", Value: "u", Strength: reconcile.StrengthStrong}}
	res := &reconcile.Result{Candidates: []reconcile.Candidate{
		{Category: reconcile.CategoryTracked, Kind: reconcile.KindIssue, Number: 5, Signals: strong},
		{Category: reconcile.CategoryConsidered, Kind: reconcile.KindIssue, Number: 6, Signals: strong},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindPR, Number: 7},
		{Category: reconcile.CategoryImplemented, Kind: reconcile.KindCode, Path: "internal/x.go"},
		{Category: reconcile.CategoryRelated, Kind: reconcile.KindIssue, Number: 8},
	}}
	none := &reconcile.Result{}
	for _, tc := range []struct {
		name    string
		res     *reconcile.Result
		d       *items.Disposition
		state   string
		numbers []int
		revisit bool
	}{
		{"tracked candidate", res, nil, PostCovered, []int{5}, false},
		{"nothing", none, nil, PostOpen, nil, false},
		{"created", none, &items.Disposition{Decision: items.DecisionCreated, Number: 9, Also: []int{10}}, PostCovered, []int{9, 10}, false},
		{"duplicate over candidates", res, &items.Disposition{Decision: items.DecisionDuplicate, Number: 11}, PostCovered, []int{11}, false},
		{"skipped", res, &items.Disposition{Decision: items.DecisionSkipped, Reason: "no API"}, PostNoWork, nil, false},
		{"deferred", none, &items.Disposition{Decision: items.DecisionDeferred, RevisitAfter: now.Add(time.Hour)}, PostDeferred, nil, false},
		{"deferred and due", none, &items.Disposition{Decision: items.DecisionDeferred, RevisitAfter: now.Add(-time.Hour)}, PostOpen, nil, true},
		{"deferred and due, covered", res, &items.Disposition{Decision: items.DecisionDeferred, RevisitAfter: now.Add(-time.Hour)}, PostCovered, []int{5}, false},
		{"skipped and due", none, &items.Disposition{Decision: items.DecisionSkipped, RevisitAfter: now.Add(-time.Hour)}, PostOpen, nil, true},
		{"created and due", res, &items.Disposition{Decision: items.DecisionCreated, Number: 9, RevisitAfter: now.Add(-time.Hour)}, PostCovered, []int{5}, false},
	} {
		e := postEntry(p, nil, tc.res, tc.d, 2, now)
		if e.State != tc.state || !slices.Equal(e.Numbers, tc.numbers) || e.Revisit != tc.revisit {
			t.Errorf("%s: state %s, numbers %v, revisit %t; want %s, %v, %t", tc.name, e.State, e.Numbers, e.Revisit, tc.state, tc.numbers, tc.revisit)
		}
	}
	e := postEntry(p, nil, res, nil, 2, now)
	if !e.Launch || !e.NewsBlog || e.Posted != "2026-11-18" {
		t.Errorf("flags = launch %t, news %t, posted %s", e.Launch, e.NewsBlog, e.Posted)
	}
	if len(e.Covering) != 1 || !slices.Equal(e.Covering[0].Signals, []string{"url:u"}) {
		t.Errorf("covering = %+v", e.Covering)
	}
	// A closed issue isn't coverage, and code candidates aren't listed; related stops at the limit.
	var rel []int
	for _, c := range e.Related {
		rel = append(rel, c.Number)
	}
	if !slices.Equal(rel, []int{6, 7}) {
		t.Errorf("related = %v, want [6 7]", rel)
	}
}

func TestPostsInWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	since := now.AddDate(0, 0, -30)
	ps := []whatsnew.Post{
		{URL: "new", PostedAt: now.AddDate(0, 0, -1)},
		{URL: "new-launch", PostedAt: now.AddDate(0, 0, -1), Tags: []string{whatsnew.LaunchTag}},
		{URL: "old", PostedAt: now.AddDate(0, 0, -40)},
		{URL: "old-due", PostedAt: now.AddDate(0, 0, -40)},
		{URL: "old-later", PostedAt: now.AddDate(0, 0, -40)},
	}
	decisions := map[string]items.Disposition{
		"old-due":   {Decision: items.DecisionDeferred, RevisitAfter: now.Add(-time.Hour)},
		"old-later": {Decision: items.DecisionDeferred, RevisitAfter: now.Add(time.Hour)},
	}
	urls := func(ps []whatsnew.Post) []string {
		var out []string
		for _, p := range ps {
			out = append(out, p.URL)
		}
		return out
	}
	got, early := postsInWindow(slices.Clone(ps), decisions, since, false, now)
	if !slices.Equal(urls(got), []string{"new", "new-launch", "old-due"}) || len(early) != 1 || !early["old-due"] {
		t.Errorf("window = %v, early %v", urls(got), early)
	}
	if got, _ := postsInWindow(slices.Clone(ps), decisions, since, true, now); !slices.Equal(urls(got), []string{"new-launch"}) {
		t.Errorf("launch window = %v", urls(got))
	}
}

func TestWritePostsText(t *testing.T) {
	t.Parallel()

	res := PostsResult{
		Posts: []PostEntry{
			{Posted: "2026-11-18", State: PostOpen, Launch: true, NewsBlog: true, Title: "A launch",
				Related: []PostCandidate{{Number: 5, Kind: reconcile.KindIssue}, {Number: 6, Kind: reconcile.KindPR}},
				Items:   []enrich.PostItem{{ID: "aws-sdk-go-v2/kafka/release-2026-07-30"}, {ID: "aws-sdk-go-v2/kafka/release-2026-08-24"}}},
			{Posted: "2026-11-18", State: PostOpen, Title: "Items only", Items: []enrich.PostItem{{ID: "aws-sdk-go-v2/glue/release-2026-11-05"}}},
			{Posted: "2026-11-17", State: PostOpen, Title: "Deferred", Revisit: true,
				Decision: &items.Disposition{Decision: items.DecisionDeferred}},
			{Posted: "2026-11-16", State: PostCovered, Title: "Covered", URL: "https://aws.amazon.com/blogs/x/covered/", Numbers: []int{7, 8}},
			{Posted: "2026-11-15", State: PostNoWork, Title: "No work", byDecision: true,
				Decision: &items.Disposition{Decision: items.DecisionSkipped, Reason: "customer story"}},
		},
		Summary: PostsSummary{PostCounts: PostCounts{Posts: 10, Counts: map[string]int{PostOpen: 2, PostCovered: 6, PostNoWork: 2}, Coverage: 0.75},
			Significant: PostCounts{Posts: 5, Counts: map[string]int{PostOpen: 1, PostCovered: 4}, Coverage: 0.8}},
	}
	res.Posts[0].Significant = true
	var b bytes.Buffer
	if err := writePostsText(&b, res, func(s string) bool { return s == PostOpen }, false); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"news,launch,sig", "related #5, #6 (PR); items aws-sdk-go-v2/kafka/release-2026-07-30, aws-sdk-go-v2/kafka/release-2026-08-24\n",
		"  items aws-sdk-go-v2/glue/release-2026-11-05\n", "deferred; revisit due", "https://aws.amazon.com/blogs/x/covered/", "covered by #7, #8", "skipped (customer story)",
		"5 of 10 post(s) shown; open 2; hidden: covered 6, no work 2; coverage 75% (6 of 8)\nsignificant: 5 post(s), open 1, covered 4; coverage 80% (4 of 5)\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
