package enrich

import (
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

func TestTaggedService(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		tag, service string
		want         bool
	}{
		{"blog-posts#category-database-amazon-aurora#dsql", "DSQL", true},
		{"blog-posts#category-artificial-intelligence#amazon-sagemaker-ai", "SageMaker", true},
		{"blog-posts#category-compute#aws-lambda", "Lambda", true},
		{"blog-posts#category-compute#aws-lambda", "Lambda Core", false},
		{"GLOBAL#content-type#launch-announcement", "Launch", false},
		{"blog-posts#category-storage#amazon-s3", "DSQL", false},
		{"blog-posts#category-security-identity-compliance#aws-secrets-manager", "ECR", false},
		{"blog-posts#category-security-identity-compliance#aws-secrets-manager", "Secrets Manager", true},
	} {
		if got := taggedService([]string{tc.tag}, strings.Join(ServiceWords(tc.service), "")); got != tc.want {
			t.Errorf("taggedService(%s, %s) = %t", tc.tag, tc.service, got)
		}
	}
}

func TestBlogPostsBoost(t *testing.T) {
	t.Parallel()

	rel := time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC)
	posts := whatsnew.NewMatcher(whatsnew.PostsAsAnnouncements([]whatsnew.Post{
		// The same text; the launch announcement tagged with the service ranks first.
		{URL: "u-plain", Title: "Aurora DSQL query statistics", PostedAt: rel.Add(time.Hour)},
		{URL: "u-launch", Title: "Aurora DSQL query statistics", PostedAt: rel.Add(2 * time.Hour),
			Tags: []string{whatsnew.LaunchTag, "blog-posts#category-database-amazon-aurora#dsql"}},
		{URL: "u-other", Title: "Amazon S3 annotations", PostedAt: rel.Add(time.Hour)},
	}))
	it := &awssdk.Item{Service: awssdk.Service{SDKID: "DSQL", Module: "dsql"}, Release: awssdk.Release{PublishedAt: rel},
		Notes: []awssdk.Note{{Text: "Adds query statistics."}}}
	got := BlogPosts(posts, it, nil, 2)
	if len(got) != 2 || got[0].URL != "u-launch" || got[1].URL != "u-plain" || got[0].Score != got[1].Score || !got[0].Launch || !got[0].TaggedService {
		t.Errorf("BlogPosts = %+v", got)
	}
	if got := BlogPosts(nil, it, nil, 5); got == nil || len(got) != 0 {
		t.Errorf("BlogPosts without posts = %#v", got)
	}
}
