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
| `item-no-work` | 19 | Items that need no provider work: one cited only by its bump PR, and 18 from the gap sample. |
| `item-gap` | 8 | Gap-sample items that need provider work nothing covers yet. |
| `item-covered` | 4 | Gap-sample items that existing code, an issue, or a PR covers, though no tracker cites the release commit. |

The gap sample tests the claim behind every gap draft: that nothing covers the item. Items in implemented services, with the SDK version available and some inferred work, were drafted in a seeded random order until 30 had no confident existing work (32 drafted; the other 2 had correct confident candidates). Each was checked by hand against provider code and a GitHub search of issues and PRs. These labels don't come from the release-commit signal Radar matches on.

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
| Add 30 gap-sample item cases (158 cases in all) | 124 | 79% | 88% | 93% | 96% | 69% | 78% |

Judging confident candidates changed no reconciliation code: three cases fail because they return different work with a confident category. Checking operation names against the API models removed the 6 wrong labels that came from `StartTime`, and lost nothing. The last two rows add cases, so their numbers aren't comparable with the rows above them. On the earlier cases they're unchanged.

Current, by kind:

```
FAIL  dup-40793                                duplicate    40793
      - #38644 is related, want already_tracked
      top: 1. #38644 (related), 2. #34935 (related), 3. #33911 (related), 4. #38599 (related), 5. #28187 (related)
FAIL  dup-41200                                duplicate    41200
      - #38983 is related, want already_tracked
      top: 1. #38983 (related), 2. #11068 (related), 3. #29753 (related), 4. #39308 (related), 5. #34408 (related)
FAIL  dup-41552                                duplicate    41552
      - #40135 is related, want already_tracked
      top: 1. #40181 (tracked), 2. #39722 (related), 3. #25506 (related), 4. #40135 (related), 5. #40056 (related)
FAIL  dup-41782                                duplicate    41782
      - #41710 ranked 6, beyond top 5
      - #41710 is related, want already_tracked
      - #41462 is already_implemented, but is different work
      - #31299 is already_tracked, but is different work
      - #41199 is already_tracked, but is different work
      - #41126 is already_tracked, but is different work
      - #41620 is previously_considered, but is different work
      top: 1. #41462 (implemented), 2. #31299 (tracked), 3. #41199 (tracked), 4. #41126 (tracked), 5. #41620 (considered)
FAIL  dup-41973                                duplicate    41973
      - #41710 ranked 16, beyond top 5
      - #41710 is related, want already_tracked
      - #41462 is already_implemented, but is different work
      - #41199 is already_tracked, but is different work
      - #31299 is already_tracked, but is different work
      - #41126 is already_tracked, but is different work
      - #41620 is previously_considered, but is different work
      top: 1. #41462 (implemented), 2. #41199 (tracked), 3. #31299 (tracked), 4. #41126 (tracked), 5. #41782 (considered)
FAIL  dup-42127                                duplicate    42127
      - #42125 is related, want already_tracked
      top: 1. #31131 (related), 2. #40599 (related), 3. #42125 (related), 4. #33436 (related), 5. #26594 (related)
FAIL  dup-42605                                duplicate    42605
      - #40597 is related, want previously_considered
      - #35931 is already_tracked, but is different work
      top: 1. #35931 (tracked), 2. #40597 (related), 3. #37456 (related), 4. #42536 (related), 5. #40493 (related)
FAIL  dup-43223                                duplicate    43223
      - #39383 not returned
      top: 1. #36958 (related), 2. #38859 (related), 3. #38108 (related), 4. #40323 (related), 5. #34630 (related)
FAIL  dup-43828                                duplicate    43828
      - #41241 is already_tracked, but is different work
      - #41384 is already_tracked, but is different work
      top: 1. #42850 (tracked), 2. #41241 (tracked), 3. #41384 (tracked), 4. #37253 (related), 5. #41285 (related)
FAIL  dup-44751                                duplicate    44751
      - #39447 is already_tracked, but is different work
      top: 1. #44672 (tracked), 2. #39447 (tracked), 3. #33168 (related), 4. #41116 (related), 5. #43715 (related)
FAIL  dup-45838                                duplicate    45838
      - #44836 is related, want already_tracked
      - #43043 is already_tracked, but is different work
      top: 1. #43043 (tracked), 2. #44836 (related), 3. #44857 (related), 4. #43316 (related), 5. #43382 (related)
FAIL  dup-46606                                duplicate    46606
      - #46322 is related, want already_tracked
      top: 1. #46322 (related), 2. #46285 (related), 3. #43049 (related), 4. #44692 (related), 5. #41343 (related)
FAIL  dup-47486                                duplicate    47486
      - #47022 is related, want already_tracked
      top: 1. #47022 (related), 2. #46998 (related), 3. #46682 (related), 4. #44490 (related), 5. #46507 (related)
FAIL  dup-47987                                duplicate    47987
      - #44688 is related, want already_tracked
      top: 1. #44688 (related), 2. #43972 (related), 3. #29871 (related), 4. #41305 (related), 5. #32793 (related)
FAIL  dup-48354                                duplicate    48354
      - #45987 is related, want already_tracked
      top: 1. #48353 (related), 2. #46790 (related), 3. #48345 (related), 4. #45987 (related), 5. #48349 (related)
FAIL  dup-48760                                duplicate    48760
      - #48554 is related, want already_tracked
      top: 1. #48554 (related), 2. #45745 (related), 3. #45588 (related), 4. #40996 (related), 5. #42708 (related)
FAIL  dup-49183                                duplicate    49183
      - #43509 is related, want already_tracked
      top: 1. #43509 (related), 2. #43621 (related), 3. #40152 (related), 4. #49075 (related), 5. #25651 (related)
FAIL  dup-49365                                duplicate    49365
      - #46315 is already_tracked, but is different work
      top: 1. #46315 (tracked), 2. #45533 (tracked), 3. #14619 (considered), 4. #33088 (considered), 5. #40639 (related)
FAIL  dup-49894                                duplicate    49894
      - #47673 not returned
      top: 1. #46281 (related), 2. #42445 (related), 3. #48785 (related), 4. #44692 (related), 5. #40141 (related)
FAIL  dup-50540                                duplicate    50540
      - #41838 is related, want already_tracked
      top: 1. #41838 (related), 2. #40803 (related), 3. #49864 (related), 4. #47293 (related), 5. #47547 (related)
PASS  ann-42864                                announcement https://aws.amazon.com/about-aws/whats-new/2025/06/amazon-api-gateway-routing-rules-rest-apis/
FAIL  ann-text-42864                           announcement-text text: Amazon API Gateway now supports routing rules for REST APIs using custom domain names.
      - #42864 is related, want already_tracked
      top: 1. #40274 (related), 2. #42864 (related), 3. #42110 (related), 4. #41375 (related), 5. #41659 (related)
PASS  ann-43005                                announcement https://aws.amazon.com/about-aws/whats-new/2025/06/aws-waf-automatic-application-layer-ddos-protection/
FAIL  ann-text-43005                           announcement-text text: AWS WAF now supports automatic application layer distributed denial of service (DDoS) protection
      - #43005 is related, want already_tracked
      top: 1. #43005 (related), 2. #33432 (related), 3. #42002 (related), 4. #41803 (related), 5. #36854 (related)
PASS  ann-45162                                announcement https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-fsx-windows-file-server-resource-manager/
FAIL  ann-text-45162                           announcement-text text: Amazon FSx for Windows File Server now supports File Server Resource Manager
      - #45162 is related, want already_tracked
      top: 1. #45162 (related), 2. #40817 (related), 3. #40289 (related), 4. #33303 (related), 5. #32116 (related)
PASS  ann-47671                                announcement https://aws.amazon.com/about-aws/whats-new/2026/04/aws-kms-tracks-last-usage-kms-keys/
FAIL  ann-text-47671                           announcement-text text: AWS KMS now tracks last usage of all KMS keys
      - #47671 is related, want already_tracked
      top: 1. #47671 (related), 2. #43671 (related), 3. #27611 (related), 4. #44011 (related), 5. #20490 (related)
PASS  ann-49350                                announcement https://aws.amazon.com/about-aws/whats-new/2026/08/amazon-dynamodb-vector-search/
FAIL  ann-text-49350                           announcement-text text: Amazon DynamoDB now supports real-time vector search
      - #49350 is related, want already_tracked
      top: 1. #49350 (related), 2. #49342 (related), 3. #41484 (related), 4. #41035 (related), 5. #41504 (related)
PASS  ann-44928                                announcement https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-vpc-ipam-prefix-list-automation/
FAIL  ann-text-44928                           announcement-text text: Today, AWS announced the ability for Amazon VPC IP Address Manager (IPAM) to automate prefix lists updates with prefix list resolver (PLR).
      - #44928 is related, want already_tracked
      top: 1. #44928 (related), 2. #38289 (related), 3. #28946 (related), 4. #43966 (related), 5. #37097 (related)
PASS  impl-45278                               implemented  https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-cloudwatch-deletion-protection-logs/
PASS  impl-45354                               implemented  https://aws.amazon.com/about-aws/whats-new/2025/12/lambda-durable-multi-step-applications-ai-workflows/
PASS  impl-47389                               implemented  https://aws.amazon.com/about-aws/whats-new/2026/04/amazon-eks-managed-node-groups-ec2-warm-pools/
PASS  impl-48487                               implemented  https://aws.amazon.com/about-aws/whats-new/2026/05/amazon-mq-private-network-connectivity/
PASS  impl-45014                               implemented  https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-vpc-lattice-custom-domain-name-resource-configuration/
PASS  impl-45219                               implemented  https://aws.amazon.com/about-aws/whats-new/2025/11/announcing-amazon-ecs-express-mode/
PASS  rel-45362                                related      45362
PASS  rel-47610                                related      47610
PASS  rel-47542                                related      47542
FAIL  rel-45361                                related      45361
      - #45353 is already_tracked, but is different work
      - #45360 is already_tracked, but is different work
      top: 1. #45353 (tracked), 2. #45360 (tracked), 3. #45358 (related), 4. #45138 (related), 5. #42824 (related)
PASS  new-45193                                new          text: EC2 Auto Scaling introduces instance lifecycle policy
PASS  new-50573                                new          text: AWS announces Amazon Elastic Container Service (Amazon ECS) Gateway, a new resource that simplifies ingress networking for containerized applications by dynamically provisioning and managing Application Load Balancers (ALBs) and their supporting resources directly in your account.
PASS  new-42410                                new          text: Today, AWS announced a new AWS Client VPN feature that monitors device networking routes, prevents VPN traffic leaks, and strengthens remote access security.
PASS  new-43082                                new          text: Amazon GuardDuty Extended Threat Detection now supports Amazon EKS
PASS  new-45219                                new          text: Today, AWS announces Amazon Elastic Container Service (Amazon ECS) Express Mode, a new feature that empowers developers to rapidly launch containerized applications, including web applications and APIs.
PASS  nowork-cbor-codebuild                    no-work      text: AWS CodeBuild: This release adds Smithy RPC v2 CBOR as an additional protocol alongside the existing AWS JSON 1.1. The SDK will prioritize its most performant protocol.
PASS  nowork-schema-serde-iam                  no-work      text: IAM: Enable schema-based (de)serialization for this service.
PASS  nowork-retryable-conflict                no-work      text: ARC Region switch: Adds RetryableConflictException to UpdatePlan so AWS SDKs automatically retry updates that conflict with a concurrent change.
PASS  nowork-model-formatting                  no-work      text: Amazon Verified Permissions: Inconsequential changes to model formatting
FAIL  dev-45362-code                           dev          45362
      - #50872 ranked 6, beyond top 5
      - #43391 is already_implemented, but is different work
      - #43195 is previously_considered, but is different work
      top: 1. internal/service/fsx/s3_access_point_attachment.go (implemented), 2. internal/service/fsx/sweep.go (implemented), 3. website/docs/r/fsx_s3_access_point_attachment.html.markdown (implemented), 4. #43391 (implemented), 5. #50917 (tracked)
PASS  dev-45362-url                            dev          https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/
PASS  dev-50921                                dev          50921
PASS  dev-text-fsx                             dev          text: FSx S3 access point support
PASS  item-tracked-acm-2026-06-30              item-tracked item: aws-sdk-go-v2/acm/release-2026-06-30
PASS  item-new-acm-2026-06-30                  item-new     item: aws-sdk-go-v2/acm/release-2026-06-30
FAIL  item-tracked-agentregistrycontrol-2026-08-06 item-tracked item: aws-sdk-go-v2/agentregistrycontrol/release-2026-08-06
      - #48693 is already_implemented, but is different work
      top: 1. #48693 (implemented), 2. #48694 (tracked), 3. #48501 (related), 4. #48564 (related), 5. #49027 (related)
FAIL  item-tracked-bedrockagentcorecontrol-2026-08-06 item-tracked item: aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-08-06
      - #48693 is already_implemented, but is different work
      top: 1. #48693 (implemented), 2. #49344 (tracked), 3. #49352 (tracked), 4. #48694 (tracked), 5. #43685 (related)
PASS  item-tracked-cloudformation-2026-06-30   item-tracked item: aws-sdk-go-v2/cloudformation/release-2026-06-30
PASS  item-new-cloudformation-2026-06-30       item-new     item: aws-sdk-go-v2/cloudformation/release-2026-06-30
PASS  item-tracked-cloudwatchlogs-2026-06-22   item-tracked item: aws-sdk-go-v2/cloudwatchlogs/release-2026-06-22
PASS  item-new-cloudwatchlogs-2026-06-22       item-new     item: aws-sdk-go-v2/cloudwatchlogs/release-2026-06-22
PASS  item-tracked-dynamodb-2026-08-04         item-tracked item: aws-sdk-go-v2/dynamodb/release-2026-08-04
PASS  item-new-dynamodb-2026-08-04             item-new     item: aws-sdk-go-v2/dynamodb/release-2026-08-04
PASS  item-tracked-ec2-2025-12-18              item-tracked item: aws-sdk-go-v2/ec2/release-2025-12-18
PASS  item-new-ec2-2025-12-18                  item-new     item: aws-sdk-go-v2/ec2/release-2025-12-18
PASS  item-tracked-ec2-2026-02-12              item-tracked item: aws-sdk-go-v2/ec2/release-2026-02-12
PASS  item-new-ec2-2026-02-12                  item-new     item: aws-sdk-go-v2/ec2/release-2026-02-12
PASS  item-tracked-ec2-2026-04-22              item-tracked item: aws-sdk-go-v2/ec2/release-2026-04-22
PASS  item-new-ec2-2026-04-22                  item-new     item: aws-sdk-go-v2/ec2/release-2026-04-22
PASS  item-tracked-ecs-2026-04-01              item-tracked item: aws-sdk-go-v2/ecs/release-2026-04-01
PASS  item-new-ecs-2026-04-01                  item-new     item: aws-sdk-go-v2/ecs/release-2026-04-01
PASS  item-tracked-eks-2025-12-01              item-tracked item: aws-sdk-go-v2/eks/release-2025-12-01
PASS  item-new-eks-2025-12-01                  item-new     item: aws-sdk-go-v2/eks/release-2025-12-01
PASS  item-tracked-eks-2026-04-07              item-tracked item: aws-sdk-go-v2/eks/release-2026-04-07
PASS  item-new-eks-2026-04-07                  item-new     item: aws-sdk-go-v2/eks/release-2026-04-07
PASS  item-tracked-eks-2026-06-18              item-tracked item: aws-sdk-go-v2/eks/release-2026-06-18
PASS  item-new-eks-2026-06-18                  item-new     item: aws-sdk-go-v2/eks/release-2026-06-18
PASS  item-tracked-eks-2026-06-30              item-tracked item: aws-sdk-go-v2/eks/release-2026-06-30
PASS  item-new-eks-2026-06-30                  item-new     item: aws-sdk-go-v2/eks/release-2026-06-30
PASS  item-tracked-fsx-2025-12-02              item-tracked item: aws-sdk-go-v2/fsx/release-2025-12-02
PASS  item-new-fsx-2025-12-02                  item-new     item: aws-sdk-go-v2/fsx/release-2025-12-02
PASS  item-tracked-grafana-2026-02-17          item-tracked item: aws-sdk-go-v2/grafana/release-2026-02-17
PASS  item-new-grafana-2026-02-17              item-new     item: aws-sdk-go-v2/grafana/release-2026-02-17
PASS  item-tracked-kinesis-2026-08-31          item-tracked item: aws-sdk-go-v2/kinesis/release-2026-08-31
PASS  item-new-kinesis-2026-08-31              item-new     item: aws-sdk-go-v2/kinesis/release-2026-08-31
PASS  item-tracked-kms-2026-04-27              item-tracked item: aws-sdk-go-v2/kms/release-2026-04-27
PASS  item-new-kms-2026-04-27                  item-new     item: aws-sdk-go-v2/kms/release-2026-04-27
PASS  item-tracked-lambda-2025-12-01           item-tracked item: aws-sdk-go-v2/lambda/release-2025-12-01
PASS  item-new-lambda-2025-12-01               item-new     item: aws-sdk-go-v2/lambda/release-2025-12-01
PASS  item-tracked-lambda-2025-12-02           item-tracked item: aws-sdk-go-v2/lambda/release-2025-12-02
PASS  item-tracked-lambdacore-2026-06-22       item-tracked item: aws-sdk-go-v2/lambdacore/release-2026-06-22
PASS  item-new-lambdacore-2026-06-22           item-new     item: aws-sdk-go-v2/lambdacore/release-2026-06-22
PASS  item-tracked-lambdamicrovms-2026-06-22   item-tracked item: aws-sdk-go-v2/lambdamicrovms/release-2026-06-22
PASS  item-new-lambdamicrovms-2026-06-22       item-new     item: aws-sdk-go-v2/lambdamicrovms/release-2026-06-22
PASS  item-tracked-lambdaweb-2026-10-12        item-tracked item: aws-sdk-go-v2/lambdaweb/release-2026-10-12
PASS  item-new-lambdaweb-2026-10-12            item-new     item: aws-sdk-go-v2/lambdaweb/release-2026-10-12
PASS  item-tracked-mq-2026-06-17               item-tracked item: aws-sdk-go-v2/mq/release-2026-06-17
PASS  item-new-mq-2026-06-17                   item-new     item: aws-sdk-go-v2/mq/release-2026-06-17
PASS  item-tracked-mq-2026-07-14               item-tracked item: aws-sdk-go-v2/mq/release-2026-07-14
PASS  item-new-mq-2026-07-14                   item-new     item: aws-sdk-go-v2/mq/release-2026-07-14
PASS  item-tracked-networkmanager-2026-10-22   item-tracked item: aws-sdk-go-v2/networkmanager/release-2026-10-22
PASS  item-new-networkmanager-2026-10-22       item-new     item: aws-sdk-go-v2/networkmanager/release-2026-10-22
PASS  item-tracked-odb-2026-08-12              item-tracked item: aws-sdk-go-v2/odb/release-2026-08-12
PASS  item-new-odb-2026-08-12                  item-new     item: aws-sdk-go-v2/odb/release-2026-08-12
PASS  item-tracked-opensearch-2026-07-01       item-tracked item: aws-sdk-go-v2/opensearch/release-2026-07-01
PASS  item-new-opensearch-2026-07-01           item-new     item: aws-sdk-go-v2/opensearch/release-2026-07-01
PASS  item-tracked-rds-2026-03-24              item-tracked item: aws-sdk-go-v2/rds/release-2026-03-24
PASS  item-new-rds-2026-03-24                  item-new     item: aws-sdk-go-v2/rds/release-2026-03-24
PASS  item-tracked-resiliencehubv2-2026-05-28  item-tracked item: aws-sdk-go-v2/resiliencehubv2/release-2026-05-28
PASS  item-new-resiliencehubv2-2026-05-28      item-new     item: aws-sdk-go-v2/resiliencehubv2/release-2026-05-28
PASS  item-tracked-s3-2026-03-12               item-tracked item: aws-sdk-go-v2/s3/release-2026-03-12
PASS  item-new-s3-2026-03-12                   item-new     item: aws-sdk-go-v2/s3/release-2026-03-12
PASS  item-tracked-s3-2026-04-22               item-tracked item: aws-sdk-go-v2/s3/release-2026-04-22
PASS  item-new-s3-2026-04-22                   item-new     item: aws-sdk-go-v2/s3/release-2026-04-22
PASS  item-tracked-s3-2026-06-16               item-tracked item: aws-sdk-go-v2/s3/release-2026-06-16
PASS  item-new-s3-2026-06-16                   item-new     item: aws-sdk-go-v2/s3/release-2026-06-16
PASS  item-tracked-s3-2026-08-06               item-tracked item: aws-sdk-go-v2/s3/release-2026-08-06
PASS  item-new-s3-2026-08-06                   item-new     item: aws-sdk-go-v2/s3/release-2026-08-06
PASS  item-tracked-s3-2026-09-08               item-tracked item: aws-sdk-go-v2/s3/release-2026-09-08
PASS  item-new-s3-2026-09-08                   item-new     item: aws-sdk-go-v2/s3/release-2026-09-08
PASS  item-tracked-s3control-2025-12-02        item-tracked item: aws-sdk-go-v2/s3control/release-2025-12-02
PASS  item-new-s3control-2025-12-02            item-new     item: aws-sdk-go-v2/s3control/release-2025-12-02
PASS  item-tracked-s3control-2026-09-08        item-tracked item: aws-sdk-go-v2/s3control/release-2026-09-08
PASS  item-new-s3control-2026-09-08            item-new     item: aws-sdk-go-v2/s3control/release-2026-09-08
PASS  item-tracked-s3tables-2025-12-02         item-tracked item: aws-sdk-go-v2/s3tables/release-2025-12-02
PASS  item-nowork-ec2-2026-09-04               item-no-work item: aws-sdk-go-v2/ec2/release-2026-09-04
PASS  item-nowork-mediaconvert-2026-07-01      item-no-work item: aws-sdk-go-v2/mediaconvert/release-2026-07-01
PASS  item-nowork-gamelift-2026-06-18          item-no-work item: aws-sdk-go-v2/gamelift/release-2026-06-18
PASS  item-nowork-connect-2026-01-27           item-no-work item: aws-sdk-go-v2/connect/release-2026-01-27
PASS  item-nowork-cleanrooms-2026-07-27        item-no-work item: aws-sdk-go-v2/cleanrooms/release-2026-07-27
PASS  item-nowork-sagemaker-2026-03-11         item-no-work item: aws-sdk-go-v2/sagemaker/release-2026-03-11
PASS  item-nowork-bcmdataexports-2026-07-27    item-no-work item: aws-sdk-go-v2/bcmdataexports/release-2026-07-27
PASS  item-nowork-glue-2026-10-12              item-no-work item: aws-sdk-go-v2/glue/release-2026-10-12
PASS  item-nowork-cloudformation-2026-07-31-2  item-no-work item: aws-sdk-go-v2/cloudformation/release-2026-07-31.2
PASS  item-nowork-sagemaker-2026-04-10         item-no-work item: aws-sdk-go-v2/sagemaker/release-2026-04-10
PASS  item-nowork-connect-2025-12-29           item-no-work item: aws-sdk-go-v2/connect/release-2025-12-29
PASS  item-nowork-codecommit-2026-08-13        item-no-work item: aws-sdk-go-v2/codecommit/release-2026-08-13
PASS  item-nowork-glue-2026-05-14              item-no-work item: aws-sdk-go-v2/glue/release-2026-05-14
PASS  item-nowork-datazone-2026-09-30          item-no-work item: aws-sdk-go-v2/datazone/release-2026-09-30
PASS  item-nowork-sagemaker-2026-08-07         item-no-work item: aws-sdk-go-v2/sagemaker/release-2026-08-07
PASS  item-nowork-bedrock-2026-05-14           item-no-work item: aws-sdk-go-v2/bedrock/release-2026-05-14
PASS  item-nowork-bedrock-2026-05-28           item-no-work item: aws-sdk-go-v2/bedrock/release-2026-05-28
PASS  item-nowork-sagemaker-2026-05-19         item-no-work item: aws-sdk-go-v2/sagemaker/release-2026-05-19
PASS  item-nowork-ec2-2026-06-02               item-no-work item: aws-sdk-go-v2/ec2/release-2026-06-02
PASS  item-gap-docdb-2026-09-21                item-gap     item: aws-sdk-go-v2/docdb/release-2026-09-21
PASS  item-gap-batch-2026-09-30                item-gap     item: aws-sdk-go-v2/batch/release-2026-09-30
PASS  item-gap-medialive-2026-06-10            item-gap     item: aws-sdk-go-v2/medialive/release-2026-06-10
PASS  item-gap-configservice-2026-07-02        item-gap     item: aws-sdk-go-v2/configservice/release-2026-07-02
PASS  item-gap-quicksight-2026-05-29           item-gap     item: aws-sdk-go-v2/quicksight/release-2026-05-29
PASS  item-gap-kafkaconnect-2026-10-22         item-gap     item: aws-sdk-go-v2/kafkaconnect/release-2026-10-22
PASS  item-gap-rds-2026-08-27                  item-gap     item: aws-sdk-go-v2/rds/release-2026-08-27
PASS  item-gap-appstream-2026-06-19            item-gap     item: aws-sdk-go-v2/appstream/release-2026-06-19
FAIL  item-covered-arcregionswitch-2026-02-27  item-covered item: aws-sdk-go-v2/arcregionswitch/release-2026-02-27
      - internal/service/arcregionswitch/plan.go not returned
      top: 1. #48382 (related), 2. #47282 (related), 3. #47168 (related), 4. #47167 (related), 5. #45302 (related)
FAIL  item-covered-bedrockagent-2026-08-28-2   item-covered item: aws-sdk-go-v2/bedrockagent/release-2026-08-28.2
      - internal/service/bedrockagent/data_source.go not returned
      top: 1. #44060 (related), 2. #49958 (related), 3. #49888 (related), 4. #45466 (related), 5. #48744 (related)
FAIL  item-covered-cloudwatch-2026-05-01       item-covered item: aws-sdk-go-v2/cloudwatch/release-2026-05-01
      - #48548 is related, want already_tracked
      top: 1. #49472 (related), 2. #49507 (related), 3. #46511 (related), 4. #48548 (related), 5. #25548 (related)
FAIL  item-covered-cognitoidentityprovider-2026-06-01 item-covered item: aws-sdk-go-v2/cognitoidentityprovider/release-2026-06-01
      - #48670 is related, want already_tracked
      top: 1. #48227 (related), 2. #41131 (related), 3. #50066 (related), 4. #48670 (related), 5. #46809 (related)

kind               cases  pass  hit@1  hit@3  hit@5 recall category unmatch precision wrong unjudged
announcement           6     6   100%   100%   100%   100%     100%       -      100%     0        0
announcement-text      6     0    83%   100%   100%   100%       0%       -         -     0        0
dev                    4     3    75%   100%   100%   100%     100%    100%       80%     2        0
duplicate             20     0    50%    70%    80%    90%      15%       -       30%    16        0
implemented            6     6   100%   100%   100%   100%     100%       -      100%     0        0
item-covered           4     0     0%     0%    50%    50%       0%       -         -     0        0
item-gap               8     8      -      -      -      -        -    100%         -     0        0
item-new              34    34      -      -      -      -        -    100%         -     0        0
item-no-work          19    19      -      -      -      -        -    100%         -     0        0
item-tracked          38    36    95%   100%   100%   100%     100%       -       96%     2        0
new                    5     5      -      -      -      -        -    100%         -     0        0
no-work                4     4      -      -      -      -        -    100%         -     0        0
related                4     3      -      -      -      -        -       -        0%     2        0
overall              158   124    79%    88%    93%    96%      69%    100%       78%    22        0
22 case(s) with existing work were summarized as unmatched
```

