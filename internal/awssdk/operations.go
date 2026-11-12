package awssdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/YakDriver/radar/internal/smithy"
)

// Operations is the set of operation names across all service models at one commit.
type Operations struct {
	Commit string   `json:"commit"`
	Names  []string `json:"names"`

	once sync.Once
	set  map[string]bool
}

// Has reports whether name is an operation in some model. It's safe for concurrent use.
func (o *Operations) Has(name string) bool {
	o.once.Do(func() {
		o.set = make(map[string]bool, len(o.Names))
		for _, n := range o.Names {
			o.set[n] = true
		}
	})
	return o.set[name]
}

// Operations returns every operation name in the service models at Branch's tip. The result is cached in cachePath,
// keyed by the tip commit, so it's only recomputed after a fetch brings new models. An empty cachePath disables caching.
func (r *ModelsRepo) Operations(ctx context.Context, cachePath string) (*Operations, error) {
	tip, err := r.Tip(ctx)
	if err != nil {
		return nil, err
	}
	if tip.SHA == "" {
		return nil, fmt.Errorf("models repository %s has no commits on %s", r.Dir, r.Branch)
	}
	if cachePath != "" {
		if b, err := os.ReadFile(cachePath); err == nil {
			var ops Operations
			if json.Unmarshal(b, &ops) == nil && ops.Commit == tip.SHA {
				return &ops, nil
			}
		}
	}

	ops, err := r.operationsAt(ctx, tip.SHA)
	if err != nil {
		return nil, err
	}
	if cachePath != "" {
		b, err := json.Marshal(ops)
		if err != nil {
			return nil, err
		}
		if err := writeFileAtomic(cachePath, b); err != nil {
			return nil, err
		}
	}
	return ops, nil
}

// writeFileAtomic writes data to path through a unique temporary file in the same directory, so concurrent writers
// don't rename each other's file away and readers never see a partial file.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}

// operationsAt parses every service model at commit, streaming blobs through one git cat-file process.
func (r *ModelsRepo) operationsAt(ctx context.Context, commit string) (*Operations, error) {
	out, err := r.git(ctx, "ls-tree", "-r", commit, "--", "models/")
	if err != nil {
		return nil, err
	}
	var blobs []string
	for line := range strings.Lines(out) {
		// "<mode> blob <id>\t<path>"
		meta, path, ok := strings.Cut(strings.TrimSpace(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[1] != "blob" || !strings.HasSuffix(path, ".json") || !strings.Contains(path, "/service/") {
			continue
		}
		blobs = append(blobs, fields[2])
	}

	cmd := exec.CommandContext(ctx, "git", "-C", r.Dir, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(blobs, "\n") + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	names := map[string]bool{}
	rd := bufio.NewReaderSize(stdout, 1<<20)
	readErr := func() error {
		for range blobs {
			// "<id> blob <size>\n<content>\n"
			header, err := rd.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading git cat-file: %w", err)
			}
			fields := strings.Fields(header)
			if len(fields) != 3 {
				return fmt.Errorf("unexpected git cat-file header %q", header)
			}
			size, err := strconv.Atoi(fields[2])
			if err != nil {
				return fmt.Errorf("unexpected git cat-file header %q", header)
			}
			data := make([]byte, size+1)
			if _, err := io.ReadFull(rd, data); err != nil {
				return fmt.Errorf("reading blob %s: %w", fields[0], err)
			}
			m, err := smithy.Parse(data[:size])
			if err != nil {
				return fmt.Errorf("parsing blob %s: %w", fields[0], err)
			}
			for _, op := range m.Operations() {
				names[op] = true
			}
		}
		return nil
	}()
	if readErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, readErr
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	ops := &Operations{Commit: commit, Names: make([]string, 0, len(names))}
	for n := range names {
		ops.Names = append(ops.Names, n)
	}
	slices.Sort(ops.Names)
	return ops, nil
}
