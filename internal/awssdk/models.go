// Package awssdk turns AWS SDK for Go v2 releases into items: one per service per release, with the API model change
// computed from a local clone of the AWS API models repository.
package awssdk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ModelsRepo is a local bare clone of a Smithy models repository such as aws/api-models-aws.
type ModelsRepo struct {
	// Dir is the bare clone's directory.
	Dir string
	// URL is cloned from when Dir doesn't exist, and fetched from.
	URL string
	// Branch is the branch whose history is read.
	Branch string
}

// Ensure clones the repository if Dir doesn't exist, and otherwise fetches Branch if fetch is set.
func (r *ModelsRepo) Ensure(ctx context.Context, fetch bool, logf func(string, ...any)) error {
	if _, err := os.Stat(r.Dir); errors.Is(err, os.ErrNotExist) {
		if r.URL == "" {
			return fmt.Errorf("models repository %s doesn't exist and no URL is configured", r.Dir)
		}
		if err := os.MkdirAll(filepath.Dir(r.Dir), 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(r.Dir), err)
		}
		logf("cloning %s into %s", r.URL, r.Dir)
		if _, err := run(ctx, "", "clone", "--quiet", "--bare", "--single-branch", "--branch", r.Branch, "--", r.URL, r.Dir); err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	if !fetch {
		return nil
	}
	logf("fetching %s", r.URL)
	_, err := r.git(ctx, "fetch", "--quiet", "--", r.URL, "+refs/heads/"+r.Branch+":refs/heads/"+r.Branch)
	return err
}

// Commit is a commit and its committer time.
type Commit struct {
	SHA  string    `json:"sha"`
	Time time.Time `json:"time"`
}

// CommitAt returns the last commit on Branch at or before t, or a zero Commit if there is none.
func (r *ModelsRepo) CommitAt(ctx context.Context, t time.Time) (Commit, error) {
	out, err := r.git(ctx, "log", "-1", "--format=%H %cI", "--before="+t.UTC().Format(time.RFC3339), "refs/heads/"+r.Branch, "--")
	if err != nil {
		return Commit{}, err
	}
	line := strings.TrimSpace(out)
	if line == "" {
		return Commit{}, nil
	}
	sha, ts, ok := strings.Cut(line, " ")
	if !ok {
		return Commit{}, fmt.Errorf("unexpected git log output %q", line)
	}
	when, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return Commit{}, fmt.Errorf("parsing commit time %q: %w", ts, err)
	}
	return Commit{SHA: sha, Time: when.UTC()}, nil
}

// ModelChange is a model file added, modified, or deleted between two commits.
type ModelChange struct {
	// Status is "A", "M", or "D".
	Status string
	Path   string
}

// ChangedModels returns model JSON files under models/ that differ between base and head.
func (r *ModelsRepo) ChangedModels(ctx context.Context, base, head string) ([]ModelChange, error) {
	if base == head {
		return nil, nil
	}
	out, err := r.git(ctx, "diff", "--name-status", "--no-renames", base, head, "--", "models/")
	if err != nil {
		return nil, err
	}
	var changes []ModelChange
	for line := range strings.Lines(out) {
		status, path, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || !strings.HasSuffix(path, ".json") || !strings.Contains(path, "/service/") {
			continue
		}
		changes = append(changes, ModelChange{Status: status[:1], Path: path})
	}
	return changes, nil
}

// BlobID returns the object ID of path at commit, for caching parsed models.
func (r *ModelsRepo) BlobID(ctx context.Context, commit, path string) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--verify", "--quiet", commit+":"+path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Blob returns an object's content.
func (r *ModelsRepo) Blob(ctx context.Context, id string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Dir, "cat-file", "blob", id)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git cat-file blob %s: %w: %s", id, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (r *ModelsRepo) git(ctx context.Context, args ...string) (string, error) {
	return run(ctx, r.Dir, args...)
}

func run(ctx context.Context, dir string, args ...string) (string, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
