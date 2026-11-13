package items

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/YakDriver/avizor/internal/whatsnew"
)

const announcementsSchema = `
CREATE TABLE IF NOT EXISTS announcements (
	id        TEXT PRIMARY KEY,
	source    TEXT NOT NULL,
	url       TEXT NOT NULL,
	headline  TEXT NOT NULL,
	posted_at TEXT NOT NULL,
	body      TEXT NOT NULL,
	tags      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS announcements_posted ON announcements(posted_at);
`

// AnnouncementStore stores announcements for one source.
type AnnouncementStore struct {
	*Store
	Source string
}

// PutAnnouncements inserts or replaces announcements in one transaction.
func (s AnnouncementStore) PutAnnouncements(ctx context.Context, as []whatsnew.Announcement) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range as {
		tags, err := json.Marshal(a.Tags)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO announcements(id, source, url, headline, posted_at, body, tags) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET source = excluded.source, url = excluded.url, headline = excluded.headline,
	posted_at = excluded.posted_at, body = excluded.body, tags = excluded.tags`,
			a.ID, s.Source, a.URL, a.Headline, a.PostedAt.UTC().Format(time.RFC3339), a.Body, string(tags))
		if err != nil {
			return fmt.Errorf("storing announcement %s: %w", a.ID, err)
		}
	}
	return tx.Commit()
}

// Announcements returns every stored announcement, newest first.
func (s *Store) Announcements(ctx context.Context) ([]whatsnew.Announcement, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, url, headline, posted_at, body, tags FROM announcements ORDER BY posted_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("reading announcements: %w", err)
	}
	defer rows.Close()
	var out []whatsnew.Announcement
	for rows.Next() {
		var a whatsnew.Announcement
		var posted, tags string
		if err := rows.Scan(&a.ID, &a.URL, &a.Headline, &posted, &a.Body, &tags); err != nil {
			return nil, fmt.Errorf("reading announcements: %w", err)
		}
		if a.PostedAt, err = time.Parse(time.RFC3339, posted); err != nil {
			return nil, fmt.Errorf("announcement %s: %w", a.ID, err)
		}
		if err := json.Unmarshal([]byte(tags), &a.Tags); err != nil {
			return nil, fmt.Errorf("announcement %s tags: %w", a.ID, err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
