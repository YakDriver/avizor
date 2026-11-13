// Package syncer refreshes a target's index from GitHub.
package syncer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/YakDriver/avizor/internal/config"
	"github.com/YakDriver/avizor/internal/github"
	"github.com/YakDriver/avizor/internal/index"
)

// Metadata keys.
const (
	MetaRepo          = "repo"
	MetaIssuesSince   = "issues_since"
	MetaCommentsSince = "comments_since"
	MetaLastSync      = "last_sync"
	MetaLastFullSync  = "last_full_sync"
	metaGen           = "sync_gen"
)

// Options control one sync run.
type Options struct {
	// Full ignores checkpoints, refetches everything, and removes records GitHub no longer returns.
	Full bool
	// Since, if set, fetches only records updated at or after it, without reading or moving checkpoints. Use it to
	// index a small, recent slice.
	Since time.Time
	// Log receives progress messages.
	Log func(format string, args ...any)
}

// Result summarizes a sync run.
type Result struct {
	Repo      string    `json:"repo"`
	Issues    int       `json:"issues_synced"`
	Comments  int       `json:"comments_synced"`
	Removed   int64     `json:"removed"`
	Full      bool      `json:"full"`
	Since     time.Time `json:"since,omitzero"`
	StartedAt time.Time `json:"started_at"`
	Duration  string    `json:"duration"`
}

// Run syncs issues, PRs, and (depending on configuration) comment signals for a target.
func Run(ctx context.Context, gh *github.Client, st *index.Store, t *config.Target, opts Options) (*Result, error) {
	logf := opts.Log
	if logf == nil {
		logf = func(string, ...any) {}
	}
	res := &Result{Repo: t.Repo, Full: opts.Full, Since: opts.Since, StartedAt: time.Now().UTC()}

	if prev, err := st.Meta(ctx, MetaRepo); err != nil {
		return nil, err
	} else if prev != "" && prev != t.Repo {
		return nil, fmt.Errorf("index belongs to %s, but target %q is %s", prev, t.Name, t.Repo)
	}
	if err := st.SetMeta(ctx, MetaRepo, t.Repo); err != nil {
		return nil, err
	}

	gen, err := generation(ctx, st, opts.Full)
	if err != nil {
		return nil, err
	}

	checkpoints := opts.Since.IsZero()
	issuesSince, err := startTime(ctx, st, MetaIssuesSince, opts)
	if err != nil {
		return nil, err
	}

	err = gh.ListIssues(ctx, t.Repo, issuesSince, func(page []github.Issue) error {
		tx, err := st.Begin(ctx, t.Repo)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		var latest time.Time
		for i := range page {
			if err := tx.UpsertIssue(ctx, Convert(&page[i]), gen); err != nil {
				return err
			}
			latest = maxTime(latest, page[i].UpdatedAt)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing issues: %w", err)
		}
		res.Issues += len(page)
		if checkpoints && !latest.IsZero() {
			if err := st.SetMeta(ctx, MetaIssuesSince, latest.Format(time.RFC3339)); err != nil {
				return err
			}
		}
		logf("issues: %d synced (updated through %s)", res.Issues, latest.Format(time.DateOnly))
		return nil
	})
	if err != nil {
		return res, fmt.Errorf("syncing issues: %w", err)
	}

	if opts.Full {
		tx, err := st.Begin(ctx, t.Repo)
		if err != nil {
			return res, err
		}
		defer tx.Rollback()
		if res.Removed, err = tx.DeleteNotSeen(ctx, gen); err != nil {
			return res, err
		}
		if err := tx.Commit(); err != nil {
			return res, fmt.Errorf("committing removals: %w", err)
		}
	}

	if t.Index.Comments == config.CommentsURLsAndIdentifiers {
		commentsSince, err := startTime(ctx, st, MetaCommentsSince, opts)
		if err != nil {
			return res, err
		}
		err = gh.ListComments(ctx, t.Repo, commentsSince, func(page []github.Comment) error {
			tx, err := st.Begin(ctx, t.Repo)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			var latest time.Time
			for i := range page {
				c := &page[i]
				n, err := c.IssueNumber()
				if err != nil {
					return err
				}
				if err := tx.ReplaceCommentSignals(ctx, n, c.ID, c.CreatedAt, c.Body); err != nil {
					return err
				}
				latest = maxTime(latest, c.UpdatedAt)
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("committing comments: %w", err)
			}
			res.Comments += len(page)
			if checkpoints && !latest.IsZero() {
				if err := st.SetMeta(ctx, MetaCommentsSince, latest.Format(time.RFC3339)); err != nil {
					return err
				}
			}
			logf("comments: %d scanned (updated through %s)", res.Comments, latest.Format(time.DateOnly))
			return nil
		})
		if err != nil {
			return res, fmt.Errorf("syncing comments: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := st.SetMeta(ctx, MetaLastSync, now); err != nil {
		return res, err
	}
	if opts.Full {
		if err := st.SetMeta(ctx, MetaLastFullSync, now); err != nil {
			return res, err
		}
	}
	res.Duration = time.Since(res.StartedAt).Round(time.Second).String()
	return res, nil
}

// generation returns the sync generation stamped on records. A full sync starts a new generation immediately, so
// records only seen by an interrupted full sync are still removed by the next completed one.
func generation(ctx context.Context, st *index.Store, full bool) (int64, error) {
	v, err := st.Meta(ctx, metaGen)
	if err != nil {
		return 0, err
	}
	var gen int64
	if v != "" {
		if gen, err = strconv.ParseInt(v, 10, 64); err != nil {
			return 0, fmt.Errorf("invalid %s %q: %w", metaGen, v, err)
		}
	}
	if full || v == "" {
		gen++
		if err := st.SetMeta(ctx, metaGen, strconv.FormatInt(gen, 10)); err != nil {
			return 0, err
		}
	}
	return gen, nil
}

func startTime(ctx context.Context, st *index.Store, key string, opts Options) (time.Time, error) {
	switch {
	case !opts.Since.IsZero():
		return opts.Since, nil
	case opts.Full:
		return time.Time{}, nil
	}
	v, err := st.Meta(ctx, key)
	if err != nil || v == "" {
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid checkpoint %s %q: %w", key, v, err)
	}
	return t, nil
}

// Convert maps a GitHub issue or PR to an index record.
func Convert(g *github.Issue) *index.Issue {
	is := &index.Issue{
		Number:      g.Number,
		Kind:        index.KindIssue,
		State:       g.State,
		StateReason: g.StateReason,
		Title:       g.Title,
		Body:        g.Body,
		Author:      g.User.Login,
		URL:         g.HTMLURL,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
	if g.ClosedAt != nil {
		is.ClosedAt = *g.ClosedAt
	}
	if g.PullRequest != nil {
		is.Kind = index.KindPR
		if g.PullRequest.MergedAt != nil {
			is.Merged = true
			is.MergedAt = *g.PullRequest.MergedAt
		}
	}
	for _, l := range g.Labels {
		is.Labels = append(is.Labels, l.Name)
	}
	return is
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
