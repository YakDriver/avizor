package main

import (
	"slices"
	"testing"

	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestPostSignificance(t *testing.T) {
	t.Parallel()

	ann := []PostAnnouncement{{URL: "https://aws.amazon.com/about-aws/whats-new/2026/11/x/"}}
	launch := []string{whatsnew.LaunchTag}
	for name, tc := range map[string]struct {
		p       whatsnew.Post
		anns    []PostAnnouncement
		sig     bool
		signals []string
		kind    string
	}{
		"launch":       {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/compute/a/", Title: "Introducing A", Tags: launch}, sig: true, signals: []string{SignalLaunch}},
		"news blog":    {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/a/", Title: "A"}, sig: true, signals: []string{SignalNewsBlog}},
		"announcement": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/database/a/", Title: "How to use A"}, anns: ann, sig: true, signals: []string{SignalAnnouncement}},
		"no signals":   {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/database/a/", Title: "How to use A"}},
		"all signals": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/a/", Title: "A", Tags: launch}, anns: ann, sig: true,
			signals: []string{SignalLaunch, SignalNewsBlog, SignalAnnouncement}},
		"weekly roundup": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/r/", Title: "AWS Weekly Roundup: A, B, and more", Tags: launch},
			signals: []string{SignalLaunch, SignalNewsBlog}, kind: KindRoundup},
		"monthly roundup title": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/m/", Title: "AWS Monthly Roundup: November"},
			signals: []string{SignalNewsBlog}, kind: KindRoundup},
		"monthly roundup": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/r/", Title: "ICYMI: What landed in October", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindRoundup},
		"event tag": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/security/e/", Title: "Sessions you won't miss", Tags: append(slices.Clone(launch), tagEvents)},
			signals: []string{SignalLaunch}, kind: KindEvent},
		"event title": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/storage/e/", Title: "Your guide to storage at re:Invent 2026", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindEvent},
		"developer tools tag": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/d/", Title: "AWS Emulator", Tags: append(slices.Clone(launch), tagDevTools)},
			signals: []string{SignalLaunch, SignalNewsBlog}, kind: KindDeveloperTools},
		"developer blog": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/developer/d/", Title: "Presigned URL downloads"}, anns: ann,
			signals: []string{SignalAnnouncement}, kind: KindDeveloperTools},
		"customer story": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/c/", Title: "How A uses B", Tags: []string{tagCustomerSolution}}, anns: ann,
			signals: []string{SignalAnnouncement}, kind: KindCustomerStory},
		"partner blog": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/apn/c/", Title: "A launches B"}, anns: ann,
			signals: []string{SignalAnnouncement}, kind: KindCustomerStory},
		"this month in": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/mt/m/", Title: "This Month in AWS DevOps Agent: November 2026", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindRoundup},
		"model in bedrock": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/m/", Title: "Pegasus 1.5 from TwelveLabs is now available in Amazon Bedrock"},
			signals: []string{SignalNewsBlog}, kind: KindModelAvailability},
		"bedrock expands models": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/m/", Title: "Amazon Bedrock expands Claude model availability to in-country inferencing in India"},
			anns: ann, signals: []string{SignalAnnouncement}, kind: KindModelAvailability},
		"model in jumpstart": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/m/", Title: "PolyAI Dialog-RSN-1 is now on Amazon SageMaker JumpStart", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindModelAvailability},
		"introducing on aws": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/networking-and-content-delivery/n/", Title: "Introducing 1:1 NAT on AWS Transit Gateway and AWS Cloud WAN", Tags: launch},
			sig: true, signals: []string{SignalLaunch}},
		"bedrock feature": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/w/", Title: "Introducing Web Search on Amazon Bedrock for foundation model grounding", Tags: launch},
			sig: true, signals: []string{SignalLaunch}},
		"transform": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/migration-and-modernization/t/", Title: "Modernize .NET applications from the command line with ATX CLI", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindTransform},
		"transformation isn't transform": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/enterprise-strategy/t/", Title: "Why executive sponsorship drives AWS transformation success", Tags: launch},
			sig: true, signals: []string{SignalLaunch}},
		"open source blog": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/opensource/o/", Title: "Introducing Strands Box", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindOpenSource},
		"soc report": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/security/s/", Title: "Summer 2026 SOC 1 report is now available with 185 services in scope", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindCompliance},
		"compliance guide": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/security/s/", Title: "New compliance guide available: ISO/IEC 42001:2023 on AWS", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindCompliance},
		"accreditation": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/security/s/", Title: "AWS achieves Spain's national Limited Distribution accreditation", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindCompliance},
		"certification": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/training-and-certification/c/", Title: "Updates to AWS Certified Machine Learning Engineer - Associate", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindTraining},
		"model training": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/c/", Title: "Checkpointless training on Amazon SageMaker HyperPod", Tags: launch},
			sig: true, signals: []string{SignalLaunch}},
		"analyst": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/desktop-and-application-streaming/g/", Title: "AWS recognized as a Leader in the 2026 Gartner Magic Quadrant for Desktop as a Service", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindRecognition},
		"model on aws": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/c/", Title: "Introducing Claude Opus 5 on AWS: Anthropic's most capable Opus model", Tags: launch}, anns: ann,
			signals: []string{SignalLaunch, SignalNewsBlog, SignalAnnouncement}, kind: KindModelAvailability},
		"model feature on bedrock": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/c/", Title: "Introducing Grok on Amazon Bedrock", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindModelAvailability},
		"platform first": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/c/", Title: "Amazon Bedrock introduces Grok", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindModelAvailability},
		"gpt-oss": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/c/", Title: "Run gpt-oss models on Amazon SageMaker AI", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindModelAvailability},
		"gpt-4o": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/c/", Title: "Use GPT-4o with Amazon Bedrock", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindModelAvailability},
		"c5 report": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/security/s/", Title: "AWS achieves 2025 C5 Type 2 attestation report with 183 services in scope", Tags: launch},
			signals: []string{SignalLaunch}, kind: KindCompliance},
		"c5 instances": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/c/", Title: "Introducing Amazon EC2 C5 instances", Tags: launch},
			sig: true, signals: []string{SignalLaunch, SignalNewsBlog}},
		"anthropic api": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/machine-learning/t/", Title: "Introducing the Amazon Bedrock Trusted Access API with Anthropic's enterprise frontier safeguards", Tags: launch},
			sig: true, signals: []string{SignalLaunch}},
		"local zone": {p: whatsnew.Post{URL: "https://aws.amazon.com/blogs/aws/l/", Title: "Now Open—AWS Local Zones in Bogotá, Colombia"},
			signals: []string{SignalNewsBlog}, kind: KindRegion},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := postSignificance(tc.p, tc.anns)
			if got.Significant != tc.sig || !slices.Equal(got.Signals, tc.signals) || got.Kind != tc.kind {
				t.Errorf("got %+v, want significant %t, signals %v, kind %q", got, tc.sig, tc.signals, tc.kind)
			}
		})
	}
}

func TestPostCounts(t *testing.T) {
	t.Parallel()

	c := PostCounts{Counts: map[string]int{}}
	for _, s := range []string{PostOpen, PostCovered, PostCovered, PostNoWork, PostDeferred, PostCovered} {
		c.add(s)
	}
	// Coverage leaves out no-work and deferred posts.
	if c.Posts != 6 || c.Counts[PostCovered] != 3 || c.Coverage != 0.75 {
		t.Errorf("counts = %+v", c)
	}
}
