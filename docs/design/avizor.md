# Avizor design

Status: milestones 1–4 done. The next step is the [agent contract](#next).

The first part of this document describes how Avizor works now. [Decisions and measurements](#decisions-and-measurements) records what each milestone changed from the plan and why. Avizor was called Radar until 2026-11-13.

## Problem

AWS ships API changes every weekday. Many of them should become a Terraform AWS Provider issue, like [#45362](https://github.com/hashicorp/terraform-provider-aws/issues/45362). Filing one by hand takes these steps:

1. Notice the change and work out which service and API it touches.
2. Find the SDK release that added it and the provider PR that bumped the SDK.
3. Decide whether the work is a new resource or a new argument, and name it.
4. Search issues, PRs, and code for existing work.
5. Write the issue, link the announcement, and label it.

Step 4 is the hard one. Community issues rarely use AWS's wording, so a deterministic check misses real duplicates. A person or agent reading a short candidate list catches them.

Volume is the other problem. A year of SDK releases has 1,487 items with API changes, about 130 a month, and only a small share needs an issue. Choosing which items need one matters more than writing any single issue well.

## Goal

Avizor gathers evidence and drafts, and a person or agent decides. A good result reads like this:

> AWS added `CreateAndAttachS3AccessPoint`, `DescribeS3AccessPointAttachments`, and `DetachAndDeleteS3AccessPoint` to FSx in aws-sdk-go-v2 release 2025-12-02. The provider picked up that release in #45358. Issue #45362 tracks this. `internal/service/fsx` already calls `CreateAndAttachS3AccessPoint`. No new issue appears necessary.

A wrong confident claim is worse than a weak one. When Avizor isn't sure, it says "related" and shows its evidence.

Non-goals:

- Deciding duplicates on its own.
- Writing to GitHub. Avizor prints `gh` commands for someone else to run.
- A general ingestion framework. Interfaces allow other sources and targets, but only AWS and the Terraform AWS Provider are built.

## Milestones

1. **Index and reconcile.** Find existing work for an issue, URL, or text. Done.
2. **Evaluate.** Build a labeled corpus from provider history and measure before tuning. Done.
3. **SDK items and drafts.** Turn SDK releases into items, relate them to the provider, and draft issues. Done.
4. **Queue and decisions.** Queue the items that need a decision, record decisions, and match announcements. Also measure the gap claim and test an agent on duplicates. Done.
5. **Generalize.** Add other sources or targets when a real use case needs them.

## How it works

```
 sources ──> items ──> enrich ──> reconcile ──> queue ──> draft
 (SDK, What's New)    (service,    (candidates)    │
                       availability,    ^          v
                       work, announcement)  │       decisions
                                          index
                              (issues, PRs, checkout)
```

### Findings

Avizor keeps statements of different reliability apart, so a reader can tell facts from guesses.

| Kind | Example | From |
|---|---|---|
| Evidence | Release 2025-12-02 adds FSx operation `CreateAndAttachS3AccessPoint` | Sources, enrichment |
| Inference | The new create, describe, and delete operations suggest a resource, `aws_fsx_s3_access_point_attachment` | Enrichment |
| Candidate | #45362 may track this (matched the operation name and `service/fsx`) | Reconcile |
| Decision | #45362 is the tracker | Person or agent |

Each finding records its provenance: inputs, tool version, and time.

### Sources

- **AWS SDK for Go v2 releases.** A feature can't be built until it's in the SDK, so the SDK is the trigger. The releases API gives module versions and notes. A local bare clone of `aws/api-models-aws` gives the model diff. One item is created per service per release.
- **AWS What's New.** Posts are matched to items as evidence. They don't become items.
- **Ad hoc input.** `avizor candidates` also takes an issue, a URL, or text.

### Enrichment

- **Service**: maps the item's Go module to a provider package with `names_data.hcl`.
- **Availability**: whether the checkout's `go.mod` at `ref` requires the item's version, and which first-parent commit and PR brought it in.
- **Work** (inferred): new operations are grouped by noun. Create or paired verbs mean a resource; reads only mean a data source. New members are arguments if an existing operation's input reaches them, and attributes otherwise. Tagging operations mean tagging support. Each kind lists the provider files that call its operations.
- **Announcement** (inferred): the What's New post most similar to the release notes, near the release date.

### Index

SQLite with FTS5, synced from the GitHub REST API. It holds issues and PRs with their titles, bodies, and labels. A signals table holds exact anchors (URLs, operation names, Terraform names, issue references) from titles, bodies, and comments. Comment text isn't stored. Signals and merges record when they happened, so reconciliation can replay the index as of a past time. A full sync drops records GitHub no longer returns. The checkout is searched with ripgrep and isn't copied.

### Reconcile

Candidates come from the index and the checkout. Each one gets a category instead of a score, because existing code, an open request, and a closed one mean different things.

| Category | Meaning |
|---|---|
| `already_implemented` | A merged PR, code, or docs matches a strong signal. |
| `already_tracked` | An open issue or PR matches a strong signal. |
| `previously_considered` | A closed issue or unmerged PR matches a strong signal. |
| `related` | Only weak signals match. |

Only strong signals set a category. The README lists the signals. A PR that references a strongly matched issue inherits the match. Signals found in more than 40 records are dropped as boilerplate.

Items use their own signals. New operation names and the release commit are strong only with the item's service label. The SDK bump PR is always `related`. Code search covers only new operations in the item's package.

A second, agent stage reads the top candidates and judges them. Avizor doesn't run that stage. [Step 5](#step-5-agent-experiment) measured it by hand.

### Queue

The queue enriches and reconciles recent items with API changes (not skipped ones) and gives each one a state. `gap`, `pending`, `new_service`, and `resurfaced` items are shown. `covered`, `no_code`, `unimplemented`, `no_work`, and `decided` items are hidden. An item is `no_code` only if both the model load and the code search succeeded, so a failure never hides an item.

### Decisions

A decision is `created #N`, `duplicate #N`, `skipped` (with a reason), `deferred` (by default for 30 days), or `cleared`. Each one stores who decided, when, an optional revisit date, and the evidence at that time: state, availability, confident candidates, the announcement, and whether code was searched. History is kept, and the latest decision is current.

A decided item resurfaces when:

- its revisit date passes;
- for `duplicate`: the covering issue closes as anything but completed, or the PR closes unmerged;
- for `skipped` and `deferred`: the evidence changes.

`created` otherwise stands. Evidence is compared only on what both runs measured. Code isn't compared unless both runs searched code, and the announcement isn't compared unless both runs had announcements.

### Draft

The draft is a title, labels, a body, and the `gh issue create` command. The body follows the provider's trackers and holds only facts from the item: the quoted release notes and the required SDK release with its bump PR. With `--announcement`, the reviewer accepts the matched post, and the body uses its headline and link instead of the notes.

Guesses go to the reviewer, never into the body: the title, Terraform names, labels, the headline, related issues for a `Relates` line, and warnings. When existing work covers the item, the draft warns. It doesn't draft a comment on the existing issue.

## Evaluation

[`eval/tfaws.hcl`](../../eval/tfaws.hcl) has 158 cases labeled from provider history, and `avizor eval` scores them. Every confident candidate is judged `same` or `different`, so the eval reports the precision of confident categories. [eval/README.md](../../eval/README.md) covers the case kinds, metrics, and results. Currently 124 cases pass, hit@5 is 93%, recall is 96%, and confident precision is 78%. On duplicate reports, confident precision is 30%.

## Configuration and state

The config is HCL. [avizor.example.hcl](../../avizor.example.hcl) documents every setting and its default. Targets set the repo, checkout, `ref`, label prefix, and draft template. Sources set where to read and where the first pull starts. With no source blocks, the config has both default sources.

State lives in `$XDG_DATA_HOME/avizor`. It holds one index per target, `items.db` (items, announcements, decisions, and checkpoints), and the models clone.

## Interface

Small commands, JSON by default, `--format text` for people. People and agents use the same commands.

| Command | Purpose |
|---|---|
| `avizor sync` | Refresh the index. |
| `avizor candidates <issue\|url\|item-id>` or `--text` | Find existing work. |
| `avizor pull [source]` | Pull SDK releases and announcements. |
| `avizor items`, `avizor inspect <item-id>` | List items; show one with its enrichment. |
| `avizor queue` | List items that need a decision. |
| `avizor decide`, `avizor decisions` | Record and list decisions. |
| `avizor draft <item-id>` | Draft an issue and its `gh` command. |
| `avizor eval` | Score the corpus. |

## Implementation choices

| Area | Choice |
|---|---|
| Language | Go: the provider's ecosystem, and a single binary. |
| Storage | SQLite (pure-Go `modernc.org/sqlite`) with FTS5. Full-text scores are computed in Go so a past view isn't skewed by later records. |
| Models | A bare clone of `aws/api-models-aws`, diffed offline. |
| Ranking | Signals and categories; weighted scores only where the eval supports them. |
| Extensibility | Compiled-in sources and targets, no plugins. |
| Writes | Printed `gh` commands. |

## Decisions and measurements

### Milestone 1: index and reconcile

- **Weak by default.** Terraform names in bodies, shared SDK release commits, and same-resource titles on change requests all produced false confident categories, so they're weak signals.
- **Request kind** (`new_type`, `change`, `unknown`) comes from the input's wording. Existing code for a named type counts as implemented only for `new_type`.
- **`--format text`**, because `--text` is the free-text input.

### Milestone 2: evaluation

- **Point in time.** Without it, labels leak: a duplicate's original already looks closed. The index records when signals and merges happened (schema v2), and `--as-of` replays it.
- **Sync.** GitHub's comments endpoint stops paging after 300 pages, which cut the first sync off at 30,000 comments. The sync now restarts from the last update time.
- **Ranking.** FTS5's `bm25()` uses whole-table statistics, which reorders past views, so scoring moved to Go.
- **Following references** fixed all 4 implemented-PR misses.
- **Precision labels** ([#3](https://github.com/YakDriver/avizor/issues/3)): confident precision was 56% at first. Documentation URLs and a field name misread as an operation caused most errors.

### Milestone 3: SDK items and drafts

- **Models from `aws/api-models-aws`, not the SDK repo.** The SDK repo is 1.6 GB, and a year of model files would take tens of thousands of downloads. The models repo is 180 MB and gets a commit an hour before each release. A release's change is the diff between the last models commit before the previous release and the last one before this release.
- **Skipping uses the model diff, not note text.** Trait-only changes, like CBOR or length limits, are skipped without a phrase list.
- **Operation names come from the models.** 15,364 names, cached next to the clone. This removed the 6 wrong labels caused by `StartTime` and lost nothing.
- **Service mapping is by Go module.** `sdk.id` doesn't always match the model's, for example `ODB` vs `odb`.
- **Availability comes from checkout history.** The first first-parent commit whose `go.mod` has the version. All 1,086 available items found one with a PR.
- **The body holds only facts**, so it can be filed as is. `Relates` lines are suggestions, and existing work is a warning, not a comment draft.
- **Drafts against 39 tracker pairs:**
  - The service label matched all 39.
  - `new-resource` agreed in 9, was drafted without the tracker having it in 8 (several are new resources the tracker didn't label), and was on the tracker only in 1.
  - Of 5 trackers that name a type, 3 drafts named the same one.
  - `new-data-source` was dropped: 14 drafted, 0 on trackers.

### Milestone 4: queue, decisions, and announcements

After milestone 3, measuring came first. Every eval item with a tracker was tracked, so nothing yet measured the claim behind a gap draft: that nothing covers the item.

#### Step 1: gap sample

30 untracked items in implemented services were checked by hand.

- **"No existing work" was right for 26 of 30.** 2 misses were implemented through new members, which code search doesn't cover. 2 were tracked at rank 4.
- **8 needed an issue and 18 needed no work.** Most of the no-work items touch APIs the provider doesn't call.
- **10 of the 18 no-work items have no inferred work in provider code and no new resource. No gap or covered item lacks both.** That became the `no_code` filter.

The 30 items are now eval cases.

#### Step 2: queue

- **The states agree with the labels:**
  - all 38 tracked items are `covered`;
  - all 8 gaps are `gap`;
  - 10 of 18 no-work items are `no_code`;
  - all 4 covered misses are `gap`, but 2 of them have the covering work in the related list.
- **Related lists include PRs**, because the sample's Cognito PR was missed when only issues were listed. The bump PR is left out.
- **Unimplemented services are hidden unless the item is a new SDK module.** Most are services the provider chose not to implement.
- **Over all 1,487 items:** 638 gap, 59 pending, 19 new service, 99 covered, 374 no code, 272 unimplemented, 26 no work.

#### Step 3: decisions

- **Evidence is stored readably, not hashed**, so the queue can say what changed, such as "availability pending → available".
- **Resurfacing depends on the decision.** One fingerprint would resurface every `created` item once its new issue was indexed.
- **Decisions aren't limited to the queue window.** A 30-day deferral of a 20-day-old item still comes back.
- **`commented` was dropped**, and `cleared` was added.

#### Step 4: announcements

- **Endpoint.** The RSS feed has 100 posts, about 9 days, too few to backfill. The undocumented JSON endpoint behind the What's New pages pages through history: 2,822 posts since 2025-11-20 in 5 seconds. If it breaks, only matching suffers, and `url` can point elsewhere.
- **Matching.** TF-IDF cosine between the release notes plus the service name (weighted ×2) and each post's headline (×2) plus body. Posts from 3 days before to 7 days after the release count, and those more than 1.5 days away score ×0.75. Region-expansion posts are excluded, because they reuse earlier launches' words. A match needs a score of 0.4 and 1.3× the runner-up. Every word weighs at least 0.01, so a one-post corpus still matches.
- **Tuned on** the 36 items whose trackers link a post: 26 right, 2 wrong, 8 unmatched. The wrong ones are a CloudFormation release with two same-day launches and a multi-feature S3 Control release.
- **Checked on** 300 random untracked items: 79 matched, and 3 were wrong.
- **Coverage.** 30% of `gap` items and 53% of `covered` items have a match.
- **The match titles drafts but stays out of the body** unless the reviewer passes `--announcement`.
- **Not in reconciliation yet.** Trackers link announcements, so the matched URL could find the trackers that `item-covered` misses. A wrong match combined with the service label would be confidently wrong, so that needs its own eval run.

#### Step 5: agent experiment

The question was whether an agent reading the candidate list catches what the categories get wrong. That assumption is why "related" is an acceptable answer.

- **Setup.** The 20 `duplicate` cases. Each agent got the input issue and the top 10 candidates as of the case's time, with titles, labels, dates, and bodies trimmed to 2,500 and 1,200 characters. It named the duplicate or none, and marked every candidate `same`, `different`, or `unsure`. It could use only that packet. One run hid Avizor's categories; the other showed them as hints.
- **Labels.** The agents marked 8 unlabeled candidates `same`. I read them, and all 8 are the same work (fixing PRs and second reports), so they're now labeled: dup-41782 #41725, dup-42605 #42536, dup-45838 #44857, dup-46606 #46285, dup-48354 #48344 and #48345, dup-49183 #43621, dup-49894 #48785. Labeling after seeing the answers could bias the result.
- **Blind run:**
  - It picked a correct duplicate in 19 of 20 cases. Avizor's top candidate is correct in 11.
  - The 20th case has no correct candidate in the top 10, and the agent said none.
  - All 29 of its `same` verdicts were right.
  - It marked all 16 of Avizor's wrong confident candidates `different`.
- **Run with categories:** the same answers, but it left one case out of its output.
- **Meaning.** With an agent reading the list, wrong categories on duplicates stop mattering, and retrieval recall becomes the limit. [#3](https://github.com/YakDriver/avizor/issues/3) still matters for people who read only the categories.
- **Limits.** 20 cases, one model, one run, on public issues the model may have seen in training.

### Next

The agent contract. Avizor would write the step 5 packet itself (input, candidates, evidence, and the verdict format), and `avizor eval` would score a file of agent verdicts. Agent setups could then be compared on the same cases, as retrieval changes are.

## Open questions

- **Agent contract.** The step 5 packet worked on duplicates. Should it carry Avizor's evidence and ask for justifications that cite it?
- **Announcement-only work.** How often does provider work come from a post with no SDK change? Those posts aren't queued.
- **Announcements in reconciliation.** Would the matched URL find trackers that cite only the announcement, without wrong confident matches?
- **New members in code.** Searching the files that call an item's operations for its new members' names would catch 2 of the 4 `item-covered` misses. Generic names like `tags` risk false matches.
- **Item granularity.** One item per service per release merges unrelated features. Both wrong tracker matches in step 4 came from such releases.
- **Decisions as eval cases.** Which decisions make good labels without tuning the corpus to Avizor's output?
- **Constraint changes.** Should skipped length, range, and pattern changes get their own lower-priority state?
- **Common-signal threshold.** 40 records is a guess from one dataset.
- **Comments.** Would indexing full comment text improve recall enough to justify its cost?
