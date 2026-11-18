package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/index"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/queue"
	"github.com/YakDriver/avizor/internal/reconcile"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestServiceMentions(t *testing.T) {
	t.Parallel()

	if got := serviceWords("Amazon SageMaker"); !slices.Equal(got, []string{"sage", "maker"}) {
		t.Errorf("serviceWords = %v", got)
	}
	dsql := serviceWords("DSQL")
	for _, tc := range []struct {
		a    whatsnew.Announcement
		want bool
	}{
		{whatsnew.Announcement{Headline: "Amazon Aurora DSQL now offers SQL statement statistics"}, true},
		{whatsnew.Announcement{Headline: "Statement statistics", Body: "<p>Aurora <b>DSQL</b> now</p>"}, true},
		{whatsnew.Announcement{Headline: "Amazon Aurora PostgreSQL adds Graviton5"}, false},
	} {
		if got := mentions(tc.a, dsql); got != tc.want {
			t.Errorf("mentions(%q) = %t, want %t", tc.a.Headline, got, tc.want)
		}
	}
	if mentions(whatsnew.Announcement{Headline: "Amazon anything"}, serviceWords("Amazon")) {
		t.Error("a service name of only generic words mentions everything")
	}
}

func TestBriefDecide(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	closed := map[int]*index.Issue{7: {Kind: index.KindPR, State: "closed"}}
	lookup := func(n int) *index.Issue { return closed[n] }

	ev := queue.Evidence{CodeSearched: true, CodeCandidates: true, State: queue.Pending, Availability: "pending"}
	evJSON, _ := json.Marshal(ev)
	b := Brief{State: queue.Pending, Queue: queue.Pending, entry: queue.Entry{State: queue.Pending, Evidence: ev}}
	b.decide(items.Disposition{Decision: items.DecisionCreated, Number: 5}, lookup, now)
	if b.Queue != queue.Decided || b.Decision == nil || b.Resurfaced != "" {
		t.Errorf("created: queue %s, resurfaced %q", b.Queue, b.Resurfaced)
	}
	// A skipped item with the same evidence stays decided; the entry's evidence, not just its state, is compared.
	b = Brief{State: queue.Pending, Queue: queue.Pending, entry: queue.Entry{State: queue.Pending, Evidence: ev}}
	b.decide(items.Disposition{Decision: items.DecisionSkipped, Reason: "x", Evidence: evJSON}, lookup, now)
	if b.Queue != queue.Decided || b.Resurfaced != "" {
		t.Errorf("skipped with the same evidence: queue %s, resurfaced %q", b.Queue, b.Resurfaced)
	}
	b = Brief{State: queue.Gap, Queue: queue.Gap}
	b.decide(items.Disposition{Decision: items.DecisionDuplicate, Number: 7}, lookup, now)
	if b.Queue != queue.Resurfaced || !strings.Contains(b.Resurfaced, "#7 was closed unmerged") {
		t.Errorf("duplicate of a closed PR: queue %s, resurfaced %q", b.Queue, b.Resurfaced)
	}
}

func TestWriteBriefText(t *testing.T) {
	t.Parallel()

	b := Brief{
		Item: "aws-sdk-go-v2/dsql/release-2026-11-16", Published: "2026-11-16", Module: "github.com/aws/aws-sdk-go-v2/service/dsql",
		Version: "v1.23.0", Availability: &enrich.Availability{Status: enrich.Pending, ProviderVersion: "v1.22.6"},
		State: queue.Pending, Reason: "needs v1.23.0", StateIfAvailable: queue.Gap, Queue: queue.Pending,
		Announcements: []BriefAnnouncement{{Headline: "Aurora DSQL now offers SQL statement statistics", URL: "u", Days: 0.9, Score: 0.38, MentionsService: true}},
	}
	// Five confident candidates and seven related: all confident ones and five related are shown.
	for i := range 5 {
		b.Candidates = append(b.Candidates, reconcile.Candidate{Category: reconcile.CategoryTracked, Kind: reconcile.KindIssue, Number: 100 + i, Title: fmt.Sprintf("tracked %d", i)})
	}
	for i := range 7 {
		b.Candidates = append(b.Candidates, reconcile.Candidate{Category: reconcile.CategoryRelated, Kind: reconcile.KindIssue, Number: 200 + i, Title: fmt.Sprintf("related %d", i)})
	}
	var buf bytes.Buffer
	writeBriefText(&buf, b)
	out := buf.String()
	for _, want := range []string{
		"availability: pending (target requires v1.22.6)",
		"state:        pending: needs v1.23.0 (gap once the SDK version is available)",
		"0.38   +0.9d  Aurora DSQL now offers SQL statement statistics [names the service]",
		"tracked 4", "related 4",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text has no %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "related 5") {
		t.Errorf("text shows more than 5 related candidates:\n%s", out)
	}
}
