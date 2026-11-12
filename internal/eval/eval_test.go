package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/index"
	"github.com/YakDriver/radar/internal/reconcile"
)

const repo = "o/r"

func writeCorpus(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "corpus.hcl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ body, want string }{
		"no cases":     {`target = "t"`, "no cases"},
		"both inputs":  {`target = "t"` + "\n" + `case "a" {` + "\n" + `input = "1"` + "\n" + `text = "x"` + "\n" + `kind = "k"` + "\n" + `unmatched = true` + "\n}", "exactly one of input and text"},
		"no labels":    {`target = "t"` + "\n" + `case "a" {` + "\n" + `input = "1"` + "\n" + `kind = "k"` + "\n}", "at least one of"},
		"bad category": {`target = "t"` + "\n" + `case "a" {` + "\n" + `input = "1"` + "\n" + `kind = "k"` + "\n" + `expect {` + "\n" + `number = 2` + "\n" + `category = "dup"` + "\n}\n}", "unknown category"},
		"filed text":   {`target = "t"` + "\n" + `case "a" {` + "\n" + `text = "x"` + "\n" + `as_of = "filed"` + "\n" + `kind = "k"` + "\n" + `unmatched = true` + "\n}", "needs an issue input"},
		"path no code": {`target = "t"` + "\n" + `case "a" {` + "\n" + `input = "1"` + "\n" + `kind = "k"` + "\n" + `expect {` + "\n" + `path = "x.go"` + "\n}\n}", "needs code = true"},
		"bad name":     {`target = "t"` + "\n" + `case "A b" {` + "\n" + `input = "1"` + "\n" + `kind = "k"` + "\n" + `unmatched = true` + "\n}", "name must be"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(writeCorpus(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestLoadAsOfDefaults(t *testing.T) {
	t.Parallel()

	c, err := Load(writeCorpus(t, `
target = "t"
case "issue" {
  input     = "12"
  kind      = "duplicate"
  unmatched = false
}
case "text" {
  text      = "widgets"
  kind      = "no-work"
  unmatched = true
}
case "dated" {
  input     = "12"
  as_of     = "2026-01-02"
  kind      = "duplicate"
  unmatched = false
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Cases[0].AsOf != AsOfFiled || c.Cases[1].AsOf != AsOfNow {
		t.Errorf("defaults = %q, %q", c.Cases[0].AsOf, c.Cases[1].AsOf)
	}
	if got, want := c.Cases[2].AsOfTime(), time.Date(2026, 1, 2, 23, 59, 59, 0, time.UTC); !got.Equal(want) {
		t.Errorf("dated as_of = %s, want %s", got, want)
	}
}

func seed(t *testing.T) *index.Store {
	t.Helper()
	ctx := context.Background()
	st, err := index.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	tx, err := st.Begin(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range []*index.Issue{
		// Original request, later closed when the resource shipped.
		{Number: 1, Kind: index.KindIssue, State: "closed", StateReason: "completed", ClosedAt: day(9), CreatedAt: day(1),
			Labels: []string{"service/widget"}, Title: "Support CreateWidget", Body: "AWS added CreateWidget."},
		// Duplicate filed later.
		{Number: 2, Kind: index.KindIssue, State: "closed", StateReason: "duplicate", ClosedAt: day(6), CreatedAt: day(5),
			Labels: []string{"service/widget"}, Title: "Widgets please", Body: "Needs CreateWidget."},
		// PR filed after the duplicate.
		{Number: 3, Kind: index.KindPR, State: "closed", Merged: true, MergedAt: day(9), ClosedAt: day(9), CreatedAt: day(8),
			Labels: []string{"service/widget"}, Title: "New resource: widget", Body: "Closes #1. Calls CreateWidget."},
		// Same service, unrelated.
		{Number: 4, Kind: index.KindIssue, State: "open", CreatedAt: day(2),
			Labels: []string{"service/widget"}, Title: "Widget gizmo crash", Body: "panic"},
	} {
		is.UpdatedAt = day(10)
		if err := tx.UpsertIssue(ctx, is, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestRun(t *testing.T) {
	t.Parallel()

	path := writeCorpus(t, `
target = "t"

case "dup-as-filed" {
  input = "2"
  kind  = "duplicate"
  expect {
    number   = 1
    category = "already_tracked"
  }
  related_only = [4]
  unmatched    = false
}

case "dup-now" {
  input = "2"
  as_of = "now"
  kind  = "duplicate"
  expect {
    number   = 1
    category = "already_tracked"
  }
}

case "missing" {
  input = "99"
  kind  = "duplicate"
  expect {
    number = 1
  }
}

case "nothing" {
  text      = "unrelated gadget"
  kind      = "no-work"
  unmatched = true
}
`)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{Base: &reconcile.Reconciler{
		Store:  seed(t),
		Target: &config.Target{Name: "t", Repo: repo, ServiceLabelPrefix: "service/"},
	}}
	rep := r.Run(context.Background(), c, path)

	byName := map[string]CaseResult{}
	for _, cr := range rep.Cases {
		byName[cr.Name] = cr
	}

	// As filed on day 5, #1 was open and #3 didn't exist: #1 is "tracked" at rank 1.
	cr := byName["dup-as-filed"]
	if !cr.Pass || !cr.Hit[1] || cr.AsOf != "2026-01-05T00:00:00Z" {
		t.Errorf("dup-as-filed = %+v", cr)
	}
	for _, top := range cr.Top {
		if top.Ref == "#3" {
			t.Error("#3 visible before it was created")
		}
	}

	// Now, #1 is closed, so it's "previously considered" and the category label fails.
	cr = byName["dup-now"]
	if cr.Pass || cr.Expected[0].Got != reconcile.CategoryConsidered || *cr.Expected[0].Category {
		t.Errorf("dup-now = %+v", cr)
	}

	if cr := byName["missing"]; cr.Pass || cr.Error == "" {
		t.Errorf("missing = %+v", cr)
	}
	if cr := byName["nothing"]; !cr.Pass {
		t.Errorf("nothing = %+v", cr)
	}

	m := rep.Overall
	if m.Cases != 4 || m.Passed != 2 || m.Errors != 1 || m.WithExpect != 2 {
		t.Errorf("overall = %+v", m)
	}
	if m.Hit[1] != 0.5 || m.Hit[3] != 1 || m.CategoryLabeled != 2 || m.CategoryAccuracy != 0.5 || m.RelatedOnlyLabeled != 1 || m.FalseEquivalent != 0 {
		t.Errorf("overall = %+v", m)
	}
	if rep.ByKind["no-work"].UnmatchedAccuracy != 1 {
		t.Errorf("no-work = %+v", rep.ByKind["no-work"])
	}
}
