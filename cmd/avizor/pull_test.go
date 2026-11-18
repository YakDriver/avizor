package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/config"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestPullWhatsNewCheckpoint(t *testing.T) {
	t.Parallel()

	posted := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "0" {
			w.Write([]byte(`{"items":[]}`))
			return
		}
		w.Write([]byte(`{"items":[{"item":{"name":"a","additionalFields":{"headline":"A","headlineUrl":"/a","postDateTime":"` + posted + `"}}}]}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	st, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	src := &config.Source{Name: "wn", Type: config.SourceAWSWhatsNew, URL: srv.URL + "/s", Start: "30d"}

	res, err := pullDirectory(ctx, src, st, time.Time{}, nil)
	if err != nil || res.Announcements == nil || *res.Announcements != 1 {
		t.Fatalf("first pull = %+v, %v", res, err)
	}
	cp, _ := st.Meta(ctx, "checkpoint:wn")
	if cp == "" {
		t.Fatal("no checkpoint")
	}

	// The next pull starts a week before the checkpoint, so it sees the two-day-old post again.
	res, err = pullDirectory(ctx, src, st, time.Time{}, nil)
	if err != nil || res.Announcements == nil || *res.Announcements != 1 {
		t.Fatalf("second pull = %+v, %v", res, err)
	}
	cpTime, _ := time.Parse(time.RFC3339, cp)
	if since, _ := time.Parse(time.RFC3339, res.Since); !since.Equal(cpTime.Add(-whatsNewOverlap)) {
		t.Errorf("second pull since %s, want a week before %s", res.Since, cp)
	}

	// An explicit --since doesn't move the checkpoint.
	if _, err := pullDirectory(ctx, src, st, time.Now().Add(-time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Meta(ctx, "checkpoint:wn"); got == "" {
		t.Error("checkpoint cleared")
	}
}

func TestPullBlogs(t *testing.T) {
	t.Parallel()

	created := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	old := time.Now().UTC().Add(-90 * 24 * time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "0" {
			w.Write([]byte(`{"items":[]}`))
			return
		}
		w.Write([]byte(`{"items":[
{"item":{"name":"p1","additionalFields":{"title":" Use query statistics in Aurora DSQL ","link":"https://aws.amazon.com/blogs/database/q/","createdDate":"` + created + `","postExcerpt":"Find slow queries."}},
 "tags":[{"id":"blog-posts#category-post-types#announcements"},{"id":"blog-posts#category-database-amazon-aurora#dsql"}]},
{"item":{"name":"p0","additionalFields":{"title":"Old","link":"https://aws.amazon.com/blogs/x/old/","createdDate":"` + old + `"}}}]}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	st, err := items.Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	src := &config.Source{Name: "blogs", Type: config.SourceAWSBlogs, URL: srv.URL + "/s", Start: "30d"}
	res, err := pullDirectory(ctx, src, st, time.Time{}, nil)
	if err != nil || res.Posts == nil || *res.Posts != 1 || res.Announcements != nil {
		t.Fatalf("pull = %+v, %v", res, err)
	}
	ps, err := st.Posts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Title != "Use query statistics in Aurora DSQL" || ps[0].Excerpt != "Find slow queries." ||
		!slices.Equal(ps[0].Tags, []string{"blog-posts#category-post-types#announcements", "blog-posts#category-database-amazon-aurora#dsql"}) {
		t.Errorf("posts = %+v", ps)
	}
	// Blog posts aren't announcements.
	if as, err := st.Announcements(ctx); err != nil || len(as) != 0 {
		t.Errorf("announcements = %v, %v", as, err)
	}
}

func TestBlogPostLookup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	st, err := items.Open(config.ItemsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := (items.PostStore{Store: st, Source: "b"}).PutPosts(ctx, []whatsnew.Post{{ID: "p", URL: "https://aws.amazon.com/blogs/x/p/", Title: "P", PostedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	st.Close()
	// Stored posts are found with or without the trailing slash; others are linked by URL.
	if p, err := blogPost(ctx, dir, "https://aws.amazon.com/blogs/x/p"); err != nil || p.Title != "P" {
		t.Errorf("stored post = %+v, %v", p, err)
	}
	if p, err := blogPost(ctx, dir, "https://example.com/q"); err != nil || p.Title != "https://example.com/q" {
		t.Errorf("unknown post = %+v, %v", p, err)
	}
}
