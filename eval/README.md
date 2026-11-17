# Evaluation

`avizor eval` runs a hand-labeled corpus through reconciliation and reports how often Avizor finds the right existing work. Use it before and after changing retrieval or categorization rules, so changes are measured instead of guessed.

```
avizor eval --format text              # all cases, with failures explained
avizor eval --format text --failures   # failing cases only
avizor eval dup-41552 impl-45278       # selected cases
avizor eval --format text --unjudged   # confident candidates that still need a same or different label
avizor eval                            # JSON report
```

The corpus is [`tfaws.hcl`](tfaws.hcl), built into the `avizor` binary, so `avizor eval` works without a source checkout. Use `--corpus path` for another file. Eval runs offline: it reads only the local index (and the checkout for cases with `code = true`). It doesn't fetch issues or pages, so results are repeatable. It runs 4 cases at a time (`--jobs`), and the full corpus takes about 15 seconds.

## Index requirements

The cases reference provider issues filed since 2025-01-01. Sync at least that far back:

```
avizor sync --since 2025-01-01
```

A case whose input issue isn't indexed fails with an error rather than being skipped.

Item cases need pulled SDK items and the models clone:

```
avizor pull --since 2025-11-30
```

## Point in time

Each case reconciles against the index as it looked at the case's `as_of` time. Without that, the answer leaks into the result: a duplicate's original would show as closed, or the PR that implemented an announcement would appear before the announcement was even filed.

`as_of` is `"filed"` (the default for issue inputs: when the input was created), `"now"` (the default for text and URLs), a date (the end of that day, UTC), or an RFC 3339 time.

The reconstruction is approximate. Issues created later are hidden, including from the full-text ranking statistics and from cross-references in the input, signals from comments posted later are ignored, and issues closed or merged later are shown open. Titles, bodies, and labels are their current versions, since edits aren't tracked. Code searches use the checkout as it is now, so they're off unless a case sets `code = true`.

