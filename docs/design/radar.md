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
4. **Queue and decisions.** Record dispositions, revisit deferred items, and add the announcement feed as a secondary source.
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
- **Service mapping**: SDK service ID → provider package and `service/*` labels, from the provider's `names/data/names_data.hcl`.
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

Renders a new issue, or a comment on an existing one, from the item's findings using a per-target template. Output is the Markdown body plus `gh issue create` arguments (title, labels). For the Terraform AWS Provider, the format follows #45362:

- Title: `r/aws_<service>_<thing>: <capability>` or `r/aws_<resource>: Add <argument> argument`.
- Quoted announcement headline, when one was found.
- Links to the announcement and blog post.
- Required SDK release and bump PR.
- `Relates #...` for related candidates.
- Suggested labels.

Inferred values, such as the Terraform name, are marked in the draft for the reviewer to confirm.

### Disposition

The recorded outcome for an item.

- `created #N`
- `duplicate-of #N`
- `commented #N`
- `skipped` (with reason)
- `deferred`

Each disposition stores:

- `decided_at`, `decided_by` (human or agent identity).
- `revisit_after`: optional date to re-surface the item.
- `evidence_fingerprint`: a hash of the findings that informed the decision. If later findings differ (the provider picks up the SDK version, the matched issue is closed, an announcement appears), the item re-surfaces.
- `last_evaluated_at`.

Dispositions are never treated as permanent. Over time they also become labeled data for the evaluation corpus.

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

## Open questions

- **Item granularity**: is one item per service per SDK release right, or do multi-feature releases need splitting?
- **Comment indexing**: is extracting URLs and identifiers enough, or does full comment text improve recall enough to justify the cost?
- **Agent evaluation contract**: what input and output format should the stage-2 agent use so its justifications are checkable?
- **Announcement-only changes**: how often does provider work come from announcements with no SDK model change? The corpus should answer whether the What's New source is worth adding.
- **Checkout branch**: code matches reflect whatever branch the checkout is on. Should Radar require or verify `main`?
- **Common-signal threshold**: 40 records is a guess from one dataset. The corpus can now test changes to it.
- **Corpus growth**: dispositions recorded in milestone 4 are natural new cases. How should they be added without tuning labels to Radar's output?
- **Bootstrapping dispositions**: should past provider issues that link SDK releases or announcements be imported as historical dispositions?
