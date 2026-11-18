// Package whatsnew reads AWS What's New announcements and AWS blog posts, and matches announcements to SDK items.
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

// BlogsURL is the same directory API's blog posts, newest first.
const BlogsURL = "https://aws.amazon.com/api/dirs/items/search?item.directoryId=blog-posts&sort_by=item.additionalFields.createdDate&sort_order=desc&item.locale=en_US"

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

// Post is one AWS blog post.
type Post struct {
	// ID is the directory item's name.
	ID       string    `json:"id"`
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	PostedAt time.Time `json:"posted_at"`
	// Excerpt is the post's summary. The directory doesn't have the body.
	Excerpt string `json:"excerpt,omitempty"`
	// Tags are tag IDs: the post type (blog-posts#category-post-types#announcements), the content type
	// (GLOBAL#content-type#launch-announcement), and product categories (blog-posts#category-database-amazon-aurora#dsql).
	Tags []string `json:"tags,omitempty"`
}

// Client pages through a directory, newest first.
type Client struct {
	// URL is the directory search URL without size and page. Empty uses DefaultURL for announcements and BlogsURL
	// for posts.
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
				// Blog posts.
				Title       string `json:"title"`
				Link        string `json:"link"`
				CreatedDate string `json:"createdDate"`
				PostExcerpt string `json:"postExcerpt"`
			} `json:"additionalFields"`
		} `json:"item"`
		Tags []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
		} `json:"tags"`
	} `json:"items"`
}

// fetch decodes page n of a directory.
func (c *Client) fetch(ctx context.Context, base string, n, size int) (*page, error) {
	if c.URL != "" {
		base = c.URL
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("directory URL: %w", err)
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
		return nil, fmt.Errorf("fetching page %d: %w", n, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("fetching page %d: %s: %s", n, resp.Status, strings.TrimSpace(string(b)))
	}
	var p page
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decoding page %d: %w", n, err)
	}
	return &p, nil
}

// Page returns one page of announcements, newest first. An empty page means there are no more.
func (c *Client) Page(ctx context.Context, n, size int) ([]Announcement, error) {
	p, err := c.fetch(ctx, DefaultURL, n, size)
	if err != nil {
		return nil, fmt.Errorf("what's new: %w", err)
	}
	out := make([]Announcement, 0, len(p.Items))
	for _, it := range p.Items {
		f := it.Item.AdditionalFields
		posted, err := time.Parse(time.RFC3339, f.PostDateTime)
		if err != nil {
			return nil, fmt.Errorf("what's new item %s: posted at %q: %w", it.Item.Name, f.PostDateTime, err)
		}
		a := Announcement{ID: it.Item.Name, URL: Absolute(f.HeadlineURL), Headline: strings.TrimSpace(f.Headline), PostedAt: posted.UTC(), Body: f.PostBody}
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

// Posts returns one page of blog posts, newest first. An empty page means there are no more.
func (c *Client) Posts(ctx context.Context, n, size int) ([]Post, error) {
	p, err := c.fetch(ctx, BlogsURL, n, size)
	if err != nil {
		return nil, fmt.Errorf("blogs: %w", err)
	}
	out := make([]Post, 0, len(p.Items))
	for _, it := range p.Items {
		f := it.Item.AdditionalFields
		created, err := time.Parse(time.RFC3339, f.CreatedDate)
		if err != nil {
			return nil, fmt.Errorf("blog post %s: created %q: %w", it.Item.Name, f.CreatedDate, err)
		}
		bp := Post{ID: it.Item.Name, URL: Absolute(f.Link), Title: strings.TrimSpace(f.Title), PostedAt: created.UTC(),
			Excerpt: strings.TrimSpace(f.PostExcerpt)}
		for _, t := range it.Tags {
			bp.Tags = append(bp.Tags, t.ID)
		}
		out = append(out, bp)
	}
	return out, nil
}

// Absolute resolves a relative URL from the directory, with or without a leading slash, against Site.
func Absolute(u string) string {
	switch {
	case u == "", strings.HasPrefix(u, "https://"), strings.HasPrefix(u, "http://"):
		return u
	case strings.HasPrefix(u, "/"):
		return Site + u
	}
	return Site + "/" + u
}

// Store keeps announcements.
type Store interface {
	PutAnnouncements(ctx context.Context, as []Announcement) error
}

// PostStore keeps blog posts.
type PostStore interface {
	PutPosts(ctx context.Context, ps []Post) error
}

// Pull fetches announcements posted at or after since, newest first, and stores them. It stops at the first page that
// reaches past since. It returns the number stored.
func Pull(ctx context.Context, c *Client, st Store, since time.Time, logf func(string, ...any)) (int, error) {
	return pull(ctx, c.Page, func(a Announcement) time.Time { return a.PostedAt }, st.PutAnnouncements, since, logf, "what's new", "announcement(s)")
}

// PullPosts fetches blog posts created at or after since, newest first, and stores them, as Pull does.
func PullPosts(ctx context.Context, c *Client, st PostStore, since time.Time, logf func(string, ...any)) (int, error) {
	return pull(ctx, c.Posts, func(p Post) time.Time { return p.PostedAt }, st.PutPosts, since, logf, "blogs", "post(s)")
}

func pull[T any](ctx context.Context, page func(context.Context, int, int) ([]T, error), at func(T) time.Time,
	put func(context.Context, []T) error, since time.Time, logf func(string, ...any), label, noun string) (int, error) {
	const size = 100
	total := 0
	for n := 0; ; n++ {
		xs, err := page(ctx, n, size)
		if err != nil {
			return total, err
		}
		keep := xs[:0]
		done := len(xs) == 0
		for _, x := range xs {
			if at(x).Before(since) {
				done = true
				continue
			}
			keep = append(keep, x)
		}
		if err := put(ctx, keep); err != nil {
			return total, err
		}
		total += len(keep)
		if logf != nil && len(keep) > 0 {
			logf("%s: %d %s back to %s", label, total, noun, at(keep[len(keep)-1]).Format(time.DateOnly))
		}
		if done {
			return total, nil
		}
	}
}