Issue inputs are recognized the same way `avizor candidates` recognizes them: a number, or an issue or PR URL for the target repo, with or without a fragment such as `#issuecomment-1`. Another repository's issue URL is treated as a plain URL.

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
  # state   = "covered"        # item cases only: expected queue state
}
```

`same_paths` and `different_paths` judge code and docs candidates in cases with `code = true`.

A case passes when every expected candidate is in the top 5 with the expected category, no `different` candidate has a confident category, and `unmatched` and `state` match if set. With `code = true`, item enrichment searches the checkout too, as the queue does.

## Judging confident candidates

Avizor is used in a human/AI workflow, where misleading information is worse than none. A duplicate shown as "related" is weak but honest. A candidate shown as "already tracked" or "already implemented" that is the wrong work is confidently wrong.

So every candidate Avizor returns with a confident category (anything above "related"), at any rank, needs a verdict: it's an expected candidate, labeled `same`, or labeled `different`. Judgments come from reading the candidate's title and body, not from its category. `avizor eval --format text --unjudged` lists the ones still missing. After a change to reconciliation, label any new ones before reading precision.

## Kinds and where labels come from

Labels come from repository history, not from Avizor's output.

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
| `item-tracked` | 38 | Pulled SDK items whose release commit a tracker or PR with the item's service label cites, 30 minutes after the later of the release and the last such tracker. That's every such item from 2025-12-01 to 2026-11-11 except one only its bump PR cites. Avizor matches on the same commit, so these mostly measure precision on the other candidates, not retrieval. |
| `item-new` | 34 | The same items a minute before their first tracker was filed, when that was after the release. Nothing should be confident. |
| `item-no-work` | 19 | Items that need no provider work: one cited only by its bump PR, and 18 from the gap sample. |
| `item-gap` | 8 | Gap-sample items that need provider work nothing covers yet. |
| `item-covered` | 4 | Gap-sample items that existing code, an issue, or a PR covers, though no tracker cites the release commit. |
| `item-covered-sample` | 30 | The covered sample: random `covered` items, each hand-checked for its queue state (`covered` or `partial`) and with every confident candidate judged. |
| `item-partial` | 15 | Covered items the first `partial` run flagged, outside the sample, hand-checked: `partial` when some work has no code or issue, `covered` when the launch's tracker covers it. |

The gap sample tests the claim behind every gap draft: that nothing covers the item. Items in implemented services, with the SDK version available and some inferred work, were drafted in a seeded random order until 30 had no confident existing work (32 drafted; the other 2 had correct confident candidates). Each was checked by hand against provider code and a GitHub search of issues and PRs. These labels don't come from the release-commit signal Avizor matches on.

The covered sample tests the opposite claim: that a `covered` item, which the queue hides, needs nothing more. It's 30 of the 61 covered items that aren't other item cases, in a seeded random order (seed 13). Two items (Network Firewall and DevOps Agent) were first labeled `covered` and relabeled `partial` after the queue flagged them, so those two labels could be biased toward Avizor's output.

`same` and `different` labels were added by reading each confident candidate. Gap-sample cases were judged against provider `main` at `37436a92a45` (2026-11-12), and covered-sample and `item-partial` cases at `873712b39f8` (2026-11-13).

## Metrics

| Column | Meaning |
|---|---|
| `hit@k` | Cases where an expected candidate is in the top k. |
| `recall` | Expected candidates returned at any rank. |
| `category` | Expected candidates with a labeled category that got that category. |
| `unmatch` | Cases labeling `unmatched` that got it right. |
| `state` | Item cases labeling a queue state that got it. |
| `precision` | Confident candidates that are the right work (expected or `same`), out of those judged. |
| `wrong` | Confident candidates labeled `different`. Each fails its case. |
| `unjudged` | Confident candidates with no verdict. Should be 0. |

The report also counts cases with existing work that were summarized as unmatched, and ends with the index it ran against: when it was synced and how many issues, PRs, signals, and comments with signals it holds. Results can move when the index does, even for cases pinned with `as_of`, because a sync can add or correct records created before a case's `as_of`. Compare runs only on the same index, and note the index line with any result.

## Results

Rows before the resync used an index synced from 2025-01-01 (15,493 issues and PRs, 52,167 comments). Later rows use the full index synced 2026-11-16 (23,644 issues, 27,183 PRs, 100,842 comments with signals).

| Change | pass | hit@1 | hit@3 | hit@5 | recall | category | precision |
|---|---|---|---|---|---|---|---|
| Milestone 2 baseline | 27 | 69% | 86% | 90% | 88% | 37% | - |
| View-scoped full-text ranking (replaces FTS5 `bm25()`) | 27 | 71% | 83% | 90% | 88% | 37% | - |
| Follow references from matched issues to PRs | 30 | 69% | 83% | 88% | 96% | 47% | - |
| Judge confident candidates ([#3](https://github.com/YakDriver/avizor/issues/3)) | 27 | 69% | 83% | 88% | 96% | 47% | 56% |
| Count only real API operation names as operations | 27 | 71% | 86% | 90% | 96% | 47% | 62% |
| Add 73 SDK item cases (128 cases in all) | 98 | 82% | 92% | 95% | 98% | 73% | 78% |
| Add 30 gap-sample item cases (158 cases in all) | 124 | 79% | 88% | 93% | 96% | 69% | 78% |
| Add 45 covered-sample and partial cases (203 in all) | 166 | 79% | 88% | 93% | 96% | 69% | 89% |
| Resync the full index, no code change ([#34](https://github.com/YakDriver/avizor/issues/34)) | 166 | 71% | 87% | 90% | 96% | 69% | 89%, 87 unjudged |
| Docs URLs are common above 5 records ([#34](https://github.com/YakDriver/avizor/issues/34)), 11 new labels | 165 | 74% | 89% | 93% | 95% | 68% | 90% |
| New member names and the matched announcement count for items ([#23](https://github.com/YakDriver/avizor/issues/23)), 3 new cases (206 in all), 31 new labels | 166 | 75% | 90% | 93% | 95% | 71% | 90% |
| Sub-service labels count as the item's service, and followed references need one ([#22](https://github.com/YakDriver/avizor/issues/22)), 1 new case (207 in all) | 170 | 75% | 90% | 93% | 95% | 71% | 92% |
| Bot PRs aren't followed ([#39](https://github.com/YakDriver/avizor/issues/39)) | 170 | 76% | 91% | 93% | 95% | 71% | 93% |
| Followed references are strong only from PRs that close the issue or have their own evidence ([#38](https://github.com/YakDriver/avizor/issues/38)) | 172 | 76% | 91% | 93% | 95% | 71% | 94% |

Judging confident candidates changed no reconciliation code: three cases fail because they return different work with a confident category. Checking operation names against the API models removed the 6 wrong labels that came from `StartTime`, and lost nothing. Rows that add cases aren't comparable with the rows above them. On the earlier cases they're unchanged.

The resync exposed 87 new confident candidates, all from reference documentation URLs (a resource's Terraform Registry page, AWS API reference pages, pkg.go.dev, an AWS XML namespace) or references followed from them; 62 came from one registry URL. Such a URL names a resource or API that many unrelated issues discuss. Treating it as common above 5 records, instead of 40, removes 76 of them. Making docs URLs always weak removes 81 but loses 4 expected `already_tracked` matches that rest on one. Labeling the remaining 11 fails `dup-41200`: four dependency-bump PRs quote upstream changelogs that cite another repository's #723, which matches the input.

For items, two kinds of evidence in a candidate's title or body used to count for nothing: the What's New post the item matched, and the names of the members it adds. Now the matched announcement URL is strong, like the release commit, when the match scores at least 0.5. A new member named in the title as whole words (the API name, snake_case, or "Route 53 Resolved IP Validation") is strong when the candidate was created from 2 days before the release to 60 days after. In the body it's weak: config examples and lists of SDK fields name members the issue isn't about. A member whose name another shape already had, as when an update request gains its create request's member, doesn't count. The 3 new `item-covered` cases are the queue-pass misses this fixes. In the 30-day queue, 9 pending items now show as covered, each by an open tracker and PR. The 3 new wrong labels come through a followed reference from a PR that only relates to a tracker (#48699, #50777), and a title naming a new structure for a later member (#45800).

An item's candidates need the item's service label for operation, URL, and member matches to be strong. EC2 trackers and PRs often carry only a sub-service label, such as `service/transitgateway` or `service/vpc`, so they stayed related even when they cited the release commit. The labels of the `sub_service` blocks in `names_data.hcl` (9, all under `ec2`) now count. Following references used to skip the label check: a launch tracker that spans two services passed its strong match to every PR that referenced it, including the other service's half. Now a PR labeled only with other services inherits a weak match. A PR with no service label keeps the strong one. On the same index (synced 2026-11-17T16:11:16Z), main passes 166 of the 206 earlier cases with 27 wrong; this passes 169 of them with 22 wrong. The 5 removed wrong labels are the networkmanager item's 3 EC2 PRs and 2 PRs that only relate to a tracker (#48693, #48699). The new case, `item-covered-ec2-2026-10-22`, was `gap` before. In the 30-day queue, it and `ec2/release-2026-11-11` now show as covered, both correctly.

Following references gave every PR that mentioned a confident issue the issue's strong match. Two kinds of PR got it wrongly. Dependabot PRs quote upstream changelogs, and the `#N` in them refer to other repositories; the 4 in `dup-41200` matched through another repository's #723. PRs by bots (`[bot]` logins) aren't followed now. Other PRs only relate to a tracker: a docs change, a deprecation, another feature of the same resource. Now a followed PR is strong only if it closes the issue (`Closes`, `Fixes`, or `Resolves`, with `#N` or the issue URL) or says it's `Part of #N`, or, for an item, has its own evidence: a new member named in its title or body, a new operation, or a title naming a Terraform type for a noun a new operation creates. Otherwise the match is weak. On the same index, confident candidates go from 271 to 263 and wrong ones from 22 to 16: the 4 bot PRs, #48693, and #50777. Two cases now pass. Two labeled-correct PRs lose their match because they only say "Relates": #50341 (`lambdaweb` resources, which relates to its new-service issue) and #50121 (which says it's about a different part of the feature). No queue state changes in the 30-day (237 items) or 120-day (693 items) queues; 5 covered items list fewer PRs.

