// Package whatsnew reads AWS What's New announcements and matches them to SDK items.
package whatsnew

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultURL is the directory API behind https://aws.amazon.com/about-aws/whats-new/. It isn't a documented API, but
// it's what the What's New pages use, and unlike the RSS feed (the latest 100) it pages back through history.
const DefaultURL = "https://aws.amazon.com/api/dirs/items/search?item.directoryId=whats-new-v2&sort_by=item.additionalFields.postDateTime&sort_order=desc&item.locale=en_US"

// Site is prefixed to relative announcement URLs.
const Site = "https://aws.amazon.com"

// Announcement is one What's New post.
type Announcement struct {
	// ID is the directory item's name, a UUID.
	ID       string    `json:"id"`
	URL      string    `json:"url"`
	Headline string    `json:"headline"`
	PostedAt time.Time `json:"posted_at"`
	// Body is the post's HTML.
	Body string   `json:"body,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

// Client pages through announcements, newest first.
type Client struct {
	// URL is the directory search URL without size and page. Empty uses DefaultURL.
	URL       string
	HTTP      *http.Client
	UserAgent string
}

type page struct {
	Items []struct {
		Item struct {
			Name             string `json:"name"`
			AdditionalFields struct {
				Headline     string `json:"headline"`
				HeadlineURL  string `json:"headlineUrl"`
				PostDateTime string `json:"postDateTime"`
				PostBody     string `json:"postBody"`
			} `json:"additionalFields"`
		} `json:"item"`
		Tags []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
		} `json:"tags"`
	} `json:"items"`
}

// Page returns one page of announcements, newest first. An empty page means there are no more.
func (c *Client) Page(ctx context.Context, n, size int) ([]Announcement, error) {
	base := c.URL
	if base == "" {
		base = DefaultURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("what's new URL: %w", err)
	}
	q := u.Query()
	q.Set("size", strconv.Itoa(size))
	q.Set("page", strconv.Itoa(n))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: time.Minute}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching what's new page %d: %w", n, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("fetching what's new page %d: %s: %s", n, resp.Status, strings.TrimSpace(string(b)))
	}
	var p page
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decoding what's new page %d: %w", n, err)
	}
	out := make([]Announcement, 0, len(p.Items))
	for _, it := range p.Items {
		f := it.Item.AdditionalFields
		posted, err := time.Parse(time.RFC3339, f.PostDateTime)
		if err != nil {
			return nil, fmt.Errorf("what's new item %s: posted at %q: %w", it.Item.Name, f.PostDateTime, err)
		}
		a := Announcement{ID: it.Item.Name, URL: absolute(f.HeadlineURL), Headline: strings.TrimSpace(f.Headline), PostedAt: posted.UTC(), Body: f.PostBody}
		for _, t := range it.Tags {
			tag := t.Description
			if tag == "" || tag == "description placeholder" {
				tag = t.ID
			}
			a.Tags = append(a.Tags, tag)
		}
		out = append(out, a)
	}
	return out, nil
}

func absolute(u string) string {
	if strings.HasPrefix(u, "/") {
		return Site + u
	}
	return u
}

// Store keeps announcements.
type Store interface {
	PutAnnouncements(ctx context.Context, as []Announcement) error
}

// Pull fetches announcements posted at or after since, newest first, and stores them. It stops at the first page that
// reaches past since. It returns the number stored.
func Pull(ctx context.Context, c *Client, st Store, since time.Time, logf func(string, ...any)) (int, error) {
	const size = 100
	total := 0
	for n := 0; ; n++ {
		as, err := c.Page(ctx, n, size)
		if err != nil {
			return total, err
		}
		keep := as[:0]
		done := len(as) == 0
		for _, a := range as {
			if a.PostedAt.Before(since) {
				done = true
				continue
			}
			keep = append(keep, a)
		}
		if err := st.PutAnnouncements(ctx, keep); err != nil {
			return total, err
		}
		total += len(keep)
		if logf != nil && len(keep) > 0 {
			logf("what's new: %d announcement(s) back to %s", total, keep[len(keep)-1].PostedAt.Format(time.DateOnly))
		}
		if done {
			return total, nil
		}
	}
}
