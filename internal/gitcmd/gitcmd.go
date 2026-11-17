// Package gitcmd runs git commands, optionally reusing their output for the rest of a run.
package gitcmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Output runs git in dir and returns its standard output. A failure includes git's standard error.
func Output(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// Cache reuses the result of each git command for the rest of a run, so it assumes the repositories don't change
// meanwhile. Create one per run. It's safe for concurrent use, and a nil Cache runs every command.
type Cache struct {
	mu      sync.Mutex
	results map[string]*result
}

type result struct {
	once sync.Once
	out  string
	err  error
}

// Output is like the package-level Output, but runs each command in each dir once.
func (c *Cache) Output(ctx context.Context, dir string, args ...string) (string, error) {
	if c == nil {
		return Output(ctx, dir, args...)
	}
	key := strings.Join(append([]string{dir}, args...), "\x00")
	c.mu.Lock()
	if c.results == nil {
		c.results = map[string]*result{}
	}
	r := c.results[key]
	if r == nil {
		r = &result{}
		c.results[key] = r
	}
	c.mu.Unlock()
	r.once.Do(func() { r.out, r.err = Output(ctx, dir, args...) })
	return r.out, r.err
}
