package reconcile

import (
	"slices"
	"time"
)

// Match categories. They are provisional findings for a human or agent to confirm, not dispositions.
const (
	CategoryImplemented = "already_implemented"
	CategoryTracked     = "already_tracked"
	CategoryConsidered  = "previously_considered"
	CategoryRelated     = "related"
)

// Categories in display order.
var Categories = []string{CategoryImplemented, CategoryTracked, CategoryConsidered, CategoryRelated}

// Signal strengths. Strong signals are specific anchors (a URL, an API operation, a Terraform name in a title) that
// rarely appear by coincidence. Weak signals (keywords, labels, cross-references) suggest relatedness only.
const (
	StrengthStrong = "strong"
	StrengthWeak   = "weak"
)

// Match types for cross-references between the input and a candidate.
const (
	matchReferencedByInput = "referenced_by_input"
	matchReferencesInput   = "references_input"
	// matchReferencesCandidate marks a PR that references a strongly matched issue candidate.
	matchReferencesCandidate = "references_candidate"
)

// notIndexedTitle is the title of a cross-referenced issue that isn't in the index.
const notIndexedTitle = "(not in index)"

// Candidate kinds.
const (
	KindIssue = "issue"
	KindPR    = "pr"
	KindCode  = "code"
	KindDoc   = "doc"
)

// Result is the output of a reconciliation.
type Result struct {
	Input      InputInfo   `json:"input"`
	Target     TargetInfo  `json:"target"`
	Evidence   []Evidence  `json:"evidence"`
	Inferences []Inference `json:"inferences"`
	Candidates []Candidate `json:"candidates"`
	Summary    Summary     `json:"summary"`
	Warnings   []string    `json:"warnings,omitempty"`
	Provenance Provenance  `json:"provenance"`
}

// InputInfo describes what was reconciled.
type InputInfo struct {
	// Kind is "issue", "url", or "text".
	Kind   string   `json:"kind"`
	Ref    string   `json:"ref"`
	Number int      `json:"number,omitempty"`
	Title  string   `json:"title,omitempty"`
	URL    string   `json:"url,omitempty"`
	Labels []string `json:"labels,omitempty"`
	// Keywords are the terms used for full-text retrieval.
	Keywords []string `json:"keywords"`
}

// TargetInfo describes what the input was reconciled against.
type TargetInfo struct {
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	IndexSynced string `json:"index_synced_at,omitempty"`
	// AsOf is set when the index was viewed as of an earlier time.
	AsOf        string `json:"as_of,omitempty"`
	Issues      int    `json:"indexed_issues"`
	PRs         int    `json:"indexed_prs"`
	Checkout    string `json:"checkout,omitempty"`
	CheckoutRef string `json:"checkout_ref,omitempty"`
}

// Evidence is an observation about the input, with where it came from.
type Evidence struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Source string `json:"source"`
	// Common is set when the signal appears in too many records to discriminate, so it wasn't used for matching.
	Common bool `json:"common,omitempty"`
}

// Inference is an educated guess about the input, with its basis.
type Inference struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Basis string `json:"basis"`
}

// Candidate is existing work that may correspond to the input.
type Candidate struct {
	Category    string    `json:"category"`
	Kind        string    `json:"kind"`
	Number      int       `json:"number,omitempty"`
	Path        string    `json:"path,omitempty"`
	Title       string    `json:"title,omitempty"`
	State       string    `json:"state,omitempty"`
	StateReason string    `json:"state_reason,omitempty"`
	Merged      bool      `json:"merged,omitempty"`
	URL         string    `json:"url,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitzero"`
	Signals     []Match   `json:"signals"`

	rank float64
}

// Match is one reason a candidate was returned.
type Match struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Where    string `json:"where,omitempty"`
	Strength string `json:"strength"`
}

// Summary counts candidates by category. Unmatched is true when nothing beyond "related" was found.
type Summary struct {
	Counts    map[string]int `json:"counts"`
	Unmatched bool           `json:"unmatched"`
}

// Provenance records how and when the result was produced.
type Provenance struct {
	Tool        string    `json:"tool"`
	Version     string    `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	// Operations is where operation names were checked against, if anywhere.
	Operations string `json:"operations,omitempty"`
}

func (c *Candidate) strongCount() int {
	n := 0
	for _, m := range c.Signals {
		if m.Strength == StrengthStrong {
			n++
		}
	}
	return n
}

// directStrongCount counts strong signals matched by the candidate itself, not inherited by following a reference.
// Sorting by it first keeps an issue ahead of the PRs found through it.
func (c *Candidate) directStrongCount() int {
	n := 0
	for _, m := range c.Signals {
		if m.Strength == StrengthStrong && m.Type != matchReferencesCandidate {
			n++
		}
	}
	return n
}

// crossReferenced reports whether the candidate and the input reference each other directly.
func (c *Candidate) crossReferenced() bool {
	for _, m := range c.Signals {
		if m.Type == matchReferencedByInput || m.Type == matchReferencesInput {
			return true
		}
	}
	return false
}

func (c *Candidate) addSignal(m Match) {
	if slices.Contains(c.Signals, m) {
		return
	}
	c.Signals = append(c.Signals, m)
}
