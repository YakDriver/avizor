# Evaluation

`radar eval` runs a hand-labeled corpus through reconciliation and reports how often Radar finds the right existing work. Use it before and after changing retrieval or categorization rules, so changes are measured instead of guessed.

```
radar eval --format text              # all cases, with failures explained
radar eval --format text --failures   # failing cases only
radar eval dup-41552 impl-45278       # selected cases
radar eval                            # JSON report
```

The corpus is [`tfaws.hcl`](tfaws.hcl), built into the `radar` binary, so `radar eval` works without a source checkout. Use `--corpus path` for another file. Eval runs offline: it reads only the local index (and the checkout for cases with `code = true`). It doesn't fetch issues or pages, so results are repeatable.

## Index requirements

The cases reference provider issues filed since 2025-01-01. Sync at least that far back:

```
radar sync --since 2025-01-01
```

A case whose input issue isn't indexed fails with an error rather than being skipped.

## Point in time

Each case reconciles against the index as it looked at the case's `as_of` time. Without that, the answer leaks into the result: a duplicate's original would show as closed, or the PR that implemented an announcement would appear before the announcement was even filed.

`as_of` is `"filed"` (the default for issue inputs: when the input was created), `"now"` (the default for text and URLs), a date (the end of that day, UTC), or an RFC 3339 time.

The reconstruction is approximate. Issues created later are hidden, including from the full-text ranking statistics and from cross-references in the input, signals from comments posted later are ignored, and issues closed or merged later are shown open. Titles, bodies, and labels are their current versions, since edits aren't tracked. Code searches use the checkout as it is now, so they're off unless a case sets `code = true`.

Issue inputs are recognized the same way `radar candidates` recognizes them: a number, or an issue or PR URL for the target repo, with or without a fragment such as `#issuecomment-1`. Another repository's issue URL is treated as a plain URL.

## Case format

```hcl
case "dup-41552" {
  input = "41552"              # issue number or URL; or text = "..."
  as_of = "filed"              # optional, see above
  code  = false                # search the checkout
  kind  = "duplicate"          # reporting group
  note  = "free text"

  expect {                     # a candidate that must be in the top 5
    number   = 40135           # or path = "internal/service/..."
    category = "already_tracked" # optional: the category it must have
  }

  related_only = [43195]       # may appear, but only as "related"
  unmatched    = false         # optional: expected summary.unmatched
}
```

A case passes when every expected candidate is in the top 5 with the expected category, no `related_only` candidate is above "related", and `unmatched` matches if set.

## Kinds and where labels come from

Labels come from repository history, not from Radar's output.

| Kind | Cases | Source of truth |
|---|---|---|
| `duplicate` | 20 | GitHub "Duplicate of #N" comments. Every 5th of the 98 such pairs since 2025-01-01 whose original predates the duplicate. The expected category is "already tracked", or "previously considered" if the original was closed first. |
| `announcement` | 6 | What's New URLs that maintainer tracking issues link, 30 minutes after the tracker was filed. |
| `announcement-text` | 6 | The same announcements' headlines as free text, standing in for a request worded differently. |
| `implemented` | 6 | Announcement URLs an hour after the PR that closed the tracker merged. The PR should be "already implemented" and the tracker should appear. |
| `related` | 4 | Trackers whose "Relates" link is a similar but different feature. |
| `new` | 5 | Headlines a minute before their tracker was filed. Nothing should be tracked yet. |
| `no-work` | 4 | aws-sdk-go-v2 release notes that need no provider work (protocol, serialization, and retry changes). |
| `dev` | 4 | Queries used while building milestone 1. Reported separately because the rules were tuned on them. |

## Metrics

| Column | Meaning |
|---|---|
| `hit@k` | Cases where an expected candidate is in the top k. |
| `recall` | Expected candidates returned at any rank. |
| `category` | Expected candidates with a labeled category that got that category. |
| `unmatch` | Cases labeling `unmatched` that got it right. |
| `false=` | Related-only candidates shown above "related", out of those labeled. |

The report also counts cases with existing work that were summarized as unmatched.

## Baseline

Milestone 2, index synced from 2025-01-01 (15,493 issues and PRs, 52,167 comments), checkout `main` at 2026-11-11:

```
kind               cases   pass  hit@1  hit@3  hit@5  recall category  unmatch false=
announcement           6      6   100%   100%   100%    100%     100%        -    0/0
announcement-text      6      0    83%   100%   100%    100%       0%        -    0/0
dev                    4      4    75%   100%   100%    100%     100%     100%    0/1
duplicate             20      3    45%    70%    80%     90%      15%        -    0/0
implemented            6      2   100%   100%   100%     67%      33%        -    0/0
new                    5      5      -      -      -       -        -     100%    0/0
no-work                4      4      -      -      -       -        -     100%    0/0
related                4      3      -      -      -       -        -        -    1/4
overall               55     27    69%    86%    90%     88%      37%     100%    1/5
17 case(s) with existing work were summarized as unmatched
```

What it shows:

- **Retrieval is good; categorization is the gap.** 90% of cases with an answer have it in the top 5, but only 37% of expected candidates get the right category. Most duplicates and reworded announcements are found, as "related", because they share keywords and labels but no exact anchor.
- **Exact anchors work.** Announcement URLs find their trackers every time.
- **Implementing PRs are missed when they don't repeat the anchor.** For 4 of 6 `implemented` cases the tracker is found but the PR that closed it isn't, because Radar doesn't follow links from a candidate to the PRs that reference it.
- **Some strong signals are too broad.** The one false-equivalent comes from a News Blog post that announced two features at once. Other failures show an operation name or an SDK docs URL shared by unrelated issues.
- **`new` and `no-work` pass trivially.** Keyword-only matches are always "related", so these confirm Radar doesn't invent strong matches, but they can't fail any other way.
