// Package eval measures reconciliation quality against a hand-labeled corpus.
package eval

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/YakDriver/radar/internal/reconcile"
)

// Corpus is a set of labeled cases for one target.
type Corpus struct {
	// Target names the config target the cases were labeled against.
	Target string `hcl:"target"`
	Cases  []Case `hcl:"case,block"`
}

// Case is one labeled reconciliation.
type Case struct {
	Name string `hcl:"name,label"`
	// Input is an issue number or URL, as given to `radar candidates`. Exactly one of Input and Text is set.
	Input string `hcl:"input,optional"`
	Text  string `hcl:"text,optional"`
	// AsOf reconciles against the index as it was at a time, so later activity, such as the answer itself being filed,
	// doesn't leak into the result. It is RFC 3339, YYYY-MM-DD (end of that day), "filed" (when the input issue was
	// created), or "now". The default is "filed" for issue inputs and "now" otherwise.
	AsOf string `hcl:"as_of,optional"`
	// Code enables checkout searches. They reflect the checkout as it is now, not as of AsOf, so they're off by default.
	Code bool `hcl:"code,optional"`
	// Kind groups cases for reporting, e.g. "duplicate", "announcement", "implemented", "no-work".
	Kind string `hcl:"kind"`
	Note string `hcl:"note,optional"`

	// Expect lists the candidates a correct result contains.
	Expect []Expect `hcl:"expect,block"`
	// Unmatched, if set, is the expected value of summary.unmatched.
	Unmatched *bool `hcl:"unmatched,optional"`
	// RelatedOnly lists issues and PRs that are related but not equivalent. They may appear, but only as "related".
	RelatedOnly []int `hcl:"related_only,optional"`

	asOf time.Time
}

// Expect is one candidate a correct result contains.
type Expect struct {
	Number int    `hcl:"number,optional"`
	Path   string `hcl:"path,optional"`
	// Category, if set, is the category the candidate should have.
	Category string `hcl:"category,optional"`
}

// Special as_of values.
const (
	AsOfFiled = "filed"
	AsOfNow   = "now"
)

var (
	nameRE       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	issueInputRE = regexp.MustCompile(`^(?:#?\d+|https://github\.com/[^/]+/[^/]+/(?:issues|pull)/\d+)$`)
)

// Load reads and validates a corpus file.
func Load(path string) (*Corpus, error) {
	var c Corpus
	if err := hclsimple.DecodeFile(path, nil, &c); err != nil {
		return nil, fmt.Errorf("loading corpus %s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("corpus %s: %w", path, err)
	}
	return &c, nil
}

func (c *Corpus) validate() error {
	if len(c.Cases) == 0 {
		return errors.New("no cases")
	}
	seen := map[string]bool{}
	var errs []error
	for i := range c.Cases {
		cs := &c.Cases[i]
		if err := cs.validate(); err != nil {
			errs = append(errs, fmt.Errorf("case %q: %w", cs.Name, err))
		}
		if seen[cs.Name] {
			errs = append(errs, fmt.Errorf("duplicate case %q", cs.Name))
		}
		seen[cs.Name] = true
	}
	return errors.Join(errs...)
}

func (cs *Case) validate() error {
	if !nameRE.MatchString(cs.Name) {
		return errors.New("name must be lowercase letters, digits, and dashes")
	}
	if (cs.Input == "") == (cs.Text == "") {
		return errors.New("set exactly one of input and text")
	}
	switch cs.AsOf {
	case "":
		cs.AsOf = AsOfNow
		if issueInputRE.MatchString(cs.Input) {
			cs.AsOf = AsOfFiled
		}
	case AsOfFiled:
		if !issueInputRE.MatchString(cs.Input) {
			return errors.New(`as_of = "filed" needs an issue input`)
		}
	case AsOfNow:
	default:
		t, err := parseTime(cs.AsOf)
		if err != nil {
			return err
		}
		cs.asOf = t
	}
	if len(cs.Expect) == 0 && cs.Unmatched == nil && len(cs.RelatedOnly) == 0 {
		return errors.New("needs at least one of expect, unmatched, or related_only")
	}
	for _, e := range cs.Expect {
		if (e.Number == 0) == (e.Path == "") {
			return errors.New("expect: set exactly one of number and path")
		}
		if e.Category != "" && !slices.Contains(reconcile.Categories, e.Category) {
			return fmt.Errorf("expect: unknown category %q", e.Category)
		}
		if e.Path != "" && !cs.Code {
			return fmt.Errorf("expect path %s needs code = true", e.Path)
		}
		if slices.Contains(cs.RelatedOnly, e.Number) && e.Number != 0 {
			return fmt.Errorf("#%d is in both expect and related_only", e.Number)
		}
	}
	if cs.Unmatched != nil && *cs.Unmatched {
		for _, e := range cs.Expect {
			if e.Category != "" && e.Category != reconcile.CategoryRelated {
				return errors.New("unmatched = true conflicts with an expected non-related category")
			}
		}
	}
	return nil
}

// AsOfTime returns the parsed as_of time. It is zero for "now" and "filed"; the runner resolves "filed".
func (cs *Case) AsOfTime() time.Time {
	return cs.asOf
}

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		// A date means the end of that day: everything filed that day is visible.
		return t.Add(24*time.Hour - time.Second), nil
	}
	return time.Time{}, fmt.Errorf("as_of %q: use YYYY-MM-DD or RFC 3339", s)
}
