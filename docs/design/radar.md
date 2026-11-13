# Radar: design notes

Status: milestones 1 (index and reconcile) and 2 (evaluation) implemented. See [Milestone 1 notes](#milestone-1-notes) and [Milestone 2 notes](#milestone-2-notes).

## Problem

AWS ships new APIs every weekday. For the Terraform AWS Provider, many of those changes should become a GitHub issue, such as [hashicorp/terraform-provider-aws#45362](https://github.com/hashicorp/terraform-provider-aws/issues/45362). Opening a good one by hand takes several steps:

1. Notice the change and work out which service and API it touches.
2. Find the AWS SDK release that added the API, and the provider PR that bumps the SDK.
3. Decide whether the work is a new resource or a new argument on an existing one, and name it.
4. Search open and closed issues, PRs, and the code itself to see whether it's already tracked or built.
5. Write the issue, link the announcement and blog post, and apply labels.

Step 4 is the hardest. Community issues rarely use AWS's wording, so a deterministic duplicate check misses real matches, while a human or AI agent reading a short list of candidates catches them easily.

## Goal

Radar finds what is already known about new information, gathers supporting evidence, and prepares the next action for review.

Radar gathers evidence and drafts. It does not decide. A good result reads like:

> AWS added `CreateAndAttachS3AccessPoint`, `DescribeS3AccessPointAttachments`, and `DetachAndDeleteS3AccessPoint` to FSx in aws-sdk-go-v2 release 2025-12-02. The provider picked up that release in #45358. Issue #45362 tracks this capability. `internal/service/fsx` already calls `CreateAndAttachS3AccessPoint`. No new issue appears necessary.

That explains the situation and makes the next action obvious, while leaving the decision to the human or supervising agent.

### Non-goals

- Deciding by itself whether something is a duplicate.
- Writing to GitHub. Radar produces the issue content and `gh` command arguments, and a human or agent runs them.
- A general-purpose ingestion framework up front. Radar keeps interfaces that allow other sources and targets, but only AWS and the Terraform AWS Provider are built until a second use case needs more.

## Milestones

Reconciliation is the hardest and most valuable part, and it's useful on its own, so it comes first.

1. **Index and reconcile.** Given an issue number, URL, or free text, find related issues, PRs, and existing code. This is useful immediately for triaging incoming community issues.
2. **Evaluate.** Build the evaluation corpus (see [Evaluation](#evaluation)) and measure reconciliation before tuning anything.
3. **SDK source and draft.** Turn aws-sdk-go-v2 changes into items, enrich them, and draft issues.
4. **Queue and decisions.** List the items that need a decision, record dispositions, revisit deferred items, and add the announcement feed as a secondary source. See [Milestone 4 plan](#milestone-4-plan).
5. **Generalize.** Add other sources or targets only when a concrete use case needs them.

## Concepts

```
 source ──> item ──> enrichers ──> reconcile ──> draft
                      (evidence,     (candidates)    │
                       inference)        ^           v
                                         │       disposition
                                       index
                              (issues, PRs, code, docs)
```

### Findings: evidence, inference, candidate, disposition

Radar's output mixes statements of very different reliability. It keeps them apart so a reader, especially an agent, can tell observed facts from educated guesses.

| Kind | Example | Produced by |
|---|---|---|
| Evidence | aws-sdk-go-v2 release 2025-12-02 adds FSx operation `CreateAndAttachS3AccessPoint` | Sources, enrichers |
| Inference | A new `Create*`/`Delete*`/`Describe*` set suggests a new resource, likely `aws_fsx_s3_access_point_attachment` | Enrichers |
| Candidate | Issue #45362 may already track this (matched: operation name, `service/fsx` label) | Reconcile |
| Disposition | #45362 is the existing tracker | Human or agent |

Every finding records **provenance**: which component produced it, from which inputs (URLs, release tags, file paths), and when.

### Source

A source produces items. Sources are compiled-in Go types behind one interface.

**Primary: aws-sdk-go-v2 releases.** A feature isn't buildable until it's in the SDK, so the SDK is the natural trigger.

- Releases are daily, tagged `release-YYYY-MM-DD`, at about 20:00 UTC. The release body lists each changed module with a one-line **Feature** or **Bug Fix** note.
- The release notes are prose. The structured detail comes from diffing the service model (`codegen/sdk-codegen/aws-models/<service>.json`) between consecutive release tags: new operations, new shapes, new members on existing shapes, new enum values.
- `aws/api-models-aws` publishes the Smithy models about an hour before the SDK release. AWS doesn't publish anything earlier.
- The SDK is noisy. Many entries are protocol or serialization changes ("adds Smithy RPC v2 CBOR as an additional protocol", "Enable schema-based (de)serialization"). A model diff with no new operations, members, or enum values filters these out.

**Secondary: AWS What's New feed** (RSS). Some announcements need provider work without new SDK operations or members, for example a behavior change behind an existing field. The feed catches those. It's added in milestone 4.

**Ad hoc input:** an issue number, URL, or `--text` description given on the command line. This needs no source configuration and is what milestone 1 uses.

### Item

The normalized unit of work.

- `id`: stable key. For SDK items, `aws-sdk-go-v2/<service>/<release-tag>`. For feed items, the canonical URL.
- `source`, `title`, `summary`, `url`, `published`.
- `findings`: evidence and inferences attached by the source and enrichers.

One SDK release can contain several independent features for one service, and several announcements can describe one feature. Radar doesn't model "capabilities" separately yet. One item per service per release is the starting point, and the evaluation corpus will show whether that's too coarse.

### Enricher

An enricher adds findings to an item. This is where AWS and provider knowledge lives.

- **SDK model diff**: evidence of new operations, shapes, members, and enum values.
- **Service mapping**: SDK service module → provider package and `service/*` labels, from the provider's `names/data/names_data.hcl`.
- **Provider availability**: whether the provider's `go.mod` includes the SDK version, and which bump PR brought it in. This sets the item's status to "in SDK" or "in provider".
- **Announcement link**: matching What's New entries and AWS News Blog posts, joined on service and a date window around the release. AWS sometimes updates APIs well before announcing them, so the window is configurable and a missing match is recorded as "no announcement found", not as an error.
- **Shape classification**: inference only. New `Create*`/`Delete*` operations suggest a new resource; new members on existing inputs suggest a new argument. API changes don't map cleanly onto Terraform resources (a new `Create*` operation can back an existing resource, and a new member can need a standalone resource), so this is always labeled as a guess.
- **Name suggestion**: inference only. A proposed Terraform resource or argument name following provider conventions.

### Index

A local copy of what already exists in the target, so reconciliation is fast and doesn't hit API rate limits (GitHub search allows about 30 requests a minute).

- Issues and PRs, open and closed, synced with `gh api` and refreshed with `since=`. GitHub's REST issues endpoints return PRs as issues, so the sync tags each record's kind. Sync must handle pagination, edits, and deletions, so a periodic full resync supplements incremental updates.
- Titles, bodies, and labels are indexed first.
- Comments are expensive to index in full for a repository this size. People often paste announcement links in comments ("AWS just announced this: ..."), though, so at minimum Radar extracts URLs and AWS operation names from comments into a separate table.
- A local checkout of the target repo is searched directly for code and docs. It is not copied into the index.

Storage is SQLite with FTS5. Embeddings wait until the evaluation corpus shows concrete misses. AWS operation names, Terraform resource names, and service aliases are strong lexical anchors.

### Reconcile

Reconciliation runs in two stages.

1. **Retrieval (deterministic, high recall).** Search the index and the checkout using:
   - announcement and blog URLs
   - SDK operation and shape names
   - proposed Terraform names and close variants
   - `service/*` labels plus key nouns and known service aliases
2. **Evaluation (optional, agent).** An agent reads the top candidates and the item's findings, and writes a short justification that cites evidence. Radar narrows the field so the agent reasons over a handful of candidates, not the whole repository.

Candidates are grouped by **match category** instead of one weighted score, because an existing implementation, a merged PR, and an open request mean different things:

| Category | Meaning |
|---|---|
| Already implemented | Code, docs, or a merged PR covers the capability. |
| Already tracked | An open issue or PR describes substantially the same work. |
| Previously considered | A closed issue or PR addressed it, possibly with a decision worth revisiting. |
| Related | Relevant work exists but doesn't necessarily cover the item. |
| Unmatched | No convincing candidate found. |

Categories are provisional findings, not dispositions. Each candidate lists the signals it matched.

Example:

```
item: aws-sdk-go-v2/fsx/release-2025-12-02  (in provider via #45358)

already implemented
  code    internal/service/fsx/s3_access_point_attachment.go
          matched: op:CreateAndAttachS3AccessPoint
already tracked
  #45362  open  r/aws_fsx_s3_access_point_attachment: Attach to Amazon FSx for NetApp ONTAP file systems
          matched: op:CreateAndAttachS3AccessPoint, announcement_url, label:service/fsx
related
  #43195  open  ...
          matched: label:service/s3control, keyword:"access point"
```

### Draft

Renders a new issue from the item's findings using a per-target template. Output is the Markdown body plus `gh issue create` arguments (title, labels). Radar doesn't draft comments on existing issues: if existing work covers the item, the draft warns and the item is left alone. For the Terraform AWS Provider, the format follows #45362:

- Title: ``New resource: `aws_<service>_<thing>` ``, `r/aws_<resource>: <capability>`, or `<service>: <capability>`.
- Quoted announcement headline, when one was found (milestone 4), and links to the announcement and blog post.
- The release notes, quoted.
- Required SDK release and bump PR.
- Suggested labels.

Inferred values, such as the Terraform name, and possibly related issues for a `Relates` line are listed for the reviewer, not written into the body. See [Milestone 3 notes](#milestone-3-notes).

### Disposition

The recorded outcome for an item, per target.

- `created #N`: a new issue was filed.
- `duplicate #N`: existing work covers the item.
- `skipped` (with reason)
- `deferred` (revisit after 30 days unless set)
- `cleared`: withdraws the previous decision.

Each disposition stores `decided_at`, `decided_by` (human or agent identity), an optional `revisit_after`, and the evidence Radar found when the decision was made: the queue state, SDK availability, and confident candidates. Decisions are kept as history; the latest is current.

A decided item re-surfaces in the queue, with the reason, when:

- its revisit date passes (any decision);
- for `duplicate`, the covering issue is closed other than as completed, or the covering PR is closed unmerged;
- for `skipped` and `deferred`, the evidence changed: the state, availability, or confident candidates differ.

A `created` decision otherwise stands. Dispositions are never treated as permanent. Over time they also become labeled data for the evaluation corpus.

## Evaluation

Without measurement, changes to retrieval, categories, or agent prompts are guesswork. Radar ships with an evaluation corpus and a command to run it.

The corpus starts with 30 to 50 hand-labeled cases drawn from provider history:

- SDK changes or announcements with a known corresponding issue or PR.
- Features implemented without any issue.
- Related-but-not-duplicate issues.
- SDK changes that need no provider work.

Metrics:

- The correct candidate appears in the top 3 and top 5.
- Category accuracy, especially how often "related" is shown as "already tracked".
- Unmatched rate for cases that do have a match.

The same corpus compares retrieval changes, models, and agent strategies on identical inputs.

## Configuration

HCL, matching the provider's `names_data.hcl`. Config is portable. State lives separately.

```hcl
target "tfaws" {
  repo     = "hashicorp/terraform-provider-aws"
  checkout = "~/go/src/github.com/hashicorp/terraform-provider-aws"
  template = "templates/tfaws-issue.md.tmpl"

  index {
    comments = "urls-and-identifiers" # or "none", "full"
  }
}

source "aws-sdk-go-v2" {
  type = "aws-sdk-go-v2-releases"
  repo = "aws/aws-sdk-go-v2"

  ignore_notes = [
    "adds Smithy RPC v2 CBOR as an additional protocol",
    "Enable schema-based (de)serialization",
  ]
}

source "aws-whats-new" {
  type = "rss"
  url  = "https://aws.amazon.com/about-aws/whats-new/recent/feed/"
}

pipeline "aws-sdk" {
  sources = ["aws-sdk-go-v2"]
  enrichers = [
    "sdk-model-diff",
    "service-map",
    "provider-availability",
    "announcement-link",
    "shape-classification",
    "name-suggestion",
  ]
  target = "tfaws"

  announcement_link {
    window_before = "30d"
    window_after  = "14d"
  }
}
```

State (index, items, dispositions) lives in the XDG data directory (`$XDG_DATA_HOME/radar`, usually `~/.local/share/radar`), overridable with `--state-dir`.

## Interface

A CLI with small, composable commands. JSON by default for agents, `--text` for people. Humans and agents use the same commands, so behavior doesn't diverge.

| Command | Milestone | Purpose |
|---|---|---|
| `radar sync [target]` | 1 | Refresh the index. |
| `radar candidates <issue\|url\|item-id>` or `--text "..."` | 1 | Reconcile ad hoc input or an item. |
| `radar eval` | 2 | Run the evaluation corpus. |
| `radar pull [pipeline]` | 3 | Fetch and enrich new items. |
| `radar inspect <item-id\|url>` | 3 | Show an item's findings with provenance. |
| `radar draft <item-id>` | 3 | Render an issue or comment and `gh` arguments. |
| `radar queue` | 4 | List items with no disposition, or whose revisit conditions are met. |
| `radar decide <item-id> <disposition>` | 4 | Record a disposition. |

Examples:

```
radar candidates 45362
radar candidates https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/
radar candidates --text "FSx S3 access point support"
```

An MCP server wrapping the same commands is a later option.

## Implementation choices

| Area | Choice |
|---|---|
| Language | Go: same ecosystem as the provider, single binary, good GitHub and SQLite libraries. |
| Config | HCL. |
| Extensibility | Compiled-in interfaces for sources, enrichers, and targets. No dynamic plugins or external processes. |
| State | XDG data directory, overridable. |
| Model diff | Diff the Smithy JSON models in aws-sdk-go-v2 between release tags first. Consider `aws/api-models-aws` later for its earlier publish time. |
| Ranking | Matched signals and categories. Weighted scores only after the evaluation corpus provides data. |
| Writes | Delegated to `gh`. |

## Milestone 1 notes

What was built, and where it differs from the design above:

- **Commands**: `sync` and `candidates`. Output is JSON by default. The text format is `--format text`, because `--text` is the free-text input flag.
- **Index**: SQLite (pure-Go `modernc.org/sqlite`) with FTS5 over titles, bodies, and labels, plus a `signals` table of exact anchors (URLs, operation names, Terraform names, issue references) from titles, bodies, and comments. Comment text isn't stored. A full sync stamps a generation on every record and removes records not seen.
- **Signal strength**: candidates are categorized by strong signals only. Testing on real data showed that Terraform names in bodies, shared SDK release commit URLs, and same-resource titles on change requests all produce false "tracked" or "implemented" results, so they are weak.
- **Request kind**: an extra inference (`new_type`, `change`, `unknown`) from the input's wording. Existing code for a named type counts as implemented only for `new_type` requests.
- **Common signals**: anchors found in more than 40 records are skipped and reported, which removes contributing-guide links and boilerplate.
- **Service inference**: from `service/*` labels, Terraform name prefixes, and case-sensitive service names from `names_data.hcl`.
- **Not yet**: the agent evaluation stage, the evaluation corpus (milestone 2), and recorded checkout branch handling beyond reporting `branch@sha` in the result.

## Milestone 2 notes

- **Corpus and runner**: [`eval/tfaws.hcl`](../../eval/tfaws.hcl) has 55 cases labeled from repository history, and `radar eval` scores them. See [`eval/README.md`](../../eval/README.md) for the format, label sources, and baseline.
- **Point-in-time reconciliation**: without it, labels leak (a duplicate's original shows as closed, the implementing PR appears before the request). The index now records when each signal's text was posted and when PRs merged (schema v2), and `index.Store.AsOf` hides later records and rewinds closed and merged state. `radar candidates --as-of` exposes it.
- **Sync fix**: GitHub's repository comments endpoint stops returning next links after 300 pages, so the first sync silently stopped at 30,000 comments. Listing now restarts from the latest update time when a pass ends on a full page.
- **Ranking**: FTS5's `bm25()` uses statistics from the whole table, so later records could reorder a point-in-time view. Full-text matches are now scored in Go with inverse document frequencies counted over the visible records, weighted by column. Eval results barely changed.
- **Following references**: PRs that reference a strongly matched issue in their title or body are returned with that issue's match, and categorized by their own state. This fixed all 4 implemented-PR misses.
- **Precision of confident categories** ([#3](https://github.com/YakDriver/radar/issues/3)): every candidate returned above "related" is judged `same` or `different` by reading it, and `radar eval` reports precision. A confident category on different work fails the case. At 56%, confident categories are wrong almost half the time, mostly from documentation URLs and a misread field name.
- **Results**: before precision labels, 30 of 55 cases passed, hit@5 88%, recall 96%, category accuracy 47%. Categorization is still the main gap: duplicates and reworded requests are found, but as "related". The next things to measure are narrowing broad strong signals (multi-feature blog posts, shared operation names, SDK docs URLs) and an agent stage that evaluates top candidates. See [`eval/README.md`](../../eval/README.md#results).

## Milestone 3 notes

PR 1 (SDK source):

- **Model source changed.** The design called for diffing the Smithy models in aws-sdk-go-v2 between release tags. That repository is 1.6 GB, and each release changes about 30 model files of up to 900 KB, so covering a year would take tens of thousands of downloads. Radar instead keeps a local bare clone of `aws/api-models-aws` (about 180 MB), which holds only the models and gets a commit about an hour before each SDK release. The SDK releases API still supplies each module's version and release notes. The result is the same diff, offline.
- **Release windows.** A release's change is the diff between the last models commit at or before the previous release and the last one at or before this release. Some days have several models commits; all of them before the release count.
- **Items** are one per service per release, stored in `items.db` with their evidence and provenance (models commits compared, model path, release URL). `radar pull`, `radar items`, and `radar inspect` are the commands.
- **Skipping** is based on the model diff, not on release-note text. A trait-only change (a new protocol, `jsonName`, length constraints) is skipped as "no API change". This drops CBOR and serialization notes without a phrase list.
- **Configuration.** Every source setting has a default and can be overridden: releases repo, models URL, clone location, branch, fetch, start, and service filters. The state directory is configurable too.

PR 2 (enrich and reconcile items):

- **Operation names come from the models.** A verb-prefixed PascalCase word counts as an operation only if some model at the clone's tip defines it. The 15,364 names are cached next to the clone. This removed the 6 wrong labels caused by `StartTime` and lost nothing.
- **Service mapping is by Go module, not SDK ID.** `sdk.id` in `names_data.hcl` doesn't always match the model's (`ODB` vs `odb`), so the item's module is matched to `go_packages.v2_package`, or the block label when that's unset. The block label is the provider package: `provider_package_correct` is only the preferred name (`es` for `elasticsearch`), and labels and directories use the label.
- **Availability and the bump PR come from the checkout's history**, not the index: the first first-parent commit at or after the release whose `go.mod` requires the item's version. Every one of the 1,086 available items in the year's data found one, with a PR number. Bump PRs are mostly Dependabot group PRs, which the index has but which don't name every module.
- **Work kinds are inferences** from the diff and the model: new operations grouped by noun (create, or paired verbs like register and deregister, mean a resource; reads only, a data source), and new members as arguments or attributes by whether an existing operation's input reaches the shape.
- **Item reconciliation uses item-specific signals.** New operations and the release commit are strong only with the item's service label. The bump PR is demoted to related. Code search is limited to new operations in the item's package.
- **Pull records release commits** from `git ls-remote`, so items carry the commit URL maintainers cite.
- **Eval.** 73 item cases, labeled from trackers that cite release commits. Because Radar uses the same signal, they measure precision more than retrieval. Item-level no-work cases are hard to label from history: an item no tracker cites may still need work. The corpus has one.

PR 3 (draft):

- **`radar draft <item-id>`** renders the title, labels, body, and `gh issue create` arguments. Radar doesn't run `gh`.
- **The body holds only what the item shows**: the release notes quoted in full and the required SDK release with its bump PR. Inferred values (title, Terraform names, labels, headline) are listed with their basis in the JSON and the text reviewer section, not in the body, so the body can be filed as is.
- **`Relates` is a suggestion, not body text.** Related candidates are weak matches, so writing them into the issue would state a relation Radar hasn't established.
- **Existing work is a warning.** A confident candidate means the item may already be tracked or implemented. Radar fills gaps, so it doesn't draft comments on existing issues.
- **No announcement source yet.** The body quotes the release notes in full; titles use the first sentence. The reviewer can add the announcement headline, and milestone 4's announcement source will supply it.
- **Tagging operations** (`TagResource` and its family) are inferred as tagging support, not a new resource, and aren't strong signals in reconciliation: every service has them.
- **Measured against history.** Drafts for the 37 items with maintainer trackers (39 tracker pairs):
  - The drafted service label is on the tracker in all 39.
  - `new-resource`: 9 agree, 8 drafted without it on the tracker, 1 on the tracker only. Several of the 8 (ECS Managed Daemons, S3 object annotations, Lambda Core network connectors) are new resources the tracker didn't label.
  - `new-service`: 3 agree, 1 drafted only, 1 on the tracker only.
  - Of the 5 trackers that name a Terraform type, 3 drafts name the same type: `aws_kinesis_channel` (new resource), and `aws_fsx_s3_access_point_attachment` and `aws_grafana_workspace` (`r/` titles). The S3 ones get service titles, since their new members reach operations several resources call.
  - Every draft warns that its tracker already tracks the item, which is what the warning is for when drafting after the fact.
  - I dropped the `new-data-source` label: new read-only operations were mostly statuses and sub-resource listings (14 drafted, 0 on trackers).

## Milestone 4 plan

A look back after milestone 3 changed the order of milestone 4.

**What the data showed:**

- **Volume is the bottleneck.** From 2025-11-30 there are 1,487 SDK items, about 130 a month. 37 of them have a maintainer tracker that cites the release commit. Choosing which items deserve an issue matters more than the quality of any one draft, so the queue is the main interface, not bookkeeping.
- **"No existing work" is unmeasured.** Drafting history only exercises the "already tracked" warning, since every historical item with a tracker is tracked. A gap draft rests on the claim that nothing covers the item, and nothing measures that claim's precision yet.
- **Item precision is optimistic.** Item cases (2 of 48 confident candidates wrong) come from trackers that cite the release commit, which is also a strong reconcile signal. Non-item cases (20 of 53 wrong) are likely closer to real-world precision. The `item-new` cases are sound (each replays an item just before its tracker was filed), but they come from the same tracked items, so they say nothing about items no one tracked.
- **Release-note titles are weak.** "KMS GetKeyLastUsage API provides information on..." against the maintainer's "AWS KMS key last usage". Announcements improve titles and also cover trackers that cite only an announcement.
- **The AI half of the workflow is untested.** The design assumes a reader of the candidate list catches duplicates a deterministic check misses. That is why "related" is acceptable, and it hasn't been measured.

**Order:**

1. **Measure the gap claim.** Hand-check a sample of recent items in implemented services with no confident existing work: is work really missing, and does the item deserve an issue? This gives a precision for "no existing work" and the share of items worth filing, and the checked items become eval cases that don't come from the release-commit signal.
2. **`radar queue`.** List items with no confident existing work, filtered and sorted by work kind, service, and availability.
3. **Dispositions.** Record decisions on queue items, re-surface them when evidence changes, and add them to the corpus.
4. **Announcement source.** What's New entries joined to items, for headlines, links, and announcement-only trackers.
5. **Agent experiment.** Give an agent the candidates for the duplicate cases and score its picks. If it fixes most of the confidently wrong ones, tuning matters less there; if not, [#3](https://github.com/YakDriver/radar/issues/3) comes first.

Issue #3 waits for steps 1 and 5, which show whether the broad strong signals hurt in practice.

**Step 1 result** (the gap sample, 30 items; see [eval/README.md](../../eval/README.md)):

- **"No existing work" is right for 26 of 30 (87%).** The 4 misses are covered work shown as "related" or not at all: 2 implemented by new members that code search doesn't look for (it covers new operations only), and 2 tracked by an issue and an open PR at rank 4.
- **8 of 30 deserve an issue; 18 need no provider work.** Most no-work items touch APIs the provider doesn't call: jobs, runs, contacts, health checks, and resources the provider doesn't have.
- **A queue filter suggests itself.** 10 of the 18 no-work items have no inferred work in code the provider has (no `used_by`) and no new resource. All 8 gaps and all 4 covered items have one or the other. Hiding the first group by default drops more than half the noise and loses no gap in the sample.
- **Draft warnings miss PRs.** The draft's "possibly related" list includes the CloudWatch issue but not the Cognito PR, since it lists only issues. The queue should show weak PR matches too.
- **Next experiment, not yet built:** search the files that call an item's operations for its new members' snake_case names, to catch "already implemented". Generic names like `tags` or `routing` are a precision risk, so measure before trusting it.

**Step 2: `radar queue`.** It classifies each new item from the last 30 days and shows `gap`, `pending`, and `new_service` items by default. `covered`, `no_code`, `unimplemented`, and `no_work` are hidden.

- **The default hides the gap sample's no-work items with no code.** On all 1,487 items, the queue's states agree with the gap sample labels:
  - all 38 `item-tracked` items are `covered`;
  - 10 of 18 `item-no-work` items are `no_code` (hidden) and 8 are `gap`;
  - all 8 `item-gap` items are `gap`;
  - all 4 `item-covered` items are `gap`, since Radar has no confident candidate. 2 of the 4 have the covering work in the related list: #48548, and #48670 now that related lists include PRs.
- **Related lists include PRs** in the queue and in drafts. The SDK bump PR is left out, since every item in a release shares it.
- **Unimplemented services are hidden unless the item is a new SDK module.** The gap sample didn't cover them. Most are services the provider has chosen not to implement or AWS-internal APIs, and a new module is the clearest sign of a service worth a look.
- **`pending` comes after `no_code`.** An item with nothing in code the target has stays hidden even if its SDK version isn't available yet.
- **Over all items:** 638 gap, 59 pending, 19 new service, 99 covered, 374 no code, 272 unimplemented, and 26 no work. 59 of the 95 shown in the last 30 days are pending, because the provider's AWS SDK versions predate the 2026-10-29 releases.

**Step 3: dispositions.** `radar decide <item-id> <decision> [issue]` records a decision in the items database, and `radar decisions` lists them. The queue hides decided items (`decided`) until they re-surface (`resurfaced`, shown first). Deviations from the original design:

- **Evidence is stored readably, not as a hash.** A hash only says that something changed. The stored evidence lets the queue say what: "availability pending → available", "new: already tracked #5".
- **Resurfacing depends on the decision.** A single fingerprint would re-surface every `created` item as soon as its new issue was indexed, because that issue becomes a confident candidate. So `created` stands, `duplicate` watches the covering issue or PR, and only `skipped` and `deferred` compare evidence.
- **No `last_evaluated_at`.** The queue reads decisions without writing, and comparing evidence doesn't need it.
- **Evidence from runs with different code searches isn't compared on code.** Evidence records whether enrichment searched code (which `no_code` depends on) and, separately, whether reconciliation did (which code candidates depend on). Code candidates are compared only when both runs searched, and the state only when both runs did the same searches. Availability and issue or PR candidates are always compared, so `--no-code` or a failed model load doesn't re-surface everything.
- **Decisions aren't limited to the queue's window.** Decided items published before `--since` are evaluated too, and listed if they re-surface, so a 30-day deferral of a 20-day-old item still comes back.
- **`commented` is gone** (see [Draft](#draft)), and `cleared` withdraws a decision without deleting history.
- **Turning decisions into eval cases** is left for later. It needs a rule for which decisions make good labels; see Open questions.

## Open questions

- **Item granularity**: is one item per service per SDK release right, or do multi-feature releases need splitting?
- **Comment indexing**: is extracting URLs and identifiers enough, or does full comment text improve recall enough to justify the cost?
- **Agent evaluation contract**: what input and output format should the stage-2 agent use so its justifications are checkable?
- **Announcement-only changes**: how often does provider work come from announcements with no SDK model change? The corpus should answer whether the What's New source is worth adding.
- **Checkout branch**: code matches and availability reflect whatever the checkout's `ref` points to (default `HEAD`). Should Radar default to `main`, or verify it?
- **Item labels without release commits**: trackers that cite only an announcement can't be tied to an item from history alone. Joining announcements to items (the announcement-link enricher) would add them as cases.
- **Common-signal threshold**: 40 records is a guess from one dataset. The corpus can now test changes to it.
- **Corpus growth**: dispositions recorded in milestone 4 are natural new cases. How should they be added without tuning labels to Radar's output?
- **Constraint changes**: skipped SDK items include length, range, and pattern changes, which can affect provider validation. Should they be a separate, lower-priority status instead of skipped?
- **Bootstrapping dispositions**: should past provider issues that link SDK releases or announcements be imported as historical dispositions?
