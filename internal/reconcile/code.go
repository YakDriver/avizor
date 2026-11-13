package reconcile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// docDirs are website/docs subdirectories that hold one page per Terraform type.
var docDirs = []string{"r", "d", "ephemeral-resources", "list-resources", "actions"}

// maxCodeFiles caps per-operation code hits; operations found in more files are too generic to be useful.
const maxCodeFiles = 10

// searchCode looks for the input's operations and Terraform names in the target checkout.
func (r *Reconciler) searchCode(ctx context.Context, ops, tfNames, pkgs []string) []Candidate {
	co := r.Target.Checkout
	if co == "" {
		return nil
	}
	if _, err := os.Stat(co); err != nil {
		r.warnf("checkout %s is unavailable: %v", co, err)
		r.codeFailed = true
		return nil
	}

	byPath := map[string]*Candidate{}
	add := func(kind, path string, m Match) {
		c, ok := byPath[path]
		if !ok {
			c = &Candidate{Category: CategoryImplemented, Kind: kind, Path: path}
			byPath[path] = c
		}
		c.addSignal(m)
	}

	// Terraform names: a docs page or a registration annotation means the type exists.
	for _, name := range tfNames {
		short := strings.TrimPrefix(name, "aws_")
		for _, dir := range docDirs {
			rel := filepath.Join("website", "docs", dir, short+".html.markdown")
			if _, err := os.Stat(filepath.Join(co, rel)); err == nil {
				add(KindDoc, rel, Match{Type: "tfname", Value: name, Where: "docs", Strength: StrengthStrong})
			}
		}
		files, err := r.grep(ctx, `("`+name+`"`, r.servicePaths(pkgs))
		if err != nil {
			r.warnf("code search for %s failed: %v", name, err)
			r.codeFailed = true
			continue
		}
		for _, f := range files {
			add(KindCode, f, Match{Type: "tfname", Value: name, Where: "registration", Strength: StrengthStrong})
		}
	}

	// API operations: calls in non-test provider code.
	for _, op := range ops {
		files, err := r.grepWord(ctx, op, r.servicePaths(pkgs))
		if err != nil {
			r.warnf("code search for %s failed: %v", op, err)
			r.codeFailed = true
			continue
		}
		if len(files) > maxCodeFiles {
			r.warnf("skipped code matches for op:%s (found in %d files)", op, len(files))
			continue
		}
		for _, f := range files {
			add(KindCode, f, Match{Type: "op", Value: op, Where: "code", Strength: StrengthStrong})
		}
	}

	out := make([]Candidate, 0, len(byPath))
	for _, c := range byPath {
		out = append(out, *c)
	}
	return out
}

// servicePaths returns the directories to search: the detected service packages, or all of internal/service.
func (r *Reconciler) servicePaths(pkgs []string) []string {
	var dirs []string
	for _, p := range pkgs {
		d := filepath.Join("internal", "service", p)
		if fi, err := os.Stat(filepath.Join(r.Target.Checkout, d)); err == nil && fi.IsDir() {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		dirs = []string{filepath.Join("internal", "service")}
	}
	return dirs
}

// grep returns files under dirs (relative to the checkout) containing a fixed string.
func (r *Reconciler) grep(ctx context.Context, fixed string, dirs []string) ([]string, error) {
	return r.rg(ctx, append([]string{"--fixed-strings", "--", fixed}, dirs...))
}

// grepWord returns non-test Go files under dirs containing word as a whole word.
func (r *Reconciler) grepWord(ctx context.Context, word string, dirs []string) ([]string, error) {
	return r.rg(ctx, append([]string{"--fixed-strings", "--word-regexp", "--glob", "!*_test.go", "--", word}, dirs...))
}

func (r *Reconciler) rg(ctx context.Context, args []string) ([]string, error) {
	if r.rgPath == "" {
		p, err := exec.LookPath("rg")
		if err != nil {
			return nil, errors.New("ripgrep (rg) is not installed")
		}
		r.rgPath = p
	}
	cmd := exec.CommandContext(ctx, r.rgPath, append([]string{"--files-with-matches", "--type", "go", "--sort", "path"}, args...)...)
	cmd.Dir = r.Target.Checkout
	out, err := cmd.Output()
	if err != nil {
		// Exit status 1 means no matches.
		if ee, ok := errors.AsType[*exec.ExitError](err); ok && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("rg: %w", err)
	}
	var files []string
	for l := range strings.Lines(string(out)) {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, filepath.ToSlash(l))
		}
	}
	slices.Sort(files)
	return files, nil
}

// checkoutRef returns "branch@shortsha" for the checkout, or "" if it isn't a git repository.
func checkoutRef(ctx context.Context, dir string) string {
	branch, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	sha, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(branch)) + "@" + strings.TrimSpace(string(sha))
}
