package signal

import (
	"slices"
	"testing"
)

func TestExtract(t *testing.T) {
	t.Parallel()

	const repo = "hashicorp/terraform-provider-aws"
	text := `> Amazon FSx for NetApp ONTAP now supports Amazon S3 access

* [Announcement](https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/)
* See https://www.AWS.amazon.com/blogs/aws/some-post/?utm=x#frag.

Uses CreateAndAttachS3AccessPoint and DescribeS3AccessPointAttachmentsInput. New resource aws_fsx_s3_access_point_attachment.
Requires https://github.com/hashicorp/terraform-provider-aws/pull/45358. Relates #43195 and &#39; and
https://github.com/other/repo/issues/7. Not an op: Getter, Create, Listen.`

	got := Extract(text, repo)
	want := []Signal{
		{KindURL, "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access"},
		{KindURL, "https://aws.amazon.com/blogs/aws/some-post"},
		{KindIssueRef, "45358"},
		{KindURL, "https://github.com/other/repo/issues/7"},
		{KindTFName, "aws_fsx_s3_access_point_attachment"},
		{KindOp, "CreateAndAttachS3AccessPoint"},
		{KindOp, "DescribeS3AccessPointAttachments"},
		{KindIssueRef, "43195"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Extract:\n got %v\nwant %v", got, want)
	}
}

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"http://www.Example.com/a/b/?q=1#x": "https://example.com/a/b",
		"https://example.com/a).":           "https://example.com/a)",
		"not a url":                         "",
		"https://example.com":               "https://example.com",
	} {
		if got := NormalizeURL(in); got != want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeywords(t *testing.T) {
	t.Parallel()

	got := Keywords("r/aws_fsx_s3_access_point_attachment: Add support for the FSx S3 access https://x.com/y", 0)
	want := []string{"fsx", "s3", "access", "point", "attachment"}
	if !slices.Equal(got, want) {
		t.Errorf("Keywords = %v, want %v", got, want)
	}
	if got := Keywords("one two three four", 2); !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("Keywords with limit = %v", got)
	}
}
