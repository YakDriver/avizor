package enrich

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

// BlogPost is a blog post near an item's release, scored against the item and its matched announcement's headline.
// Launch is set for a post tagged as a launch announcement, and TaggedService for one whose product tags name the
// service.
type BlogPost struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Posted        string  `json:"posted"`
	Days          float64 `json:"days"`
	Score         float64 `json:"score"`
	Launch        bool    `json:"launch,omitempty"`
	TaggedService bool    `json:"tagged_service,omitempty"`
}

// Blog posts often come days after the release, so the window reaches further after it than for announcements.
const (
	BlogBefore = 3 * 24 * time.Hour
	BlogAfter  = 45 * 24 * time.Hour
)

// Ranking boosts: a launch announcement's score counts 1.5 times, and a post tagged with the service 1.25 times
// (1.75 for both). On the blog eval cases (51 items), they raise hit@1 from 78% to 90%; hit@5 stays 98%.
const (
	launchBoost = 0.5
	tagBoost    = 0.25
)

// BlogPosts ranks the posts created from BlogBefore the release to BlogAfter it, best first, n at most (0 for all).
// The query is the item's text and the matched announcement's headline, if any. Posts are ordered by their text
// score, boosted for a launch announcement and for a tag naming the service; Score is the text score.
func BlogPosts(posts *whatsnew.Matcher, it *awssdk.Item, ann *whatsnew.Match, n int) []BlogPost {
	out := []BlogPost{}
	if posts == nil {
		return out
	}
	text, service := AnnouncementQuery(it)
	if ann != nil {
		text += "\n" + ann.Announcement.Headline
	}
	key := strings.Join(ServiceWords(service), "")
	for _, r := range posts.Rank(text, service, it.Release.PublishedAt, BlogBefore, BlogAfter, 0) {
		out = append(out, BlogPost{Title: r.Announcement.Headline, URL: r.Announcement.URL, Posted: r.Announcement.PostedAt.Format(time.DateOnly),
			Days: r.Days, Score: r.Score, Launch: slices.Contains(r.Announcement.Tags, whatsnew.LaunchTag),
			TaggedService: taggedService(r.Announcement.Tags, key)})
	}
	// Stable, so equal boosted scores keep Rank's order: score, then post time, then URL.
	slices.SortStableFunc(out, func(a, b BlogPost) int { return cmp.Compare(boosted(b), boosted(a)) })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func boosted(p BlogPost) float64 {
	f := 1.0
	if p.Launch {
		f += launchBoost
	}
	if p.TaggedService {
		f += tagBoost
	}
	return p.Score * f
}

// genericWords are words of service names that don't identify the service.
var genericWords = []string{"amazon", "aws", "service", "services"}

// ServiceWords returns the words of a service name that identify it: "SageMaker" → sage, maker.
func ServiceWords(service string) []string {
	return slices.DeleteFunc(whatsnew.Tokens(service), func(w string) bool { return slices.Contains(genericWords, w) })
}

// taggedService reports whether a blog post's product category tag names a service: whole words of the tag's last
// part, run together, equal the service's identifying words run together. "dsql" in
// blog-posts#category-database-amazon-aurora#dsql, "sagemaker" in ...#amazon-sagemaker-ai.
func taggedService(tags []string, key string) bool {
	if key == "" {
		return false
	}
	return slices.ContainsFunc(tags, func(t string) bool {
		if !strings.HasPrefix(t, "blog-posts#category") {
			return false
		}
		// A run of whole words: "secrets-manager" for Secrets Manager, but not "ecr" inside "secrets".
		words := strings.Split(t[strings.LastIndex(t, "#")+1:], "-")
		for i := range words {
			for j := i + 1; j <= len(words); j++ {
				if strings.Join(words[i:j], "") == key {
					return true
				}
			}
		}
		return false
	})
}
