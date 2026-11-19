package items

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/YakDriver/avizor/internal/whatsnew"
)

const postsSchema = `
CREATE TABLE IF NOT EXISTS blog_posts (
	id        TEXT PRIMARY KEY,
	source    TEXT NOT NULL,
	url       TEXT NOT NULL,
	title     TEXT NOT NULL,
	posted_at TEXT NOT NULL,
	excerpt   TEXT NOT NULL,
	tags      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS blog_posts_posted ON blog_posts(posted_at);
`

// PostStore stores blog posts for one source.
type PostStore struct {
	*Store
	Source string
}

// PutPosts inserts or replaces blog posts in one transaction.
func (s PostStore) PutPosts(ctx context.Context, ps []whatsnew.Post) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range ps {
		tags, err := json.Marshal(p.Tags)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO blog_posts(id, source, url, title, posted_at, excerpt, tags) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET source = excluded.source, url = excluded.url, title = excluded.title,
	posted_at = excluded.posted_at, excerpt = excluded.excerpt, tags = excluded.tags`,
			p.ID, s.Source, p.URL, p.Title, p.PostedAt.UTC().Format(time.RFC3339), p.Excerpt, string(tags))
		if err != nil {
			return fmt.Errorf("storing blog post %s: %w", p.ID, err)
		}
	}
	return tx.Commit()
}

// Posts returns every stored blog post, newest first.
func (s *Store) Posts(ctx context.Context) ([]whatsnew.Post, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, url, title, posted_at, excerpt, tags FROM blog_posts ORDER BY posted_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("reading blog posts: %w", err)
	}
	defer rows.Close()
	var out []whatsnew.Post
	for rows.Next() {
		var p whatsnew.Post
		var posted, tags string
		if err := rows.Scan(&p.ID, &p.URL, &p.Title, &posted, &p.Excerpt, &tags); err != nil {
			return nil, fmt.Errorf("reading blog posts: %w", err)
		}
		if p.PostedAt, err = time.Parse(time.RFC3339, posted); err != nil {
			return nil, fmt.Errorf("blog post %s: %w", p.ID, err)
		}
		if err := json.Unmarshal([]byte(tags), &p.Tags); err != nil {
			return nil, fmt.Errorf("blog post %s tags: %w", p.ID, err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Post returns the stored blog post with a URL, or nil if there's none. A URL that differs only by a trailing slash
// matches.
func (s *Store) Post(ctx context.Context, url string) (*whatsnew.Post, error) {
	u := strings.TrimSuffix(url, "/")
	row := s.db.QueryRowContext(ctx, `SELECT id, url, title, posted_at, excerpt, tags FROM blog_posts WHERE url IN (?, ?)
ORDER BY posted_at DESC, id LIMIT 1`, u, u+"/")
	var p whatsnew.Post
	var posted, tags string
	switch err := row.Scan(&p.ID, &p.URL, &p.Title, &posted, &p.Excerpt, &tags); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("reading blog post %s: %w", url, err)
	}
	var err error
	if p.PostedAt, err = time.Parse(time.RFC3339, posted); err != nil {
		return nil, fmt.Errorf("blog post %s: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(tags), &p.Tags); err != nil {
		return nil, fmt.Errorf("blog post %s tags: %w", p.ID, err)
	}
	return &p, nil
}
