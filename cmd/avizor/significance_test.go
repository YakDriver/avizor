package main

import (
	"slices"
	"testing"

	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestPostSignificance(t *testing.T) {
	t.Parallel()

	ann := []PostAnnouncement{{URL: "https://aws.amazon.com/about-aws/whats-new/2026/11/x/"}}
	launch := []string{whatsnew.LaunchTag}
	for name, tc := range map[string]struct {
		p       whatsnew.Post
		anns    []PostAnnouncement
		sig     bool
		signals []string
		kind    string
	}{
		"launch":       {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/compute/a/", Title: "Introducing A", Tags: launch}, sig: true, signals: []string{SignalLaunch}},
		"news blog":    {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/a/", Title: "A"}, sig: true, signals: []string{SignalNewsBlog}},
		"announcement": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/database/a/", Title: "How to use A"}, anns: ann, sig: true, signals: []string{SignalAnnouncement}},
		"no signals":   {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/database/a/", Title: "How to use A"}},
		"all signals": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/a/", Title: "A", Tags: launch}, anns: ann, sig: true,
			signals: []string{SignalLaunch, SignalNewsBlog, SignalAnnouncement}},
		"weekly roundup": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/r/", Title: "AWS Weekly Roundup: A, B, and more", Tags: launch},
			signals: []string{SignalLaunch, SignalNewsBlog}, kind: KindRoundup},
		"monthly roundup": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/r/", Title: "ICYMI: What landed in October", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindRoundup},
		"event tag": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/security/e/", Title: "Sessions you won't miss", Tags: append(slices.Clone(launch), tagEvents)},
			signals: []string{SignalLaunch}, kind: KindEvent},
		"event title": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/storage/e/", Title: "Your guide to storage at re:Invent 2026", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindEvent},
		"developer tools tag": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/d/", Title: "AWS Emulator", Tags: append(slices.Clone(launch), tagDevTools)},
			signals: []string{SignalLaunch, SignalNewsBlog}, kind: KindDeveloperTools},
		"developer blog": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/developer/d/", Title: "Presigned URL downloads"}, anns: ann,
			signals: []string{SignalAnnouncement}, kind: KindDeveloperTools},
		"customer story": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/c/", Title: "How A uses B", Tags: []string{tagCustomerSolution}}, anns: ann,
			signals: []string{SignalAnnouncement}, kind: KindCustomerStory},
		"partner blog": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/apn/c/", Title: "A launches B"}, anns: ann,
			signals: []string{SignalAnnouncement}, kind: KindCustomerStory},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := postSignificance(tc.p, tc.anns)
			if got.Significant != tc.sig || !slices.Equal(got.Signals, tc.signals) || got.Kind != tc.kind {
				t.Errorf("got %+v, want significant %t, signals %v, kind %q", got, tc.sig, tc.signals, tc.kind)
			}
		})
	}
}

func TestPostCounts(t *testing.T) {
	t.Parallel()

	c := PostCounts{Counts: map[string]int{}}
	for _, s := range []string{PostOpen, PostCovered, PostCovered, PostNoWork, PostDeferred, PostCovered} {
		c.add(s)
	}
	// Coverage leaves out no-work and deferred posts.
	if c.Posts != 6 || c.Counts[PostCovered] != 3 || c.Coverage != 0.75 {
		t.Errorf("counts = %+v", c)
	}
}
