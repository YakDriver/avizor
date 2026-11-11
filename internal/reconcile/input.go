package reconcile

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/YakDriver/radar/internal/index"
	"github.com/YakDriver/radar/internal/signal"
)

// Input kinds.
const (
	InputIssue = "issue"
	InputURL   = "url"
	InputText  = "text"
)

const maxPageBytes = 2 << 20

// input is a resolved input with the text used for each purpose.
type input struct {
	info InputInfo
	// signalText is searched for exact-match signals.
	signalText string
	// focusText is the short description (title, or the whole text for --text) used for keywords, service detection,
	// and Terraform names that drive code search.
	focusText string
	// indexed holds signals already recorded in the index for an issue input (including comment signals).
	indexed []index.SignalHit
	// pageTitle is set when a URL's page was fetched.
	pageTitle string
}

var (
	issueArgRE  = regexp.MustCompile(`^#?(\d+)$`)
	ghIssueRE   = regexp.MustCompile(`^https?://github\.com/([^/]+/[^/]+)/(?:issues|pull)/(\d+)`)
	titleRE     = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	metaRE      = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	metaAttrRE  = regexp.MustCompile(`(?is)(property|name|content)\s*=\s*"([^"]*)"`)
	titleTailRE = regexp.MustCompile(`\s+[|-]\s+(?:AWS|Amazon Web Services)(?:\s.*)?$`)
)

func (r *Reconciler) resolve(ctx context.Context, arg, text string) (*input, error) {
	if text != "" {
		if arg != "" {
			return nil, errors.New("give either an argument or --text, not both")
		}
		return &input{
			info:       InputInfo{Kind: InputText, Ref: text},
			signalText: text,
			focusText:  text,
		}, nil
	}

	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, errors.New("nothing to reconcile: give an issue number, URL, or --text")
	}

	if m := issueArgRE.FindStringSubmatch(arg); m != nil {
		n, _ := strconv.Atoi(m[1])
		return r.resolveIssue(ctx, arg, n)
	}
	if m := ghIssueRE.FindStringSubmatch(arg); m != nil && strings.EqualFold(m[1], r.Target.Repo) {
		n, _ := strconv.Atoi(m[2])
		return r.resolveIssue(ctx, arg, n)
	}
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		return r.resolveURL(ctx, arg)
	}
	return nil, fmt.Errorf("can't interpret %q: give an issue number, URL, or --text", arg)
}

func (r *Reconciler) resolveIssue(ctx context.Context, ref string, n int) (*input, error) {
	is, err := r.Store.Issue(ctx, n)
	if err != nil {
		return nil, err
	}
	if is == nil {
		if r.Issues == nil {
			return nil, fmt.Errorf("#%d is not in the index; run `radar sync`", n)
		}
		if is, err = r.Issues.FetchIssue(ctx, r.Target.Repo, n); err != nil {
			return nil, fmt.Errorf("#%d is not in the index and fetching it failed: %w", n, err)
		}
		r.warnf("#%d is not in the index; fetched it from GitHub (comment signals unavailable)", n)
	}

	in := &input{
		info: InputInfo{
			Kind:   InputIssue,
			Ref:    ref,
			Number: is.Number,
			Title:  is.Title,
			URL:    is.URL,
			Labels: is.Labels,
		},
		signalText: is.Title + "\n" + is.Body,
		focusText:  is.Title,
	}
	if in.indexed, err = r.Store.Signals(ctx, n); err != nil {
		return nil, err
	}
	return in, nil
}

func (r *Reconciler) resolveURL(ctx context.Context, raw string) (*input, error) {
	u := signal.NormalizeURL(raw)
	if u == "" {
		return nil, fmt.Errorf("invalid URL %q", raw)
	}
	in := &input{
		info:       InputInfo{Kind: InputURL, Ref: raw, URL: u},
		signalText: u,
	}
	if r.Options.NoFetch || r.HTTP == nil {
		return in, nil
	}

	title, desc, err := fetchPage(ctx, r.HTTP, raw)
	if err != nil {
		r.warnf("fetching %s failed, matching on the URL only: %v", raw, err)
		return in, nil
	}
	in.info.Title = title
	in.pageTitle = title
	in.focusText = title
	in.signalText = u + "\n" + title + "\n" + desc
	return in, nil
}

// fetchPage returns a page's title and description from <title> and Open Graph or description meta tags.
func fetchPage(ctx context.Context, c *http.Client, u string) (title, desc string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "radar")
	resp, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	if err != nil {
		return "", "", err
	}
	return parsePage(string(b))
}

func parsePage(page string) (title, desc string, err error) {
	meta := map[string]string{}
	for _, tag := range metaRE.FindAllString(page, -1) {
		var key, content string
		for _, a := range metaAttrRE.FindAllStringSubmatch(tag, -1) {
			switch strings.ToLower(a[1]) {
			case "property", "name":
				key = strings.ToLower(a[2])
			case "content":
				content = a[2]
			}
		}
		if key != "" && content != "" {
			if _, ok := meta[key]; !ok {
				meta[key] = clean(content)
			}
		}
	}

	title = meta["og:title"]
	if title == "" {
		if m := titleRE.FindStringSubmatch(page); m != nil {
			title = clean(m[1])
		}
	}
	title = titleTailRE.ReplaceAllString(title, "")
	desc = meta["og:description"]
	if desc == "" {
		desc = meta["description"]
	}
	if title == "" {
		return "", "", errors.New("no title found")
	}
	return title, desc, nil
}

func clean(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}
