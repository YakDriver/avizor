package main

import (
	"slices"
	"strings"

	"github.com/YakDriver/avizor/internal/whatsnew"
)

// Post significance signals. A significant post is one likely to have Terraform work: it announces a launch, is on the
// AWS News Blog, or matches a What's New announcement, and isn't one of the kinds that rarely has work.
const (
	SignalLaunch       = "launch"
	SignalNewsBlog     = "news_blog"
	SignalAnnouncement = "announcement"
)

// Kinds of post that are not significant, whatever their signals.
const (
	// KindRoundup: a weekly or monthly roundup. Its launches have their own posts and announcements.
	KindRoundup = "roundup"
	// KindEvent: an event guide, such as a re:Invent session list.
	KindEvent = "event"
	// KindDeveloperTools: SDKs, IDLs, and other developer tooling.
	KindDeveloperTools = "developer_tools"
	// KindCustomerStory: a customer or partner story.
	KindCustomerStory = "customer_story"
)

// Blog post tags the kinds come from.
const (
	tagEvents           = "blog-posts#category#events"
	tagDevTools         = "GLOBAL#tech-category#devtools"
	tagDeveloperTools   = "blog-posts#category#developer-tools"
	tagCustomerSolution = "blog-posts#category-post-types#customer-solutions"
	tagPartnerSolution  = "blog-posts#category-post-types#partner-solutions"
)

// Blog URL prefixes the kinds come from.
const (
	developerBlogPrefix = "https://aws.amazon.com/blogs/developer/"
	partnerBlogPrefix   = "https://aws.amazon.com/blogs/apn/"
)

// PostSignificance is whether a post is significant, and why.
type PostSignificance struct {
	Significant bool `json:"significant"`
	// Signals are the signals the post has, in a fixed order.
	Signals []string `json:"signals,omitempty"`
	// Kind is set for a kind of post that is not significant, whatever its signals.
	Kind string `json:"kind,omitempty"`
}

// postSignificance decides whether a post is significant from its tags, URL, title, and matched announcements.
func postSignificance(p whatsnew.Post, anns []PostAnnouncement) PostSignificance {
	var s PostSignificance
	if slices.Contains(p.Tags, whatsnew.LaunchTag) {
		s.Signals = append(s.Signals, SignalLaunch)
	}
	if strings.HasPrefix(p.URL, newsBlogPrefix) {
		s.Signals = append(s.Signals, SignalNewsBlog)
	}
	if len(anns) > 0 {
		s.Signals = append(s.Signals, SignalAnnouncement)
	}
	s.Kind = postKind(p)
	s.Significant = s.Kind == "" && len(s.Signals) > 0
	return s
}

// postKind returns the kind of a post that is not significant whatever its signals, or "".
func postKind(p whatsnew.Post) string {
	title := strings.ToLower(p.Title)
	has := func(tags ...string) bool {
		return slices.ContainsFunc(tags, func(t string) bool { return slices.Contains(p.Tags, t) })
	}
	switch {
	case strings.Contains(title, "weekly roundup") || strings.Contains(title, "monthly roundup") || strings.HasPrefix(title, "icymi"):
		return KindRoundup
	case has(tagEvents) || strings.Contains(title, "re:invent"):
		return KindEvent
	case has(tagDevTools, tagDeveloperTools) || strings.HasPrefix(p.URL, developerBlogPrefix):
		return KindDeveloperTools
	case has(tagCustomerSolution, tagPartnerSolution) || strings.HasPrefix(p.URL, partnerBlogPrefix):
		return KindCustomerStory
	}
	return ""
}
