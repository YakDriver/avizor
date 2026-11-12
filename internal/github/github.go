// Package github is a minimal GitHub REST client for listing issues, pull requests, and comments.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.github.com"
	perPage        = 100
	maxRetries     = 4
	// maxRateLimitWait bounds how long the client sleeps for a primary rate-limit reset before giving up.
	maxRateLimitWait = 15 * time.Minute
)

// Client calls the GitHub REST API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	token   string
	// Log, if set, receives progress messages such as rate-limit waits.
	Log func(format string, args ...any)
}

// NewClient returns a client authenticated with GITHUB_TOKEN, GH_TOKEN, or `gh auth token`.
func NewClient(ctx context.Context) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
		if err != nil {
			return nil, errors.New("no GitHub token: set GITHUB_TOKEN or run `gh auth login`")
		}
		token = strings.TrimSpace(string(out))
	}
	return &Client{
		BaseURL: defaultBaseURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		token:   token,
	}, nil
}

// Issue is the subset of the REST issue object Radar uses. The issues endpoints also return pull requests; those have
// PullRequest set.
type Issue struct {
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	State       string     `json:"state"`
	StateReason string     `json:"state_reason"`
	HTMLURL     string     `json:"html_url"`
	User        User       `json:"user"`
	Labels      []Label    `json:"labels"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	PullRequest *struct {
		MergedAt *time.Time `json:"merged_at"`
	} `json:"pull_request"`
}

// User is a GitHub account.
type User struct {
	Login string `json:"login"`
}

// Label is an issue label.
type Label struct {
	Name string `json:"name"`
}

// Comment is an issue or PR conversation comment.
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	IssueURL  string    `json:"issue_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var issueNumberRE = regexp.MustCompile(`/issues/(\d+)$`)

// IssueNumber returns the issue or PR number a comment belongs to.
func (c *Comment) IssueNumber() (int, error) {
	m := issueNumberRE.FindStringSubmatch(c.IssueURL)
	if m == nil {
		return 0, fmt.Errorf("comment %d: unexpected issue_url %q", c.ID, c.IssueURL)
	}
	return strconv.Atoi(m[1])
}

// GetIssue fetches one issue or PR.
func (c *Client) GetIssue(ctx context.Context, repo string, number int) (*Issue, error) {
	var is Issue
	if _, err := c.get(ctx, fmt.Sprintf("%s/repos/%s/issues/%d", c.BaseURL, repo, number), &is); err != nil {
		return nil, err
	}
	return &is, nil
}

// ListIssues pages through issues and PRs updated at or after since (all if zero), oldest update first, calling fn for
// each page. Returning an error from fn stops paging.
func (c *Client) ListIssues(ctx context.Context, repo string, since time.Time, fn func([]Issue) error) error {
	q := url.Values{
		"state":     {"all"},
		"sort":      {"updated"},
		"direction": {"asc"},
		"per_page":  {strconv.Itoa(perPage)},
	}
	return listWindowed(ctx, c, fmt.Sprintf("%s/repos/%s/issues", c.BaseURL, repo), q, since, func(is Issue) time.Time { return is.UpdatedAt }, fn)
}

// ListComments pages through all issue and PR comments in a repo updated at or after since, oldest update first.
func (c *Client) ListComments(ctx context.Context, repo string, since time.Time, fn func([]Comment) error) error {
	q := url.Values{
		"sort":      {"updated"},
		"direction": {"asc"},
		"per_page":  {strconv.Itoa(perPage)},
	}
	return listWindowed(ctx, c, fmt.Sprintf("%s/repos/%s/issues/comments", c.BaseURL, repo), q, since, func(cm Comment) time.Time { return cm.UpdatedAt }, fn)
}

// listWindowed pages through a list sorted by ascending update time. GitHub stops returning a next link after a fixed
// number of pages on some list endpoints, so a pass that ends on a full page may be truncated. listWindowed then starts
// a new pass from the latest update time seen. Items at that boundary are delivered twice; callers must be idempotent.
func listWindowed[T any](ctx context.Context, c *Client, base string, q url.Values, since time.Time, updated func(T) time.Time, fn func([]T) error) error {
	for {
		if !since.IsZero() {
			q.Set("since", since.UTC().Format(time.RFC3339))
		}
		var (
			latest   time.Time
			lastFull bool
			pages    int
		)
		err := paginate(ctx, c, base+"?"+q.Encode(), func(page []T) error {
			pages++
			lastFull = len(page) == perPage
			for _, it := range page {
				if u := updated(it); u.After(latest) {
					latest = u
				}
			}
			return fn(page)
		})
		if err != nil {
			return err
		}
		if !lastFull {
			return nil
		}
		if !latest.After(since) {
			// A full pass that didn't advance means one timestamp spans more items than a pass returns. Return rather
			// than loop; the next sync resumes from the checkpoint.
			if pages > 1 {
				return fmt.Errorf("listing %s: more than %d pages of items share update time %s", redact(base), pages, latest.Format(time.RFC3339))
			}
			return nil
		}
		if c.Log != nil && pages > 1 {
			c.Log("pagination ended after %d pages; continuing from %s", pages, latest.Format(time.RFC3339))
		}
		since = latest
	}
}

func paginate[T any](ctx context.Context, c *Client, next string, fn func([]T) error) error {
	for next != "" {
		var page []T
		h, err := c.get(ctx, next, &page)
		if err != nil {
			return err
		}
		if err := fn(page); err != nil {
			return err
		}
		next = nextLink(h.Get("Link"))
	}
	return nil
}

var linkNextRE = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func nextLink(h string) string {
	if m := linkNextRE.FindStringSubmatch(h); m != nil {
		return m[1]
	}
	return ""
}

// get performs a GET with retries for server errors and rate limits, and decodes JSON into v.
func (c *Client) get(ctx context.Context, u string, v any) (http.Header, error) {
	if !strings.HasPrefix(u, c.BaseURL+"/") {
		return nil, fmt.Errorf("refusing to follow URL outside %s: %s", c.BaseURL, u)
	}
	var lastErr error
	for attempt := range maxRetries + 1 {
		if attempt > 0 {
			if err := sleep(ctx, time.Duration(1<<attempt)*time.Second); err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("User-Agent", "radar")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			if err := json.Unmarshal(body, v); err != nil {
				return nil, fmt.Errorf("decoding %s: %w", redact(u), err)
			}
			return resp.Header, nil
		case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
			wait, ok := rateLimitWait(resp.Header)
			if !ok {
				return nil, fmt.Errorf("GET %s: %s: %s", redact(u), resp.Status, snippet(body))
			}
			if wait > maxRateLimitWait {
				return nil, fmt.Errorf("GET %s: rate limited for %s; try again later", redact(u), wait.Round(time.Second))
			}
			if c.Log != nil {
				c.Log("rate limited; waiting %s", wait.Round(time.Second))
			}
			if err := sleep(ctx, wait); err != nil {
				return nil, err
			}
			lastErr = fmt.Errorf("rate limited")
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("GET %s: %s", redact(u), resp.Status)
		default:
			return nil, fmt.Errorf("GET %s: %s: %s", redact(u), resp.Status, snippet(body))
		}
	}
	return nil, fmt.Errorf("GET %s: giving up after %d attempts: %w", redact(u), maxRetries+1, lastErr)
}

// rateLimitWait reports how long to wait for a primary or secondary rate limit, if the response is one.
func rateLimitWait(h http.Header) (time.Duration, bool) {
	if s := h.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second, true
		}
	}
	if h.Get("X-RateLimit-Remaining") == "0" {
		if n, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			return max(time.Until(time.Unix(n, 0))+time.Second, time.Second), true
		}
	}
	return 0, false
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func redact(u string) string {
	if before, _, ok := strings.Cut(u, "?"); ok {
		return before
	}
	return u
}

func snippet(b []byte) string {
	b = bytes.TrimSpace(b)
	if len(b) > 200 {
		b = b[:200]
	}
	return string(b)
}
