package gitcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	commit := func(msg string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte(msg), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "f"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", msg}} {
			if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v: %s", args, err, out)
			}
		}
	}
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	commit("one")

	var c Cache
	var wg sync.WaitGroup
	heads := make([]string, 4)
	for i := range heads {
		wg.Go(func() {
			var err error
			if heads[i], err = c.Output(ctx, dir, "rev-parse", "HEAD"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	first := heads[0]
	for _, h := range heads {
		if h != first || h == "" {
			t.Fatalf("concurrent heads = %q, want one value", heads)
		}
	}

	// The cache keeps the first result; a nil cache runs the command again.
	commit("two")
	if got, _ := c.Output(ctx, dir, "rev-parse", "HEAD"); got != first {
		t.Errorf("cached HEAD = %q, want %q", got, first)
	}
	var none *Cache
	if got, _ := none.Output(ctx, dir, "rev-parse", "HEAD"); got == first {
		t.Errorf("nil cache HEAD = %q, want the new commit", got)
	}

	// Failures include git's error output.
	if _, err := c.Output(ctx, dir, "show", "nope:f"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("err = %v, want git's message", err)
	}
}
