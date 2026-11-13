package awssdk

import (
	"regexp"
	"strings"
)

// NewModuleNote is the release note for a new SDK service module.
const NewModuleNote = "New AWS service client module"

// ServiceNotes are one service module's entries in an SDK release's notes.
type ServiceNotes struct {
	// Module is the module name under github.com/aws/aws-sdk-go-v2/service/, e.g. "fsx".
	Module string `json:"module"`
	// Version is the module version in the release, e.g. "v1.65.0".
	Version string `json:"version,omitempty"`
	Notes   []Note `json:"notes"`
}

// Note is one release note line.
type Note struct {
	// Kind is the note's label, e.g. "Feature", "Bug Fix", or "Documentation".
	Kind string `json:"kind"`
	Text string `json:"text"`
}

var (
	moduleLineRE = regexp.MustCompile("^\\* `github\\.com/aws/aws-sdk-go-v2/service/([a-z0-9]+)`: \\[?(v[0-9][^\\]\\s]*)")
	noteLineRE   = regexp.MustCompile(`^\s+\* \*\*([^*]+)\*\*: (.*)$`)
)

// ParseNotes reads the "Module Highlights" entries for service modules from an aws-sdk-go-v2 release body. Entries for
// non-service modules (the core module, credentials, feature modules) are ignored.
func ParseNotes(body string) map[string]*ServiceNotes {
	out := map[string]*ServiceNotes{}
	var cur *ServiceNotes
	for line := range strings.Lines(body) {
		line = strings.TrimRight(line, "\r\n")
		if m := moduleLineRE.FindStringSubmatch(line); m != nil {
			cur = &ServiceNotes{Module: m[1], Version: m[2]}
			out[cur.Module] = cur
			continue
		}
		if strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "#") {
			cur = nil
			continue
		}
		if cur == nil {
			continue
		}
		if m := noteLineRE.FindStringSubmatch(line); m != nil {
			cur.Notes = append(cur.Notes, Note{Kind: strings.TrimSpace(m[1]), Text: strings.TrimSpace(m[2])})
		}
	}
	return out
}

// ModuleName returns the aws-sdk-go-v2 service module name for an SDK ID: lowercase letters and digits only, so
// "S3 Control" is "s3control".
func ModuleName(sdkID string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(sdkID) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
