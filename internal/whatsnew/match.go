package whatsnew

import (
	"cmp"
	"fmt"
	"html"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Matching defaults. They were chosen on 36 SDK items that maintainer trackers link to an announcement, and checked on
// a random sample of untracked items; see docs/design/avizor.md.
const (
	// DefaultBefore and DefaultAfter bound how long before and after a release an announcement can be posted.
	DefaultBefore = 3 * 24 * time.Hour
	DefaultAfter  = 7 * 24 * time.Hour
	// DefaultMinScore is the lowest score that matches.
	DefaultMinScore = 0.4
	// DefaultMargin is how much the best score must exceed the runner-up's, as a ratio.
	DefaultMargin = 1.3
	// nearDays is how close to the release an announcement scores in full; further ones score 75%.
	nearDays = 1.5
	// minIDF is the least a word weighs.
	minIDF = 0.01
)

// regionRE matches headlines about availability in more Regions, which repeat an earlier launch's wording.
var regionRE = regexp.MustCompile(`(?i)\bavailable in\b.*(\bregions?\b|\bgovcloud\b|\bsovereign\b|\(.*\))|\bexpands?\b.*\bregions?\b|\badditional (aws )?regions?\b|\bin aws govcloud\b`)

// RegionExpansion reports whether a headline announces availability in more Regions.
func RegionExpansion(headline string) bool {
	return regionRE.MatchString(headline)
}

// Matcher finds the announcement for an item by text similarity: TF-IDF weighted cosine between the item's release
// notes and service name, and each announcement's headline (counted twice) and body.
type Matcher struct {
	Before, After time.Duration
	MinScore      float64
	Margin        float64

	anns []Announcement
	vecs []vector
	// region marks region expansion announcements: ranked, but never matched.
	region []bool
	idf    map[string]float64
	n      float64
}

type vector map[string]float64

// NewMatcher indexes announcements. Region expansion announcements are kept for Rank but never matched.
func NewMatcher(as []Announcement) *Matcher {
	m := &Matcher{Before: DefaultBefore, After: DefaultAfter, MinScore: DefaultMinScore, Margin: DefaultMargin,
		idf: map[string]float64{}, n: float64(len(as))}
	docs := make([][]string, len(as))
	df := map[string]int{}
	for i, a := range as {
		docs[i] = slices.Concat(Tokens(a.Headline), Tokens(a.Headline), Tokens(a.Body))
		seen := map[string]bool{}
		for _, w := range docs[i] {
			if !seen[w] {
				seen[w] = true
				df[w]++
			}
		}
	}
	for w, n := range df {
		// A word in every post weighs almost nothing, but not zero: in a corpus of one post, every word is in every
		// post, and zero weights would leave nothing to match on.
		m.idf[w] = max(math.Log((m.n+1)/float64(n+1)), minIDF)
	}
	for i, a := range as {
		m.anns = append(m.anns, a)
		m.vecs = append(m.vecs, m.vector(docs[i]))
		m.region = append(m.region, RegionExpansion(a.Headline))
	}
	return m
}

// Len returns the number of announcements that can match: all but region expansions.
func (m *Matcher) Len() int {
	n := 0
	for _, r := range m.region {
		if !r {
			n++
		}
	}
	return n
}

func (m *Matcher) weight(w string) float64 {
	if v, ok := m.idf[w]; ok {
		return v
	}
	return math.Log(m.n + 1)
}

func (m *Matcher) vector(ts []string) vector {
	v := vector{}
	for _, w := range ts {
		v[w] += m.weight(w)
	}
	// Sum in word order: float addition isn't associative, and map order is random, so scores would differ in the last
	// bits from run to run and ties would break differently.
	var norm float64
	for _, w := range slices.Sorted(maps.Keys(v)) {
		norm += v[w] * v[w]
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return v
	}
	for w := range v {
		v[w] /= norm
	}
	return v
}

// Match is an announcement matched to an item.
type Match struct {
	Announcement Announcement `json:"announcement"`
	// Score is the similarity, scaled down for announcements posted far from the release. RunnerUp is the next best.
	Score    float64 `json:"score"`
	RunnerUp float64 `json:"runner_up"`
	// Days is when the announcement was posted, relative to the release.
	Days  float64 `json:"days"`
	Basis string  `json:"basis"`
}

// Match returns the best announcement for an item's text, released at published, or nil if none is clearly best.
func (m *Matcher) Match(text, service string, published time.Time) *Match {
	best := m.score(text, service, published, m.Before, m.After, false)
	if len(best) == 0 {
		return nil
	}
	top := best[0]
	var second float64
	if len(best) > 1 {
		second = best[1].score
	}
	if top.score < m.MinScore || top.score < m.Margin*second {
		return nil
	}
	return &Match{
		Announcement: m.anns[top.i], Score: round(top.score), RunnerUp: round(second), Days: round(top.days),
		Basis: fmt.Sprintf("text similarity %.2f (next best %.2f), posted %+.1f days from the release", top.score, second, top.days),
	}
}

// Ranked is an announcement scored against an item, as Match scores it.
type Ranked struct {
	Announcement Announcement `json:"announcement"`
	Score        float64      `json:"score"`
	// Days is when the announcement was posted, relative to the release.
	Days float64 `json:"days"`
	// RegionExpansion is set for a post about availability in more Regions, which Match never picks.
	RegionExpansion bool `json:"region_expansion,omitempty"`
}

// Rank scores every announcement posted from before the release to after it, region expansions included, and returns
// the n best, highest score first; n <= 0 returns all. It doesn't pick one: Match does, with a floor and a margin over
// the runner-up.
func (m *Matcher) Rank(text, service string, published time.Time, before, after time.Duration, n int) []Ranked {
	best := m.score(text, service, published, before, after, true)
	if n > 0 && len(best) > n {
		best = best[:n]
	}
	out := make([]Ranked, len(best))
	for i, b := range best {
		out[i] = Ranked{Announcement: m.anns[b.i], Score: round(b.score), Days: round(b.days), RegionExpansion: m.region[b.i]}
	}
	return out
}

type scored struct {
	i     int
	score float64
	days  float64
}

// score scores the announcements posted within before and after published, highest first, leaving out region
// expansions unless regions is set. Ties go to the earlier post, then the URL, so the order doesn't depend on the order
// announcements were loaded.
func (m *Matcher) score(text, service string, published time.Time, before, after time.Duration, regions bool) []scored {
	q := m.vector(slices.Concat(Tokens(text), Tokens(service), Tokens(service)))
	words := slices.Sorted(maps.Keys(q))
	var best []scored
	for i, a := range m.anns {
		d := a.PostedAt.Sub(published)
		if d < -before || d > after || m.region[i] && !regions {
			continue
		}
		var s float64
		for _, w := range words {
			s += q[w] * m.vecs[i][w]
		}
		days := d.Hours() / 24
		if math.Abs(days) > nearDays {
			s *= 0.75
		}
		best = append(best, scored{i, s, days})
	}
	slices.SortFunc(best, func(a, b scored) int {
		return cmp.Or(cmp.Compare(b.score, a.score), m.anns[a.i].PostedAt.Compare(m.anns[b.i].PostedAt), strings.Compare(m.anns[a.i].URL, m.anns[b.i].URL))
	})
	return best
}

func round(f float64) float64 { return math.Round(f*100) / 100 }

// stopwords are common in both release notes and announcements.
var stopwords = map[string]bool{}

func init() {
	for w := range strings.FieldsSeq("a an the and or of to for in on with by from at as is are be this that now new support supports supported adds add added amazon aws you your can customers release today available using use via which it its into more also") {
		stopwords[w] = true
	}
}

var tagRE = regexp.MustCompile(`<[^>]+>`)

// Tokens splits text into lowercase words for matching, splitting camel case and dropping HTML tags and stopwords.
func Tokens(s string) []string {
	s = html.UnescapeString(tagRE.ReplaceAllString(s, " "))
	var b strings.Builder
	var prev rune
	for _, r := range s {
		if unicode.IsUpper(r) && unicode.IsLower(prev) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	var out []string
	for w := range strings.FieldsFuncSeq(strings.ToLower(b.String()), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if len(w) > 1 && !stopwords[w] {
			out = append(out, w)
		}
	}
	return out
}
