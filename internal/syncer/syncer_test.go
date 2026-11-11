package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/github"
	"github.com/YakDriver/radar/internal/index"
)

const repo = "o/r"

// fakeGitHub serves /repos/o/r/issues and /repos/o/r/issues/comments with one item per page.
type fakeGitHub struct {
	mu       sync.Mutex
	issues   []map[string]any
	comments []map[string]any
	sinces   []string
}

func (f *fakeGitHub) handler(base *string) http.Handler {
	mux := http.NewServeMux()
	serve := func(w http.ResponseWriter, r *http.Request, items []map[string]any) {
		f.mu.Lock()
		f.sinces = append(f.sinces, r.URL.Path+"?since="+r.URL.Query().Get("since"))
		f.mu.Unlock()

		since, _ := time.Parse(time.RFC3339, r.URL.Query().Get("since"))
		var filtered []map[string]any
		for _, it := range items {
			u, _ := time.Parse(time.RFC3339, it["updated_at"].(string))
			if since.IsZero() || !u.Before(since) {
				filtered = append(filtered, it)
			}
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		page = max(page, 1)
		var out []map[string]any
		if page <= len(filtered) {
			out = filtered[page-1 : page]
		}
		if page < len(filtered) {
			q := r.URL.Query()
			q.Set("page", strconv.Itoa(page+1))
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?%s>; rel="next"`, *base, r.URL.Path, q.Encode()))
		}
		_ = json.NewEncoder(w).Encode(out)
	}
	mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, r *http.Request) { serve(w, r, f.issues) })
	mux.HandleFunc("/repos/o/r/issues/comments", func(w http.ResponseWriter, r *http.Request) { serve(w, r, f.comments) })
	return mux
}

func issue(n int, updated string, pr bool, body string) map[string]any {
	m := map[string]any{
		"number": n, "title": fmt.Sprintf("issue %d", n), "body": body, "state": "open",
		"html_url": fmt.Sprintf("https://github.com/o/r/issues/%d", n), "user": map[string]any{"login": "u"},
		"labels": []map[string]any{{"name": "service/fsx"}}, "created_at": updated, "updated_at": updated,
	}
	if pr {
		m["pull_request"] = map[string]any{"merged_at": updated}
	}
	return m
}

func TestRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	f := &fakeGitHub{
		issues: []map[string]any{
			issue(1, "2026-01-01T00:00:00Z", false, "uses CreateThing"),
			issue(2, "2026-01-02T00:00:00Z", true, "fixes #1"),
			issue(3, "2026-01-03T00:00:00Z", false, ""),
		},
		comments: []map[string]any{
			{"id": 100, "body": "AWS announced https://aws.amazon.com/x/", "issue_url": "https://api.github.com/repos/o/r/issues/3", "updated_at": "2026-01-03T00:00:00Z"},
		},
	}
	var base string
	srv := httptest.NewServer(f.handler(&base))
	defer srv.Close()
	base = srv.URL

	gh := &github.Client{BaseURL: srv.URL, HTTP: srv.Client()}
	st, err := index.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tg := &config.Target{Name: "t", Repo: repo, Index: &config.Index{Comments: config.CommentsURLsAndIdentifiers}}

	res, err := Run(ctx, gh, st, tg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Issues != 3 || res.Comments != 1 {
		t.Errorf("synced %d issues, %d comments; want 3, 1", res.Issues, res.Comments)
	}
	stats, _ := st.Stats(ctx)
	if stats.Issues != 2 || stats.PRs != 1 {
		t.Errorf("stats = %+v", stats)
	}
	if is, _ := st.Issue(ctx, 2); is == nil || !is.Merged || is.Kind != index.KindPR {
		t.Errorf("#2 = %+v, want merged PR", is)
	}
	if hits, _ := st.SignalHits(ctx, "url", "https://aws.amazon.com/x"); len(hits) != 1 || hits[0].Number != 3 || hits[0].Where != index.WhereComment {
		t.Errorf("comment URL hits = %+v", hits)
	}
	if hits, _ := st.SignalHits(ctx, "op", "CreateThing"); len(hits) != 1 || hits[0].Number != 1 {
		t.Errorf("op hits = %+v", hits)
	}
	if v, _ := st.Meta(ctx, MetaIssuesSince); v != "2026-01-03T00:00:00Z" {
		t.Errorf("issues checkpoint = %q", v)
	}

	// Incremental: the next run starts at the checkpoint.
	f.sinces = nil
	if _, err := Run(ctx, gh, st, tg, Options{}); err != nil {
		t.Fatal(err)
	}
	if f.sinces[0] != "/repos/o/r/issues?since=2026-01-03T00:00:00Z" {
		t.Errorf("incremental request = %q", f.sinces[0])
	}

	// Full: #1 disappeared upstream and is removed with its signals.
	f.issues = f.issues[1:]
	res, err = Run(ctx, gh, st, tg, Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Removed != 1 {
		t.Errorf("removed = %d, want 1", res.Removed)
	}
	if is, _ := st.Issue(ctx, 1); is != nil {
		t.Error("#1 still indexed after full sync")
	}
	if hits, _ := st.SignalHits(ctx, "op", "CreateThing"); len(hits) != 0 {
		t.Errorf("signals for removed issue remain: %+v", hits)
	}

	// A different repo is refused.
	if _, err := Run(ctx, gh, st, &config.Target{Name: "t", Repo: "x/y", Index: tg.Index}, Options{}); err == nil {
		t.Error("expected error syncing a different repo into the same index")
	}
}

func TestRunSinceKeepsCheckpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	f := &fakeGitHub{issues: []map[string]any{issue(5, "2026-02-01T00:00:00Z", false, "")}}
	var base string
	srv := httptest.NewServer(f.handler(&base))
	defer srv.Close()
	base = srv.URL

	st, err := index.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tg := &config.Target{Name: "t", Repo: repo, Index: &config.Index{Comments: config.CommentsNone}}

	since := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if _, err := Run(ctx, &github.Client{BaseURL: srv.URL, HTTP: srv.Client()}, st, tg, Options{Since: since}); err != nil {
		t.Fatal(err)
	}
	if v, _ := st.Meta(ctx, MetaIssuesSince); v != "" {
		t.Errorf("--since moved the checkpoint to %q", v)
	}
	if len(f.sinces) != 1 {
		t.Errorf("comments were fetched with comments = none: %v", f.sinces)
	}
}
