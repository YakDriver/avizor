package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// TestListCommentsPageCap checks that listing continues past a server that stops returning next links after a fixed
// number of pages, as GitHub does on some list endpoints.
func TestListCommentsPageCap(t *testing.T) {
	t.Parallel()

	const total, maxPages = 350, 2
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	all := make([]Comment, total)
	for i := range all {
		all[i] = Comment{ID: int64(i + 1), UpdatedAt: base.Add(time.Duration(i) * time.Minute)}
	}

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, _ := time.Parse(time.RFC3339, r.URL.Query().Get("since"))
		var items []Comment
		for _, c := range all {
			if since.IsZero() || !c.UpdatedAt.Before(since) {
				items = append(items, c)
			}
		}
		page := max(1, atoi(r.URL.Query().Get("page")))
		lo, hi := min((page-1)*perPage, len(items)), min(page*perPage, len(items))
		if hi < len(items) && page < maxPages {
			q := r.URL.Query()
			q.Set("page", strconv.Itoa(page+1))
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?%s>; rel="next"`, srvURL, r.URL.Path, q.Encode()))
		}
		_ = json.NewEncoder(w).Encode(items[lo:hi])
	}))
	defer srv.Close()
	srvURL = srv.URL

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	seen := map[int64]bool{}
	err := c.ListComments(context.Background(), "o/r", time.Time{}, func(page []Comment) error {
		for _, cm := range page {
			seen[cm.ID] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != total {
		t.Errorf("saw %d distinct comments, want %d", len(seen), total)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func TestNextLink(t *testing.T) {
	t.Parallel()

	h := `<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=9>; rel="last"`
	if got := nextLink(h); got != "https://api.github.com/x?page=2" {
		t.Errorf("nextLink = %q", got)
	}
	if got := nextLink(""); got != "" {
		t.Errorf("nextLink(empty) = %q", got)
	}
}

func TestRateLimitWait(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Set("Retry-After", "7")
	if d, ok := rateLimitWait(h); !ok || d != 7*time.Second {
		t.Errorf("Retry-After: %s, %v", d, ok)
	}
	if _, ok := rateLimitWait(http.Header{}); ok {
		t.Error("plain 403 treated as a rate limit")
	}
}