What it shows:

- **Item reconciliation is mostly precise.** Of 48 confident candidates in item cases, 2 are wrong. Both are #48693, a PR that deprecates `aws_bedrockagentcore_registry` and inherits a strong match from its tracker through a followed reference. No `item-new` case gets a confident candidate before its tracker exists.
- **Duplicate bug reports are the weak spot.** In the other cases, 20 of 53 confident candidates are different work, and 16 of those are in duplicate cases. A correct answer usually shows as "related" while unrelated issues show as "already tracked".
- **"No existing work" holds for 26 of 30 gap-sample items.** The 4 misses are covered work Radar didn't flag confidently. Two are implemented: arcregionswitch's RDS execution blocks and bedrockagent's `sync_schedule`. Code search looks only for new operations, not new members. Two are tracked, and both show as "related" at rank 4: #48548 asks for CloudWatch dashboard tags, and the open PR #48670 adds the Cognito user pool replica.
- **Most untracked items need no provider work.** Of the 30, 18 need none, 8 are real gaps, and 4 are covered.
- **The remaining wrong labels trace to broad strong signals:** Terraform Registry docs URLs, followed references through a wrongly matched issue, AWS and pkg.go.dev docs URLs, a shared operation name, and a multi-feature blog post. [#3](https://github.com/YakDriver/radar/issues/3) tracks reducing them.
