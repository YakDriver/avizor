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

Item cases need pulled SDK items and the models clone:

```
radar pull --since 2025-11-30
```

## Point in time

Each case reconciles against the index as it looked at the case's `as_of` time. Without that, the answer leaks into the result: a duplicate's original would show as closed, or the PR that implemented an announcement would appear before the announcement was even filed.

`as_of` is `"filed"` (the default for issue inputs: when the input was created), `"now"` (the default for text and URLs), a date (the end of that day, UTC), or an RFC 3339 time.

The reconstruction is approximate. Issues created later are hidden, including from the full-text ranking statistics and from cross-references in the input, signals from comments posted later are ignored, and issues closed or merged later are shown open. Titles, bodies, and labels are their current versions, since edits aren't tracked. Code searches use the checkout as it is now, so they're off unless a case sets `code = true`.

Issue inputs are recognized the same way `radar candidates` recognizes them: a number, or an issue or PR URL for the target repo, with or without a fragment such as `#issuecomment-1`. Another repository's issue URL is treated as a plain URL.

## Case format

```hcl
case "dup-41552" {
  input = "41552"              # issue number or URL; or text = "..."; or item = "<item id>"
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
| `new` | 5 | Headlines a minute before their tracker was filed. Nothing should be tracked yet. |
| `no-work` | 4 | aws-sdk-go-v2 release notes that need no provider work (protocol, serialization, and retry changes). |
| `dev` | 4 | Queries used while building milestone 1. Reported separately because the rules were tuned on them. |
| `item-tracked` | 38 | Pulled SDK items whose release commit a tracker or PR with the item's service label cites, 30 minutes after the later of the release and the last such tracker. That's every such item from 2025-12-01 to 2026-11-11 except one only its bump PR cites. Radar matches on the same commit, so these mostly measure precision on the other candidates, not retrieval. |
| `item-new` | 34 | The same items a minute before their first tracker was filed, when that was after the release. Nothing should be confident. |
| `item-no-work` | 1 | An item that needs no provider work (an API only AWS services call), cited only by its bump PR. |

`same` and `different` labels were added by reading each confident candidate. Cases with `code = true` were judged against provider `main` at `37436a92a45` (2026-11-12).

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
| Count only real API operation names as operations | 27 | 71% | 86% | 90% | 96% | 47% | 62% |
| Add 73 SDK item cases (128 cases in all) | 98 | 82% | 92% | 95% | 98% | 73% | 78% |

Judging confident candidates changed no reconciliation code: three cases fail because they return different work with a confident category. Checking operation names against the API models removed the 6 wrong labels that came from `StartTime`, and lost nothing. The last row adds cases, so its numbers aren't comparable with the rows above it. On the first 55 cases they're unchanged.

Current, by kind:

```
kind               cases  pass  hit@1  hit@3  hit@5 recall category unmatch precision wrong unjudged
announcement           6     6   100%   100%   100%   100%     100%       -      100%     0        0
announcement-text      6     0    83%   100%   100%   100%       0%       -         -     0        0
dev                    4     3    75%   100%   100%   100%     100%    100%       80%     2        0
duplicate             20     0    50%    70%    80%    90%      15%       -       30%    16        0
implemented            6     6   100%   100%   100%   100%     100%       -      100%     0        0
item-new              34    34      -      -      -      -        -    100%         -     0        0
item-no-work           1     1      -      -      -      -        -    100%         -     0        0
item-tracked          38    36    95%   100%   100%   100%     100%       -       96%     2        0
new                    5     5      -      -      -      -        -    100%         -     0        0
no-work                4     4      -      -      -      -        -    100%         -     0        0
related                4     3      -      -      -      -        -       -        0%     2        0
overall              128    98    82%    92%    95%    98%      73%    100%       78%    22        0
18 case(s) with existing work were summarized as unmatched
```

What it shows:

- **Item reconciliation is mostly precise.** Of 48 confident candidates in item cases, 2 are wrong. Both are #48693, a PR that deprecates `aws_bedrockagentcore_registry` and inherits a strong match from its tracker through a followed reference. No `item-new` case gets a confident candidate before its tracker exists.
- **Duplicate bug reports are the weak spot.** In the other cases, 20 of 53 confident candidates are different work, and 16 of those are in duplicate cases. A correct answer usually shows as "related" while unrelated issues show as "already tracked".
- **The remaining wrong labels trace to broad strong signals:** Terraform Registry docs URLs, followed references through a wrongly matched issue, AWS and pkg.go.dev docs URLs, a shared operation name, and a multi-feature blog post. [#3](https://github.com/YakDriver/radar/issues/3) tracks reducing them.
