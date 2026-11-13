package whatsnew

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/radar/internal/config"
)

func TestDefaultURL(t *testing.T) {
	t.Parallel()

	if DefaultURL != config.DefaultWhatsNewURL {
		t.Errorf("DefaultURL and config.DefaultWhatsNewURL differ")
	}
}

func TestTokens(t *testing.T) {
	t.Parallel()

	got := Tokens(`<p>AWS KMS now tracks GetKeyLastUsage &amp; the S3 bucket</p>`)
	want := []string{"kms", "tracks", "get", "key", "last", "usage", "s3", "bucket"}
	if !slices.Equal(got, want) {
		t.Errorf("Tokens = %q, want %q", got, want)
	}
}

func TestRegionExpansion(t *testing.T) {
	t.Parallel()

	for h, want := range map[string]bool{
		"Amazon SNS now supports push notifications in four additional AWS Regions":  true,
		"Amazon EC2 G7 instances now available in additional regions":                true,
		"Agent Registry is now generally available in AWS GovCloud (US-West) Region": true,
		"AWS Quick apps are now available in Asia Pacific (Singapore)":               true,
		"AWS KMS now tracks last usage of all KMS keys":                              false,
		"Amazon EKS now supports Kubernetes version rollback":                        false,
	} {
		if got := RegionExpansion(h); got != want {
			t.Errorf("RegionExpansion(%q) = %v", h, got)
		}
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()

	rel := time.Date(2026, 4, 27, 19, 0, 0, 0, time.UTC)
	as := []Announcement{
		{ID: "kms", URL: "u-kms", Headline: "AWS KMS now tracks last usage of all KMS keys", PostedAt: rel.Add(time.Hour),
			Body: "View the last cryptographic operation performed with your KMS keys."},
		{ID: "kms-region", URL: "u-region", Headline: "AWS KMS key last usage now available in additional AWS Regions", PostedAt: rel},
		{ID: "late", URL: "u-late", Headline: "AWS KMS adds key last usage reports", PostedAt: rel.Add(10 * 24 * time.Hour)},
		{ID: "eks", URL: "u-eks", Headline: "Amazon EKS now supports Kubernetes version rollback", PostedAt: rel},
		{ID: "s3", URL: "u-s3", Headline: "Amazon S3 adds annotations for objects", PostedAt: rel},
	}
	m := NewMatcher(as)
	if m.Len() != 4 {
		t.Errorf("Len = %d, want 4 (the region expansion is left out)", m.Len())
	}
	got := m.Match("KMS GetKeyLastUsage API provides the last cryptographic operation performed on KMS keys.", "KMS", rel)
	if got == nil || got.Announcement.ID != "kms" || got.Score < DefaultMinScore || !strings.Contains(got.Basis, "text similarity") {
		t.Fatalf("Match = %+v", got)
	}
	if got := m.Match("Adds support for Iceberg tables.", "Glue", rel); got != nil {
		t.Errorf("unrelated item matched %s", got.Announcement.ID)
	}
	// Outside the window, the only similar post is too late.
	if got := m.Match("KMS key last usage reports", "KMS", rel.Add(-30*24*time.Hour)); got != nil {
		t.Errorf("matched %s outside the window", got.Announcement.ID)
	}

	// A corpus of one post still matches it.
	one := NewMatcher(as[:1])
	if got := one.Match(as[0].Headline+" "+as[0].Body, "KMS", rel); got == nil || got.Announcement.ID != "kms" {
		t.Errorf("single-post match = %+v", got)
	}

	// Two equally good posts: no clear winner.
	twins := NewMatcher([]Announcement{
		{ID: "a", Headline: "Amazon EKS now supports version rollback", PostedAt: rel},
		{ID: "b", Headline: "Amazon EKS now supports version rollback", PostedAt: rel},
		{ID: "c", Headline: "Amazon S3 adds annotations", PostedAt: rel},
	})
	if got := twins.Match("EKS version rollback", "EKS", rel); got != nil {
		t.Errorf("ambiguous match = %s (%.2f vs %.2f)", got.Announcement.ID, got.Score, got.RunnerUp)
	}
}

type memStore struct{ got []Announcement }

func (s *memStore) PutAnnouncements(_ context.Context, as []Announcement) error {
	s.got = append(s.got, as...)
	return nil
}

func TestPull(t *testing.T) {
	t.Parallel()

	pages := []string{
		`{"items":[
			{"item":{"name":"one","additionalFields":{"headline":" New thing ","headlineUrl":"/about-aws/whats-new/2026/11/one/","postDateTime":"2026-11-12T10:00:00Z","postBody":"<p>b</p>"}},
			 "tags":[{"id":"whats-new-v2#year#2026","description":"description placeholder"},{"id":"x","description":"general:products/amazon-s3"}]},
			{"item":{"name":"two","additionalFields":{"headline":"Older","headlineUrl":"https://example.com/two","postDateTime":"2026-11-10T10:00:00Z"}}}]}`,
		`{"items":[
			{"item":{"name":"three","additionalFields":{"headline":"Oldest","headlineUrl":"/x","postDateTime":"2026-11-01T10:00:00Z"}}}]}`,
	}
	var sizes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sizes = append(sizes, r.URL.Query().Get("size")+"/"+r.URL.Query().Get("page"))
		if r.Header.Get("User-Agent") != "radar/test" {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		switch p := r.URL.Query().Get("page"); p {
		case "0", "1":
			w.Write([]byte(pages[p[0]-'0']))
		default:
			w.Write([]byte(`{"items":[]}`))
		}
	}))
	defer srv.Close()

	st := &memStore{}
	c := &Client{URL: srv.URL + "/search?item.directoryId=whats-new-v2", HTTP: srv.Client(), UserAgent: "radar/test"}
	n, err := Pull(context.Background(), c, st, time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(st.got) != 2 || !slices.Equal(sizes, []string{"100/0", "100/1"}) {
		t.Fatalf("pulled %d (%+v), requests %v", n, st.got, sizes)
	}
	a := st.got[0]
	if a.ID != "one" || a.Headline != "New thing" || a.URL != Site+"/about-aws/whats-new/2026/11/one/" ||
		!slices.Equal(a.Tags, []string{"whats-new-v2#year#2026", "general:products/amazon-s3"}) {
		t.Errorf("announcement = %+v", a)
	}
	if st.got[1].URL != "https://example.com/two" {
		t.Errorf("absolute URL = %q", st.got[1].URL)
	}
}
