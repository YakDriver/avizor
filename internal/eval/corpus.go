// Package eval measures reconciliation quality against a hand-labeled corpus.
package eval

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
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
	// Input is an issue number or URL, as given to `radar candidates`. Item is a pulled item ID. Exactly one of Input,
	// Text, and Item is set.
	Input string `hcl:"input,optional"`
	Text  string `hcl:"text,optional"`
	Item  string `hcl:"item,optional"`
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
	// Same and SamePaths label other candidates that are the same work as the input, such as a second duplicate or the
	// PR that implemented it. They needn't appear, but a confident category on them is correct.
	Same      []int    `hcl:"same,optional"`
	SamePaths []string `hcl:"same_paths,optional"`
	// Different and DifferentPaths label candidates that are not the same work. They may appear, but only as
	// "related": a confident category on them is confidently wrong, and fails the case.
	Different      []int    `hcl:"different,optional"`
	DifferentPaths []string `hcl:"different_paths,optional"`

	asOf time.Time
	// issue is the input's issue number in the bound repo, or 0. Set by Bind.
	issue int
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

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Load reads and validates a corpus file.
func Load(path string) (*Corpus, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading corpus: %w", err)
	}
	return Parse(path, src)
}

// Parse validates a corpus from source. name is used in error messages and must end in ".hcl".
func Parse(name string, src []byte) (*Corpus, error) {
	var c Corpus
	if err := hclsimple.Decode(name, src, nil, &c); err != nil {
		return nil, fmt.Errorf("loading corpus %s: %w", name, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("corpus %s: %w", name, err)
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
	if n := btoi(cs.Input != "") + btoi(cs.Text != "") + btoi(cs.Item != ""); n != 1 {
		return errors.New("set exactly one of input, text, and item")
	}
	switch cs.AsOf {
	case "", AsOfFiled, AsOfNow:
		// Defaults and "filed" depend on the target repo; see Bind.
	default:
		t, err := parseTime(cs.AsOf)
		if err != nil {
			return err
		}
		cs.asOf = t
	}
	if len(cs.Expect) == 0 && cs.Unmatched == nil && len(cs.Different)+len(cs.DifferentPaths) == 0 {
		return errors.New("needs at least one of expect, unmatched, or different")
	}
	if err := cs.validateJudgments(); err != nil {
		return err
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

// validateJudgments checks that no candidate is labeled more than one way.
func (cs *Case) validateJudgments() error {
	seen := map[string]string{}
	add := func(ref, field string) error {
		if prev, ok := seen[ref]; ok {
			return fmt.Errorf("%s is in both %s and %s", ref, prev, field)
		}
		seen[ref] = field
		return nil
	}
	for _, e := range cs.Expect {
		if err := add(expectRef(e), "expect"); err != nil {
			return err
		}
	}
	for field, refs := range map[string][]string{
		"same":            numberRefs(cs.Same),
		"same_paths":      cs.SamePaths,
		"different":       numberRefs(cs.Different),
		"different_paths": cs.DifferentPaths,
	} {
		for _, ref := range refs {
			if err := add(ref, field); err != nil {
				return err
			}
		}
	}
	if (len(cs.SamePaths) > 0 || len(cs.DifferentPaths) > 0) && !cs.Code {
		return errors.New("same_paths and different_paths need code = true")
	}
	return nil
}

func numberRefs(ns []int) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = "#" + strconv.Itoa(n)
	}
	return out
}

// Bind resolves cases against the target repo ("owner/name"): it finds which inputs are the repo's issues, defaults
// as_of to "filed" for those and "now" otherwise, and rejects "filed" for other inputs. It uses the same input parsing
// as reconciliation, so an issue URL with a comment fragment is an issue and another repo's issue URL is not.
func (c *Corpus) Bind(repo string) error {
	var errs []error
	for i := range c.Cases {
		cs := &c.Cases[i]
		cs.issue = 0
		if cs.Input != "" {
			if n, ok := reconcile.IssueNumber(cs.Input, repo); ok {
				cs.issue = n
			}
		}
		switch cs.AsOf {
		case "":
			cs.AsOf = AsOfNow
			if cs.issue != 0 {
				cs.AsOf = AsOfFiled
			}
		case AsOfFiled:
			if cs.issue == 0 {
				errs = append(errs, fmt.Errorf(`case %q: as_of = "filed" needs an issue in %s as input`, cs.Name, repo))
			}
		}
	}
	return errors.Join(errs...)
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
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
