package enrich

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/YakDriver/avizor/internal/awssdk"
	"github.com/YakDriver/avizor/internal/items"
	"github.com/YakDriver/avizor/internal/whatsnew"
)

// PostItem is an SDK item ranked for a blog post, for drafting the post's issue from it. Days is how many days the post
// came after the release. Announcement is set when the item and the post match the same What's New announcement, and
// TaggedService when the post's product tags name the item's service.
type PostItem struct {
	ID            string  `json:"id"`
	Released      string  `json:"released"`
	Days          float64 `json:"days"`
	Score         float64 `json:"score"`
	Announcement  bool    `json:"announcement,omitempty"`
	TaggedService bool    `json:"tagged_service,omitempty"`
}

// A post can come months after its release: MSK channels shipped in the SDK 109 days before their post. So items are
// ranked from PostItemsBefore the post, which covers every pulled item in practice, to PostItemsAfter it, since a
// release can follow its announcement by a day or two.
const (
	PostItemsBefore = 365 * 24 * time.Hour
	PostItemsAfter  = 7 * 24 * time.Hour
)

// PostItemsMin is the lowest text score an item needs to be listed for a post, unless it shares an announcement with
// the post.
const PostItemsMin = 0.2

// Most posts come within brief's blog window of their release, so an item released longer than BlogAfter before the
// post counts oldItemFactor of its score. Old releases of the same service otherwise outrank the right one: on the
// 32 labeled posts with an item, it raises hit@1 from 21 to 23.
const oldItemFactor = 0.5

// Announcements that match an item: the same floors posts use, so an item and a post meet at a shared announcement even
// when neither one's best match is it.
const (
	itemAnnMin   = 0.35
	itemAnnRatio = 0.75
	itemAnnMax   = 3
)

// ItemIndex ranks SDK items for blog posts, the reverse of BlogPosts. Each item's document is its release notes, plus
// the names of the operations, shapes, and members it adds.
type ItemIndex struct {
	m     *whatsnew.Matcher
	items map[string]*awssdk.Item
	// anns maps an item to the URLs of the announcements that match it.
	anns map[string][]string
}

// NewItemIndex indexes items and matches each to announcements, if there are any. Skipped items have no API change to
// draft from, so they're left out.
func NewItemIndex(its []*awssdk.Item, anns *whatsnew.Matcher) *ItemIndex {
	x := &ItemIndex{items: map[string]*awssdk.Item{}, anns: map[string][]string{}}
	var docs []whatsnew.Announcement
	for _, it := range its {
		if it.Status == items.StatusSkipped {
			continue
		}
		x.items[it.ID] = it
		if anns != nil {
			x.anns[it.ID] = itemAnnouncements(anns, it)
		}
		docs = append(docs, whatsnew.Announcement{ID: it.ID, URL: it.ID, Headline: itemText(it), Body: diffNames(it),
			PostedAt: it.Release.PublishedAt})
	}
	x.m = whatsnew.NewMatcher(docs)
	return x
}

// Len is the number of items indexed.
func (x *ItemIndex) Len() int { return len(x.items) }

// Rank returns the items that best match a post, best first, n at most (0 for all). The query is the post's title and
// excerpt plus extra, such as the headlines of its matched announcements, whose URLs are annURLs. Items that match one
// of those announcements come first. The rest need a text score of at least PostItemsMin. Within each group, an item
// whose service the post's product tags name scores higher, as in BlogPosts, and an item released more than BlogAfter
// before the post scores lower.
func (x *ItemIndex) Rank(p whatsnew.Post, annURLs []string, extra string, n int) []PostItem {
	out := []PostItem{}
	if x == nil || len(x.items) == 0 {
		return out
	}
	text := strings.Join([]string{p.Title, p.Excerpt, extra}, "\n")
	for _, r := range x.m.Rank(text, "", p.PostedAt, PostItemsBefore, PostItemsAfter, 0) {
		it := x.items[r.Announcement.ID]
		linked := slices.ContainsFunc(x.anns[it.ID], func(u string) bool { return slices.Contains(annURLs, u) })
		if r.Score < PostItemsMin && !linked {
			continue
		}
		_, service := AnnouncementQuery(it)
		out = append(out, PostItem{ID: it.ID, Released: it.Release.PublishedAt.Format(time.DateOnly), Days: -r.Days,
			Score: r.Score, Announcement: linked, TaggedService: taggedService(p.Tags, strings.Join(ServiceWords(service), ""))})
	}
	// Stable, so equal keys keep Rank's order.
	slices.SortStableFunc(out, func(a, b PostItem) int {
		if a.Announcement != b.Announcement {
			if a.Announcement {
				return -1
			}
			return 1
		}
		return cmp.Compare(boostedItem(b), boostedItem(a))
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func boostedItem(p PostItem) float64 {
	s := p.Score
	if p.TaggedService {
		s *= 1 + tagBoost
	}
	if p.Days > BlogAfter.Hours()/24 {
		s *= oldItemFactor
	}
	return s
}

// itemAnnouncements returns the URLs of the announcements that match an item: posted from whatsnew.DefaultBefore its
// release to whatsnew.DefaultAfter, scoring at least itemAnnMin and itemAnnRatio of the best, itemAnnMax at most, with
// region expansions left out.
func itemAnnouncements(anns *whatsnew.Matcher, it *awssdk.Item) []string {
	text, service := AnnouncementQuery(it)
	var out []string
	var best float64
	for _, r := range anns.Rank(text, service, it.Release.PublishedAt, whatsnew.DefaultBefore, whatsnew.DefaultAfter, 0) {
		if r.RegionExpansion {
			continue
		}
		if r.Score < itemAnnMin || (best > 0 && r.Score < itemAnnRatio*best) {
			break
		}
		best = max(best, r.Score)
		out = append(out, r.Announcement.URL)
		if len(out) == itemAnnMax {
			break
		}
	}
	return out
}

// diffNames returns the names of the operations, shapes, and members an item adds, space-separated. Tokens splits them
// into words: CreateChannel is "create channel".
func diffNames(it *awssdk.Item) string {
	d := it.Diff
	if d == nil {
		return ""
	}
	names := slices.Clone(d.AddedOperations)
	for _, s := range d.AddedShapes {
		names = append(names, s.Name)
	}
	for _, m := range d.AddedMembers {
		names = append(names, m.Member)
	}
	return strings.Join(names, " ")
}
