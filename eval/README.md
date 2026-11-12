# Evaluation

`radar eval` runs a hand-labeled corpus through reconciliation and reports how often Radar finds the right existing work. Use it before and after changing retrieval or categorization rules, so changes are measured instead of guessed.

```
radar eval --format text              # all cases, with failures explained
radar eval --format text --failures   # failing cases only
radar eval dup-41552 impl-45278       # selected cases
radar eval --format text --unjudged   # confident candidates that still need a same or different label
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

  same      = [41782]          # also the same work: a confident category is correct
  different = [43195]          # not the same work: may appear only as "related"
  unmatched = false            # optional: expected summary.unmatched
}
```

`same_paths` and `different_paths` judge code and docs candidates in cases with `code = true`.

A case passes when every expected candidate is in the top 5 with the expected category, no `different` candidate has a confident category, and `unmatched` matches if set.

## Judging confident candidates

Radar is used in a human/AI workflow, where misleading information is worse than none. A duplicate shown as "related" is weak but honest. A candidate shown as "already tracked" or "already implemented" that is the wrong work is confidently wrong.

So every candidate Radar returns with a confident category (anything above "related"), at any rank, needs a verdict: it's an expected candidate, labeled `same`, or labeled `different`. Judgments come from reading the candidate's title and body, not from its category. `radar eval --format text --unjudged` lists the ones still missing. After a change to reconciliation, label any new ones before reading precision.

## Kinds and where labels come from

Labels come from repository history, not from Radar's output.

| Kind | Cases | Source of truth |
|---|---|---|
| `duplicate` | 20 | GitHub "Duplicate of #N" comments. Every 5th of the 98 such pairs since 2025-01-01 whose original predates the duplicate. The expected category is "already tracked", or "previously considered" if the original was closed first. |
| `announcement` | 6 | What's New URLs that maintainer tracking issues link, 30 minutes after the tracker was filed. |
| `announcement-text` | 6 | The same announcements' headlines as free text, standing in for a request worded differently. |
| `implemented` | 6 | Announcement URLs an hour after the PR that closed the tracker merged. The PR should be "already implemented" and the tracker should appear. |
| `related` | 4 | Trackers whose "Relates" link is a similar but different feature. |

`same` and `different` labels were added by reading each confident candidate. Cases with `code = true` were judged against provider `main` at `37436a92a45` (2026-11-12).
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
| `precision` | Confident candidates that are the right work (expected or `same`), out of those judged. |
| `wrong` | Confident candidates labeled `different`. Each fails its case. |
| `unjudged` | Confident candidates with no verdict. Should be 0. |

The report also counts cases with existing work that were summarized as unmatched.

## Results

Index synced from 2025-01-01 (15,493 issues and PRs, 52,167 comments).

| Change | pass | hit@1 | hit@3 | hit@5 | recall | category | precision |
|---|---|---|---|---|---|---|---|
| Milestone 2 baseline | 27 | 69% | 86% | 90% | 88% | 37% | - |
| View-scoped full-text ranking (replaces FTS5 `bm25()`) | 27 | 71% | 83% | 90% | 88% | 37% | - |
| Follow references from matched issues to PRs | 30 | 69% | 83% | 88% | 96% | 47% | - |
| Judge confident candidates ([#3](https://github.com/YakDriver/radar/issues/3)) | 27 | 69% | 83% | 88% | 96% | 47% | 56% |

The last row changes no reconciliation code. Three cases now fail because they return different work with a confident category.

Current, by kind:

```
kind               cases  pass  hit@1  hit@3  hit@5 recall category unmatch precision wrong unjudged
announcement           6     6   100%   100%   100%   100%     100%       -      100%     0        0
announcement-text      6     0    83%   100%   100%   100%       0%       -         -     0        0
dev                    4     3    75%   100%   100%   100%     100%    100%       80%     2        0
duplicate             20     0    45%    65%    75%    90%      15%       -       24%    22        0
implemented            6     6   100%   100%   100%   100%     100%       -      100%     0        0
new                    5     5      -      -      -      -        -    100%         -     0        0
no-work                4     4      -      -      -      -        -    100%         -     0        0
related                4     3      -      -      -      -        -       -        0%     2        0
overall               55    27    69%    83%    88%    96%      47%    100%       56%    26        0
17 case(s) with existing work were summarized as unmatched
```

What it shows:

- **Confident labels are wrong 44% of the time.** 26 of 59 confident candidates are different work, and 22 of those come from duplicate cases. In those cases, a correct answer usually shows as "related" while unrelated issues show as "already tracked".
- **Anchors that are exact for announcements are broad elsewhere.** Every confident candidate in `announcement` and `implemented` cases is right. Bug reports and enhancement requests share documentation links and field names with unrelated issues.
- **Every wrong label traces to one strong signal:**

  | Root signal | Wrong |
  |---|---|
  | Terraform Registry docs URL | 10 |
  | `StartTime`, a field name misread as an API operation | 5 |
  | Followed reference from a wrongly matched issue | 5 |
  | AWS docs URL | 2 |
  | Shared API operation name (`UpdateImageVersion`, `CreateAndAttachS3AccessPoint` in a comment) | 2 |
  | pkg.go.dev SDK docs URL | 1 |
  | AWS News Blog post announcing two features | 1 |

  Followed references only inherit errors: each of the 5 came through an issue that was itself wrongly matched.
- **Retrieval is still good.** 88% of cases with an answer have it in the top 5, and recall is 96%.
- **`new` and `no-work` pass trivially.** Keyword-only matches are always "related", so these confirm Radar doesn't invent strong matches, but they can't fail any other way.
