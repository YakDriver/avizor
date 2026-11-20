package main

import (
	"slices"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestLinkPosts(t *testing.T) {
	t.Parallel()

	rel := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	ann := whatsnew.Announcement{URL: "wn-insights", Headline: "Amazon GameLift Servers now supports game session placement insights",
		PostedAt: rel.Add(day)}
	anns := whatsnew.NewMatcher([]whatsnew.Announcement{ann,
		{URL: "wn-streams", Headline: "Amazon GameLift Streams adds instant play demos", PostedAt: rel}})
	ps := []whatsnew.Post{
		{URL: "b-troubleshoot", Title: "Troubleshoot game session placement with Amazon GameLift Servers insights", PostedAt: rel.Add(2 * day)},
		// A customer story about the same launch isn't linked.
		{URL: "b-story", Title: "Studio X on Amazon GameLift Servers game session placement insights support", PostedAt: rel.Add(day),
			Tags: []string{tagCustomerSolution}},
		{URL: "b-queues", Title: "Game session placement insights for Amazon GameLift Servers queues", PostedAt: rel.Add(4 * day)},
		{URL: "b-deep", Title: "Deep dive: Amazon GameLift Servers game session placement insights", PostedAt: rel.Add(5 * day)},
		// A post about another announcement isn't linked.
		{URL: "b-streams", Title: "Amazon GameLift Streams adds instant play demos", PostedAt: rel.Add(day)},
	}
	byURL := map[string]*whatsnew.Post{}
	for i := range ps {
		byURL[ps[i].URL] = &ps[i]
	}
	it := &awssdk.Item{Service: awssdk.Service{SDKID: "GameLift", Module: "gamelift"}, Release: awssdk.Release{PublishedAt: rel},
		Notes: []awssdk.Note{{Text: "Amazon GameLift Servers adds game session placement insights."}}}
	got := linkPosts(whatsnew.NewMatcher(whatsnew.PostsAsAnnouncements(ps)), anns, it, &whatsnew.Match{Announcement: ann},
		func(u string) *whatsnew.Post { return byURL[u] })

	var urls []string
	for _, p := range got {
		urls = append(urls, p.Post.URL)
		if p.Basis != "matches the item's announcement, wn-insights" {
			t.Errorf("basis = %q", p.Basis)
		}
	}
	// At most two, best first.
	if len(urls) != announcedPostsMax || slices.Contains(urls, "b-story") || slices.Contains(urls, "b-streams") {
		t.Errorf("linked posts = %q; want two insights posts, without the customer story or the streams post", urls)
	}
}