The new cases measure the two rules the covered sample led to, which only show with code search. Without the existing-code rule, the 30 covered-sample cases have 25 wrong confident candidates and 22 pass. Without the `partial` state, state accuracy over the 45 cases drops from 98% to 73%.

Current, by kind:

```
FAIL  dup-40793                                duplicate    40793
      - #38644 is related, want already_tracked
      top: 1. #38644 (related), 2. #34935 (related), 3. #35596 (related), 4. #38599 (related), 5. #20363 (related)
FAIL  dup-41200                                duplicate    41200
      - #41132 is already_implemented, but is different work
      - #41169 is already_implemented, but is different work
      - #18396 is already_implemented, but is different work
      - #18395 is already_implemented, but is different work
      - #18098 is previously_considered, but is different work
      top: 1. #41132 (implemented), 2. #41169 (implemented), 3. #18396 (implemented), 4. #18395 (implemented), 5. #38983 (tracked)
FAIL  dup-41552                                duplicate    41552
      - #40135 is related, want already_tracked
      - #35628 is already_implemented, but is different work
      top: 1. #35628 (implemented), 2. #40181 (tracked), 3. #40135 (related), 4. #26668 (related), 5. #33897 (related)
FAIL  dup-41782                                duplicate    41782
      - #41710 is related, want already_tracked
      top: 1. #41710 (related), 2. #28851 (related), 3. #31299 (related), 4. #36793 (related), 5. #41592 (related)
FAIL  dup-41973                                duplicate    41973
      - #41710 not returned
      top: 1. #41782 (related), 2. #41537 (related), 3. #41662 (related), 4. #41521 (related), 5. #41399 (related)
FAIL  dup-42127                                duplicate    42127
      - #42125 is related, want already_tracked
      top: 1. #42125 (related), 2. #10004 (related), 3. #7516 (related), 4. #31131 (related), 5. #26363 (related)
FAIL  dup-42605                                duplicate    42605
      - #40597 is related, want previously_considered
      - #35931 is already_tracked, but is different work
      top: 1. #35931 (tracked), 2. #40597 (related), 3. #37456 (related), 4. #42536 (related), 5. #17200 (related)
FAIL  dup-43223                                duplicate    43223
      - #39383 not returned
      top: 1. #36958 (related), 2. #38859 (related), 3. #38108 (related), 4. #40323 (related), 5. #34630 (related)
FAIL  dup-43828                                duplicate    43828
      - #42850 is related, want already_tracked
      top: 1. #42850 (related), 2. #37253 (related), 3. #41285 (related), 4. #41140 (related), 5. #40576 (related)
FAIL  dup-44751                                duplicate    44751
      - #14864 is already_implemented, but is different work
      - #39447 is already_tracked, but is different work
      - #13888 is previously_considered, but is different work
      top: 1. #14864 (implemented), 2. #44672 (tracked), 3. #39447 (tracked), 4. #13888 (considered), 5. #35489 (related)
FAIL  dup-45838                                duplicate    45838
      - #44836 is related, want already_tracked
      - #38345 is already_implemented, but is different work
      - #43043 is already_tracked, but is different work
      - #38231 is previously_considered, but is different work
      top: 1. #38345 (implemented), 2. #43043 (tracked), 3. #38231 (considered), 4. #44836 (related), 5. #44857 (related)
FAIL  dup-46606                                duplicate    46606
      - #46322 is related, want already_tracked
      top: 1. #46322 (related), 2. #46285 (related), 3. #43049 (related), 4. #44692 (related), 5. #24683 (related)
FAIL  dup-47486                                duplicate    47486
      - #47022 is related, want already_tracked
      top: 1. #47022 (related), 2. #46998 (related), 3. #19407 (related), 4. #44490 (related), 5. #19125 (related)
FAIL  dup-47987                                duplicate    47987
      - #44688 is related, want already_tracked
      top: 1. #29871 (related), 2. #44688 (related), 3. #43972 (related), 4. #29783 (related), 5. #41305 (related)
FAIL  dup-48354                                duplicate    48354
      - #45987 is related, want already_tracked
      top: 1. #48353 (related), 2. #46790 (related), 3. #48345 (related), 4. #45987 (related), 5. #48349 (related)
FAIL  dup-48760                                duplicate    48760
      - #48554 is related, want already_tracked
      top: 1. #48554 (related), 2. #45745 (related), 3. #45588 (related), 4. #40996 (related), 5. #42708 (related)
FAIL  dup-49183                                duplicate    49183
      - #43509 is related, want already_tracked
      top: 1. #43509 (related), 2. #43621 (related), 3. #41026 (related), 4. #44626 (related), 5. #38012 (related)
FAIL  dup-49365                                duplicate    49365
      - #46315 is already_tracked, but is different work
      top: 1. #46315 (tracked), 2. #45533 (tracked), 3. #14619 (considered), 4. #33088 (considered), 5. #26082 (related)
FAIL  dup-49894                                duplicate    49894
      - #47673 not returned
      top: 1. #46281 (related), 2. #42445 (related), 3. #48785 (related), 4. #44692 (related), 5. #40141 (related)
FAIL  dup-50540                                duplicate    50540
      - #41838 is related, want already_tracked
      top: 1. #23147 (related), 2. #22008 (related), 3. #41838 (related), 4. #40803 (related), 5. #35721 (related)
FAIL  ann-text-42864                           announcement-text text: Amazon API Gateway now supports routing rules for REST APIs using custom domain names.
      - #42864 is related, want already_tracked
      top: 1. #40274 (related), 2. #42864 (related), 3. #42110 (related), 4. #2866 (related), 5. #420 (related)
FAIL  ann-text-43005                           announcement-text text: AWS WAF now supports automatic application layer distributed denial of service (DDoS) protection
      - #43005 is related, want already_tracked
      top: 1. #43005 (related), 2. #33432 (related), 3. #27066 (related), 4. #35372 (related), 5. #35373 (related)
FAIL  ann-text-45162                           announcement-text text: Amazon FSx for Windows File Server now supports File Server Resource Manager
      - #45162 is related, want already_tracked
      top: 1. #45162 (related), 2. #19793 (related), 3. #12630 (related), 4. #11500 (related), 5. #12676 (related)
FAIL  ann-text-47671                           announcement-text text: AWS KMS now tracks last usage of all KMS keys
      - #47671 is related, want already_tracked
      top: 1. #47671 (related), 2. #43671 (related), 3. #27422 (related), 4. #27611 (related), 5. #34538 (related)
FAIL  ann-text-49350                           announcement-text text: Amazon DynamoDB now supports real-time vector search
      - #49350 is related, want already_tracked
      top: 1. #49350 (related), 2. #49342 (related), 3. #41484 (related), 4. #41035 (related), 5. #41504 (related)
FAIL  ann-text-44928                           announcement-text text: Today, AWS announced the ability for Amazon VPC IP Address Manager (IPAM) to automate prefix lists updates with prefix list resolver (PLR).
      - #44928 is related, want already_tracked
      top: 1. #36220 (related), 2. #39600 (related), 3. #44928 (related), 4. #26806 (related), 5. #5293 (related)
FAIL  rel-45361                                related      45361
      - #45353 is already_tracked, but is different work
      - #45360 is already_tracked, but is different work
      top: 1. #45353 (tracked), 2. #45360 (tracked), 3. #45358 (related), 4. #6589 (related), 5. #6588 (related)
FAIL  dev-45362-code                           dev          45362
      - #50872 ranked 6, beyond top 5
      - #43391 is already_implemented, but is different work
      - #43195 is previously_considered, but is different work
      top: 1. internal/service/fsx/s3_access_point_attachment.go (implemented), 2. #50917 (implemented), 3. internal/service/fsx/sweep.go (implemented), 4. website/docs/r/fsx_s3_access_point_attachment.html.markdown (implemented), 5. #43391 (implemented)
FAIL  dev-45362-url                            dev          https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/
      - #45362 is previously_considered, want already_tracked
      top: 1. #50917 (implemented), 2. #45362 (considered)
FAIL  item-tracked-agentregistrycontrol-2026-08-06 item-tracked item: aws-sdk-go-v2/agentregistrycontrol/release-2026-08-06
      - #48693 is already_implemented, but is different work
      top: 1. #48693 (implemented), 2. #48694 (tracked), 3. #48501 (related), 4. #48564 (related), 5. #49027 (related)
FAIL  item-tracked-bedrockagentcorecontrol-2026-08-06 item-tracked item: aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-08-06
      - #48693 is already_implemented, but is different work
      top: 1. #48693 (implemented), 2. #49344 (tracked), 3. #49352 (tracked), 4. #48694 (tracked), 5. #43685 (related)
FAIL  item-covered-arcregionswitch-2026-02-27  item-covered item: aws-sdk-go-v2/arcregionswitch/release-2026-02-27
      - internal/service/arcregionswitch/plan.go not returned
      top: 1. #46965 (implemented), 2. #48382 (related), 3. #47282 (related), 4. #47168 (related), 5. #47167 (related)
FAIL  item-covered-bedrockagent-2026-08-28-2   item-covered item: aws-sdk-go-v2/bedrockagent/release-2026-08-28.2
      - internal/service/bedrockagent/data_source.go not returned
      top: 1. #49958 (tracked), 2. #49888 (tracked), 3. #44060 (related), 4. #45466 (related), 5. #48744 (related)
FAIL  item-covered-cloudwatch-2026-05-01       item-covered item: aws-sdk-go-v2/cloudwatch/release-2026-05-01
      - #48548 ranked 7, beyond top 5
      - #48548 is related, want already_tracked
      top: 1. #49472 (related), 2. #49507 (related), 3. #46511 (related), 4. #5729 (related), 5. #14871 (related)
FAIL  item-covered-dynamodb-2026-10-20         item-covered item: aws-sdk-go-v2/dynamodb/release-2026-10-20
      - #50777 is already_tracked, but is different work
      top: 1. #50616 (tracked), 2. #50656 (tracked), 3. #50619 (tracked), 4. #50777 (tracked), 5. #48672 (related)
FAIL  item-partial-networkmanager-2026-10-22   item-partial item: aws-sdk-go-v2/networkmanager/release-2026-10-22
      - #50669 is already_tracked, but is different work
      - #50683 is already_tracked, but is different work
      - #50672 is already_tracked, but is different work
      top: 1. #50689 (tracked), 2. #50700 (tracked), 3. #50613 (tracked), 4. #50690 (tracked), 5. #50669 (tracked)
FAIL  item-partial-securityhub-2026-07-07      item-partial item: aws-sdk-go-v2/securityhub/release-2026-07-07
      - #49559 is already_implemented, but is different work
      top: 1. #49559 (implemented), 2. #49503 (implemented), 3. internal/service/securityhub/feature_v2.go (implemented), 4. #49560 (implemented), 5. #49568 (considered)
FAIL  item-partial-bedrockagentcorecontrol-2026-06-17 item-partial item: aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-06-17
      - #48699 is already_implemented, but is different work
      top: 1. #48654 (implemented), 2. #48699 (implemented), 3. #48791 (tracked), 4. #48710 (tracked), 5. #48661 (tracked)
FAIL  item-partial-sesv2-2026-05-29            item-partial item: aws-sdk-go-v2/sesv2/release-2026-05-29
      - state = partial (no code or candidate found for argument suppression_attributes; covered: already tracked #49357, already tracked #48236, already tracked #50861), want covered
      top: 1. #49357 (tracked), 2. #48236 (tracked), 3. #50861 (tracked), 4. #26796 (related), 5. #38207 (related)
FAIL  item-partial-lambda-2025-12-02           item-partial item: aws-sdk-go-v2/lambda/release-2025-12-02
      - #45800 is already_tracked, but is different work
      top: 1. #45359 (implemented), 2. internal/service/lambda/function.go (implemented), 3. #45800 (tracked), 4. #45354 (considered), 5. #43395 (related)

kind               cases  pass  hit@1  hit@3  hit@5 recall category unmatch  state precision wrong unjudged
announcement           6     6   100%   100%   100%   100%     100%       -      -      100%     0        0
announcement-text      6     0    67%   100%   100%   100%       0%       -      -         -     0        0
dev                    4     2    50%   100%   100%   100%      80%    100%      -       80%     2        0
duplicate             20     0    40%    70%    85%    85%      15%       -      -       33%    14        0
implemented            6     6   100%   100%   100%   100%     100%       -      -      100%     0        0
item-covered           7     3    57%    57%    57%    75%      62%       -      -       92%     1        0
item-covered-sample    30    30      -      -      -      -        -       -   100%      100%     0        0
item-gap               8     8      -      -      -      -        -    100%      -         -     0        0
item-new              34    34      -      -      -      -        -    100%      -         -     0        0
item-no-work          19    19      -      -      -      -        -    100%      -         -     0        0
item-partial          15    10      -      -      -      -        -       -    93%       89%     6        0
item-tracked          38    36    92%   100%   100%   100%     100%       -      -       96%     2        0
new                    5     5      -      -      -      -        -    100%      -         -     0        0
no-work                4     4      -      -      -      -        -    100%      -         -     0        0
related                4     3      -      -      -      -        -       -      -        0%     2        0
overall              206   166    75%    90%    93%    95%      71%    100%    98%       90%    27        0
21 case(s) with existing work were summarized as unmatched
index: synced 2026-11-16T17:38:49Z, 23644 issues, 27183 PRs, 630263 signals, 100842 comments with signals
```

