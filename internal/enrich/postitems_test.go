package enrich

import (
	"slices"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/smithy"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestItemIndexRank(t *testing.T) {
	t.Parallel()

	post := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return post.AddDate(0, 0, d) }
	item := func(id, sdkID string, at time.Time, notes string, ops ...string) *awssdk.Item {
		return &awssdk.Item{ID: id, Status: items.StatusNew, Service: awssdk.Service{SDKID: sdkID, Module: id},
			Release: awssdk.Release{PublishedAt: at}, Notes: []awssdk.Note{{Text: notes}}, Diff: &smithy.Diff{AddedOperations: ops}}
	}
	its := []*awssdk.Item{
		// The same release notes, released recently and long ago: the recent one ranks first.
		item("kafka-new", "Kafka", day(-10), "Amazon MSK Express brokers now deliver Kafka topic data to Amazon S3 channels."),
		item("kafka-old", "Kafka", day(-200), "Amazon MSK Express brokers now deliver Kafka topic data to Amazon S3 channels."),
		// Matched only through the operation it adds.
		item("ops", "Widget", day(-5), "Minor updates.", "CreateGizmoChannel"),
		// Shares the post's announcement, though its text barely matches.
		item("linked", "Glue", day(-1), "Glue job version auto upgrades with metadata."),
		// Skipped items are left out, however well they match.
		{ID: "skipped", Status: items.StatusSkipped, Release: awssdk.Release{PublishedAt: day(-1)},
			Notes: []awssdk.Note{{Text: "Amazon MSK Express brokers now deliver Kafka topic data to Amazon S3 channels."}}},
		// Released too long after the post.
		item("late", "Kafka", day(10), "Amazon MSK Express brokers now deliver Kafka topic data to Amazon S3 channels."),
	}
	ann := whatsnew.Announcement{URL: "https://aws.amazon.com/about-aws/whats-new/2026/11/glue-auto/", PostedAt: day(-1),
		Headline: "AWS Glue now upgrades job versions automatically", Body: "Glue job version auto upgrades with metadata."}
	x := NewItemIndex(its, whatsnew.NewMatcher([]whatsnew.Announcement{ann}))
	if x.Len() != 5 {
		t.Errorf("Len = %d, want 5", x.Len())
	}

	p := whatsnew.Post{Title: "Deliver Kafka data to Amazon S3 with MSK Express broker channels", PostedAt: post,
		Tags: []string{"blog-posts#category-analytics#amazon-managed-streaming-for-apache-kafka"}}
	got := x.Rank(p, nil, "", 0)
	ids := make([]string, len(got))
	for i, g := range got {
		ids[i] = g.ID
	}
	if len(ids) < 2 || ids[0] != "kafka-new" || ids[1] != "kafka-old" || slices.Contains(ids, "skipped") || slices.Contains(ids, "late") {
		t.Fatalf("Rank = %v", ids)
	}
	if g := got[0]; g.Days != 10 || g.Released != "2026-11-08" || !g.TaggedService || g.Announcement {
		t.Errorf("kafka-new = %+v", g)
	}
	// The old release has the same text score but counts half.
	if got[0].Score != got[1].Score {
		t.Errorf("scores = %v, %v", got[0].Score, got[1].Score)
	}
	if got := x.Rank(p, nil, "", 1); len(got) != 1 {
		t.Errorf("Rank n=1 = %+v", got)
	}

	// Operation names are words: CreateGizmoChannel matches "gizmo channel".
	if got := x.Rank(whatsnew.Post{Title: "Create a gizmo channel", PostedAt: post}, nil, "", 1); len(got) != 1 || got[0].ID != "ops" {
		t.Errorf("Rank by operation = %+v", got)
	}

	// An item matching one of the post's announcements comes first, even below the text floor.
	q := whatsnew.Post{Title: "Deliver Kafka data to Amazon S3 with MSK Express broker channels", PostedAt: post}
	got = x.Rank(q, []string{ann.URL}, "", 0)
	if len(got) == 0 || got[0].ID != "linked" || !got[0].Announcement || got[0].Score >= PostItemsMin {
		t.Errorf("linked = %+v", got)
	}

	if got := (*ItemIndex)(nil).Rank(p, nil, "", 3); got == nil || len(got) != 0 {
		t.Errorf("nil index = %#v", got)
	}
	if got := NewItemIndex(nil, nil).Rank(p, nil, "", 3); got == nil || len(got) != 0 {
		t.Errorf("empty index = %#v", got)
	}
}
