package items

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestPost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	at := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	ps := PostStore{Store: s, Source: "blogs"}
	if err := ps.PutPosts(ctx, []whatsnew.Post{{ID: "p", URL: "https://aws.amazon.com/blogs/x/post/", Title: "Post", PostedAt: at,
		Tags: []string{whatsnew.LaunchTag}}}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"https://aws.amazon.com/blogs/x/post/", "https://aws.amazon.com/blogs/x/post"} {
		p, err := s.Post(ctx, u)
		if err != nil || p == nil || p.ID != "p" || !p.PostedAt.Equal(at) || len(p.Tags) != 1 {
			t.Errorf("Post(%s) = %+v, %v", u, p, err)
		}
	}
	if p, err := s.Post(ctx, "https://aws.amazon.com/blogs/x/other/"); err != nil || p != nil {
		t.Errorf("Post(other) = %+v, %v; want nil", p, err)
	}
}