What it shows:

- **Item reconciliation is mostly precise.** Of 221 confident candidates in item cases, 9 are wrong. Two are #48693, a PR that deprecates `aws_bedrockagentcore_registry` and inherits a strong match from its tracker through a followed reference. No `item-new` case gets a confident candidate before its tracker exists.
- **Duplicate bug reports are the weak spot.** In the other cases, 18 of 51 confident candidates are different work, and 14 of those are in duplicate cases. A correct answer usually shows as "related" while unrelated issues show as "already tracked".
- **"No existing work" holds for 26 of 30 gap-sample items.** The 4 misses are covered work. With member names, 3 now show it confidently: the PRs for arcregionswitch's RDS execution blocks and bedrockagent's `sync_schedule`, and the open PR #48670 for the Cognito user pool replica. The first two still fail because code search looks only for new operations, not new members. #48548, for CloudWatch dashboard tags, is still "related" at rank 7.
- **Most untracked items need no provider work.** Of the 30, 18 need none, 8 are real gaps, and 4 are covered.
- **Covered items are almost never wrongly hidden.** All 45 covered-sample and `item-partial` cases but one get the right state. The miss is SES tenant suppression attributes, which the queue shows as `partial` although #49357 and #50861 cover it.
- **Item candidates on a launch's other services are the new wrong labels.** In `item-partial`, 6 confident candidates are wrong: the Cloud WAN NAT item gets three EC2 Transit Gateway NAT PRs through their reference to the shared tracker, the Security Hub multicloud item gets the Config connector PR, a docs PR relates to the AgentCore WAF tracker, and a later durable-functions request names `durable_config` in its title.
- **The remaining wrong labels trace to broad strong signals:** docs URLs found in 5 or fewer records, followed references through a wrongly matched issue, issue numbers from other repositories quoted in dependency-bump PRs, a shared operation name, and a multi-feature blog post. [#3](https://github.com/YakDriver/avizor/issues/3) tracks reducing them.
