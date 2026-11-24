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
	// KindCompliance: a compliance report, guide, package, accreditation, or audit.
	KindCompliance = "compliance"
	// KindTraining: AWS Training and Certification, Skill Builder, and badges. Not model training.
	KindTraining = "training"
	// KindRecognition: an analyst report naming AWS a leader.
	KindRecognition = "recognition"
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

// Titles the kinds come from, matched lowercase. Model availability names Bedrock ("now available in Amazon Bedrock",
// "Amazon Bedrock expands ... model availability"), SageMaker JumpStart, or a model family with Bedrock, SageMaker, or
// "on AWS" ("Introducing Claude Opus 5 on AWS"), since "Introducing ... on AWS" alone is also how launches with work
// are titled ("Introducing 1:1 NAT on AWS Transit Gateway"). Training names AWS Training and Certification or its
// programs, not "training", which is also model training ("Checkpointless training on Amazon SageMaker HyperPod").
var (
	roundupRE           = regexp.MustCompile(`weekly roundup|monthly roundup|^icymi|^this month in\b`)
	modelAvailabilityRE = regexp.MustCompile(`\bnow (generally )?available (in|on) amazon bedrock\b|\bsagemaker jumpstart\b|\bamazon bedrock expands\b.*\bmodel availability\b|` +
		`\b(claude|gpt-[\d.]+|gpt oss|openai (gpt|models?)|gemma|grok|llama|mistral|deepseek|qwen|kimi|nemotron)\b.*\b(amazon bedrock|on aws|sagemaker)\b`)
	transformRE  = regexp.MustCompile(`\baws transform\b|\batx\b`)
	regionRE     = regexp.MustCompile(`^now open\b|\blocal zones? in\b|\bnew (aws )?region\b`)
	complianceRE = regexp.MustCompile(`\bsoc [123]\b|\biso(/iec)? \d{4,5}\b|\biso\b.*\bcertificat|\bpci\b|\birap\b|\bhitrust\b|\bc5\b|\bdspt\b|` +
		`\bfedramp\b|\bhipaa\b|\baccreditation\b|\battestation\b|\bcertification audit\b|\brenews\b.*\bcertification\b|` +
		`\bcompliance (guide|guidance|package)|\bassessment report\b|\breports? (is|are) (now )?available\b|\bcritical third party\b`)
	trainingRE    = regexp.MustCompile(`\baws training and certification\b|\baws certifi(cation|ed)\b|\bcertification exam|\bskill builder\b|\bbadges?\b|\bcloud quest\b|\bsimulearn\b|\bvirtual training\b`)
	recognitionRE = regexp.MustCompile(`\bnamed (a )?leader\b|\b(recognized|named) as a leader\b|\ba leader in\b|\bmagic quadrant\b|\bmarketscape\b`)
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
var postKinds = []string{KindRoundup, KindEvent, KindDeveloperTools, KindCustomerStory, KindModelAvailability, KindTransform, KindOpenSource, KindRegion, KindCompliance, KindTraining, KindRecognition}

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
	case KindCompliance:
		return complianceRE.MatchString(title)
	case KindTraining:
		return trainingRE.MatchString(title)
	case KindRecognition:
		return recognitionRE.MatchString(title)
	}
	return false
}
