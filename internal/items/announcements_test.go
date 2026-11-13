package items

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/YakDriver/radar/internal/whatsnew"
)

func TestAnnouncements(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	as := AnnouncementStore{Store: s, Source: "wn"}
	if err := as.PutAnnouncements(ctx, []whatsnew.Announcement{
		{ID: "a", URL: "u-a", Headline: "A", PostedAt: at, Body: "<p>a</p>", Tags: []string{"t1"}},
		{ID: "b", URL: "u-b", Headline: "B", PostedAt: at.Add(time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}
	// A later pull replaces an edited post.
	if err := as.PutAnnouncements(ctx, []whatsnew.Announcement{{ID: "a", URL: "u-a", Headline: "A, edited", PostedAt: at, Tags: []string{"t1", "t2"}}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Announcements(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "b" || got[1].Headline != "A, edited" || !got[1].PostedAt.Equal(at) || !slices.Equal(got[1].Tags, []string{"t1", "t2"}) {
		t.Errorf("announcements = %+v", got)
	}
}
