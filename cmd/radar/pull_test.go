package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/items"
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

	res, err := pullWhatsNew(ctx, src, st, time.Time{}, nil)
	if err != nil || res.Announcements != 1 {
		t.Fatalf("first pull = %+v, %v", res, err)
	}
	cp, _ := st.Meta(ctx, "checkpoint:wn")
	if cp == "" {
		t.Fatal("no checkpoint")
	}

	// The next pull starts a week before the checkpoint, so it sees the two-day-old post again.
	res, err = pullWhatsNew(ctx, src, st, time.Time{}, nil)
	if err != nil || res.Announcements != 1 {
		t.Fatalf("second pull = %+v, %v", res, err)
	}
	cpTime, _ := time.Parse(time.RFC3339, cp)
	if since, _ := time.Parse(time.RFC3339, res.Since); !since.Equal(cpTime.Add(-whatsNewOverlap)) {
		t.Errorf("second pull since %s, want a week before %s", res.Since, cp)
	}

	// An explicit --since doesn't move the checkpoint.
	if _, err := pullWhatsNew(ctx, src, st, time.Now().Add(-time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Meta(ctx, "checkpoint:wn"); got == "" {
		t.Error("checkpoint cleared")
	}
}
