package main

import (
	"bytes"
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/YakDriver/radar/internal/eval"
)

// TestWriteUnjudgedErrors checks that errored cases are reported, so an evaluation that couldn't run doesn't look
// fully labeled.
func TestWriteUnjudgedErrors(t *testing.T) {
	t.Parallel()

	rep := &eval.Report{Cases: []eval.CaseResult{
		{Name: "ok", Input: "1", Confident: []eval.Judged{{Rank: 1, Ref: "#2", Category: "already_tracked", Verdict: eval.VerdictUnjudged}}},
		{Name: "broken", Input: "3", Error: "#3 is not in the index"},
	}}
	var b bytes.Buffer
	if err := writeUnjudged(&b, rep); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"1 unjudged confident candidate(s)", "1 case(s) errored", "broken: #3 is not in the index"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestShellJoin(t *testing.T) {
	t.Parallel()

	got := shellJoin([]string{"issue", "--title", "New resource: `aws_x`", "--label", "service/x", "it's", ""})
	want := `issue --title 'New resource: ` + "`aws_x`" + `' --label service/x 'it'\''s' ''`
	if got != want {
		t.Errorf("shellJoin = %s, want %s", got, want)
	}
}

func TestRegenArgs(t *testing.T) {
	t.Parallel()

	fs := flag.NewFlagSet("draft", flag.ContinueOnError)
	var c common
	c.register(fs)
	fs.Bool("no-code", false, "")
	fs.Bool("body", false, "")
	if err := fs.Parse([]string{"--config", "/x/radar.hcl", "--state-dir", "/s", "--format", "text", "--no-code"}); err != nil {
		t.Fatal(err)
	}
	got := regenArgs(fs, "sdk/x/release-1", "tf")
	want := []string{"radar", "draft", "sdk/x/release-1", "--config=/x/radar.hcl", "--no-code=true", "--state-dir=/s", "--target=tf", "--body"}
	if !slices.Equal(got, want) {
		t.Errorf("regenArgs = %q, want %q", got, want)
	}
}
