package main

import (
	"regexp"
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
	// KindModelAvailability: a model now available in Amazon Bedrock or SageMaker JumpStart.
	KindModelAvailability = "model_availability"
	// KindTransform: AWS Transform, whose agents and jobs the provider doesn't manage.
	KindTransform = "transform"
	// KindOpenSource: the AWS Open Source Blog, about projects outside AWS services.
	KindOpenSource = "open_source"
	// KindRegion: a new Region or Local Zone. New Regions need no provider work.
	KindRegion = "region"
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
	developerBlogPrefix  = "https://aws.amazon.com/blogs/developer/"
	partnerBlogPrefix    = "https://aws.amazon.com/blogs/apn/"
	openSourceBlogPrefix = "https://aws.amazon.com/blogs/opensource/"
)

// Titles the kinds come from, matched lowercase. Model availability names Bedrock or SageMaker JumpStart, since
// "Introducing ... on AWS" is also how launches with work are titled ("Introducing 1:1 NAT on AWS Transit Gateway").
var (
	roundupRE           = regexp.MustCompile(`weekly roundup|monthly roundup|^icymi|^this month in\b`)
	modelAvailabilityRE = regexp.MustCompile(`\bnow (generally )?available (in|on) amazon bedrock\b|\bsagemaker jumpstart\b`)
	transformRE         = regexp.MustCompile(`\baws transform\b|\batx\b`)
	regionRE            = regexp.MustCompile(`^now open\b|\blocal zones? in\b|\bnew (aws )?region\b`)
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

// postKind returns the kind of a post that is not significant whatever its signals, or "". A post can be of more than
// one kind; the first in postKinds is returned.
func postKind(p whatsnew.Post) string {
	for _, k := range postKinds {
		if isKind(p, k) {
			return k
		}
	}
	return ""
}

// postKinds are the kinds of post that are not significant, in the order postKind checks them.
var postKinds = []string{KindRoundup, KindEvent, KindDeveloperTools, KindCustomerStory, KindModelAvailability, KindTransform, KindOpenSource, KindRegion}

// isKind reports whether a post is of a kind.
func isKind(p whatsnew.Post, kind string) bool {
	title := strings.ToLower(p.Title)
	has := func(tags ...string) bool {
		return slices.ContainsFunc(tags, func(t string) bool { return slices.Contains(p.Tags, t) })
	}
	switch kind {
	case KindRoundup:
		return roundupRE.MatchString(title)
	case KindEvent:
		return has(tagEvents) || strings.Contains(title, "re:invent")
	case KindDeveloperTools:
		return has(tagDevTools, tagDeveloperTools) || strings.HasPrefix(p.URL, developerBlogPrefix)
	case KindCustomerStory:
		return has(tagCustomerSolution, tagPartnerSolution) || strings.HasPrefix(p.URL, partnerBlogPrefix)
	case KindModelAvailability:
		return modelAvailabilityRE.MatchString(title)
	case KindTransform:
		return transformRE.MatchString(title)
	case KindOpenSource:
		return strings.HasPrefix(p.URL, openSourceBlogPrefix)
	case KindRegion:
		return regionRE.MatchString(title)
	}
	return false
}
