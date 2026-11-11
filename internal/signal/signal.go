// Package signal extracts lexical anchors (URLs, AWS operation names, Terraform names, issue references) and keywords from text.
package signal

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Kinds of exact-match signals stored in the index.
const (
	KindURL      = "url"
	KindOp       = "op"
	KindTFName   = "tfname"
	KindIssueRef = "issue_ref"
)

// Signal is one exact-match anchor found in text.
type Signal struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

var (
	urlRE = regexp.MustCompile(`https?://[^\s<>()\[\]"'` + "`" + `|]+`)

	// tfNameRE matches Terraform type names such as aws_fsx_s3_access_point_attachment.
	tfNameRE = regexp.MustCompile(`\baws_[a-z0-9]+(?:_[a-z0-9]+)+\b`)

	// opRE matches PascalCase identifiers that start with a common AWS API verb and have at least one more segment.
	opRE = regexp.MustCompile(`\b(?:` + strings.Join(opVerbs, "|") + `)[A-Z0-9][A-Za-z0-9]*\b`)

	// issueRefRE matches "#123" not preceded by a word character or '&' (HTML entities).
	issueRefRE = regexp.MustCompile(`(?:^|[^\w&/])#(\d{1,7})\b`)

	ghIssueURLRE = regexp.MustCompile(`^https://github\.com/([^/]+/[^/]+)/(?:issues|pull)/(\d+)`)

	wordRE = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*`)
)

// opVerbs are leading verbs of AWS API operation names. The list favors precision: words that commonly start ordinary
// PascalCase identifiers in Terraform configuration or Go code are omitted.
var opVerbs = []string{
	"Accept", "Activate", "Add", "Allocate", "Apply", "Assign", "Associate", "Attach", "Authorize",
	"Batch", "Cancel", "Copy", "Create", "Deactivate", "Delete", "Deprovision", "Deregister", "Describe",
	"Detach", "Disable", "Disassociate", "Enable", "Export", "Get", "Import", "Invoke", "List", "Modify",
	"Promote", "Provision", "Publish", "Purchase", "Put", "Reboot", "Register", "Reject", "Release",
	"Remove", "Replace", "Reset", "Restore", "Revoke", "Rotate", "Start", "Stop", "Tag", "Terminate",
	"Unassign", "Untag", "Update", "Upgrade",
}

// goSDKSuffixes are stripped from operation-like identifiers so CreateBucketInput matches CreateBucket.
var goSDKSuffixes = []string{"WithContext", "Input", "Output", "Command", "Paginator", "Waiter"}

// Extract returns the distinct signals in text. repo ("owner/name") scopes issue references; GitHub issue and PR URLs
// for that repo become issue_ref signals instead of url signals.
func Extract(text, repo string) []Signal {
	seen := map[Signal]bool{}
	var out []Signal
	add := func(s Signal) {
		if s.Value == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	for _, raw := range urlRE.FindAllString(text, -1) {
		u := NormalizeURL(raw)
		if u == "" {
			continue
		}
		if m := ghIssueURLRE.FindStringSubmatch(u); m != nil && strings.EqualFold(m[1], repo) {
			add(Signal{Kind: KindIssueRef, Value: m[2]})
			continue
		}
		add(Signal{Kind: KindURL, Value: u})
	}

	// Remove URLs before looking for identifiers so path segments don't produce false matches.
	stripped := urlRE.ReplaceAllString(text, " ")

	for _, n := range tfNameRE.FindAllString(stripped, -1) {
		add(Signal{Kind: KindTFName, Value: n})
	}
	for _, op := range opRE.FindAllString(stripped, -1) {
		add(Signal{Kind: KindOp, Value: trimSDKSuffix(op)})
	}
	for _, m := range issueRefRE.FindAllStringSubmatch(stripped, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			add(Signal{Kind: KindIssueRef, Value: m[1]})
		}
	}

	return out
}

func trimSDKSuffix(op string) string {
	for _, s := range goSDKSuffixes {
		if t, ok := strings.CutSuffix(op, s); ok && opRE.MatchString(t) {
			return t
		}
	}
	return op
}

// NormalizeURL canonicalizes a URL for exact matching: https scheme, lowercase host without "www.", no query, fragment,
// or trailing slash, and trailing punctuation removed. It returns "" for URLs that don't parse or have no host.
func NormalizeURL(raw string) string {
	raw = strings.TrimRight(raw, ".,;:!?*_~'\"")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	path := strings.TrimRight(u.EscapedPath(), "/")
	return "https://" + host + path
}

// stopwords are dropped from keyword queries. Besides common English words, they include words that appear in most
// provider issues and so carry no information.
var stopwords = map[string]bool{
	"a": true, "about": true, "add": true, "added": true, "adding": true, "adds": true, "all": true, "allow": true,
	"allows": true, "also": true, "amazon": true, "an": true, "and": true, "any": true, "api": true, "apis": true,
	"are": true, "argument": true, "arguments": true, "as": true, "at": true, "attribute": true, "attributes": true,
	"available": true, "aws": true, "be": true, "been": true, "block": true, "but": true, "by": true, "can": true,
	"could": true, "data": true, "do": true, "does": true, "enhancement": true, "feature": true, "for": true,
	"from": true, "has": true, "have": true, "how": true, "i": true, "if": true, "in": true, "into": true,
	"is": true, "it": true, "its": true, "new": true, "no": true, "not": true, "now": true, "of": true, "on": true,
	"or": true, "our": true, "please": true, "provider": true, "request": true,
	"resource": true, "resources": true, "should": true, "so": true, "source": true, "support": true,
	"supported": true, "supporting": true, "supports": true, "terraform": true, "that": true, "the": true,
	"their": true, "them": true, "then": true, "there": true, "these": true, "this": true, "to": true,
	"use": true, "using": true, "via": true, "want": true, "was": true, "we": true, "when": true, "which": true,
	"will": true, "with": true, "would": true, "you": true, "your": true,
}

// Keywords returns distinct, lowercased content words from text, in order of first appearance, up to limit (0 means no
// limit). Terraform names are split into their parts, without the "aws" prefix.
func Keywords(text string, limit int) []string {
	text = urlRE.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "_", " ")
	var out []string
	for _, w := range wordRE.FindAllString(text, -1) {
		w = strings.ToLower(w)
		if len(w) < 2 || stopwords[w] || slices.Contains(out, w) {
			continue
		}
		out = append(out, w)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}
