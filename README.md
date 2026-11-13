# Avizor

Avizor turns upstream changes into evidence for a decision. For each new AWS SDK change, it finds existing issues, PRs, and code that cover the change, then queues the ones that need a decision and drafts the issues for real gaps. It doesn't decide or write to GitHub. A person or an AI agent does.

The first target is the [Terraform AWS Provider](https://github.com/hashicorp/terraform-provider-aws). The design, decisions, and measurements are in [docs/design/avizor.md](docs/design/avizor.md).

Status: pre-alpha. Milestones 1–4 are done: index and reconcile, evaluation, SDK items and drafts, and the queue, decisions, and announcements.

## Install

```
go install github.com/YakDriver/avizor/cmd/avizor@latest
```

Avizor needs `git`, [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`), and a GitHub token from `GITHUB_TOKEN`, `GH_TOKEN`, or `gh auth token`.

## Configure

Copy [avizor.example.hcl](avizor.example.hcl) to `./avizor.hcl` or `$XDG_CONFIG_HOME/avizor/avizor.hcl` (usually `~/.config/avizor/avizor.hcl`):

```hcl
target "tfaws" {
  repo     = "hashicorp/terraform-provider-aws"
  checkout = "~/go/src/github.com/hashicorp/terraform-provider-aws"
  ref      = "main"
}
```

Avizor searches the checkout for code and reads its service map (`names/data/names_data.hcl`) and SDK versions (`go.mod` at `ref`, default `HEAD`). Without a checkout, `avizor candidates` searches only issues and PRs, and `avizor queue` doesn't run.

State lives in `~/.local/share/avizor` (`$XDG_DATA_HOME/avizor`), or in `state_dir` or `--state-dir`. Every setting has a default. See the example config.

## Use

```
avizor sync --since 2025-11-01                    # index issues and PRs
avizor pull                                       # pull SDK releases and What's New announcements
avizor queue --format text                        # items that need a decision
avizor draft <item-id> --format text              # a new issue for a gap
avizor decide <item-id> created 51234             # record what you did
```

Output is JSON by default, for agents. Add `--format text` for people.

## Sync

```
avizor sync                 # incremental after the first run
avizor sync --full          # refetch everything and drop records GitHub no longer returns
avizor sync --since 90d     # only records updated in the last 90 days
```

A full first sync of the provider takes a long time. A slice is enough to start: `--since 2025-11-01` took about 9 minutes for about 10,100 issues and PRs and 30,000 comments. `--since` doesn't move the checkpoints, so a later plain `avizor sync` starts from the beginning.

The index stores titles, bodies, and labels. From comments it keeps only signals (URLs, operation names, Terraform names, and issue references), not text. `index { comments = "none" }` skips comments.

## Find existing work

```
avizor candidates 45362
avizor candidates https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/
avizor candidates --text "FSx S3 access point support"
avizor candidates aws-sdk-go-v2/fsx/release-2025-12-02
avizor candidates --as-of 2025-12-02 45362
```

The input can be an issue number or URL, another URL, text, or a pulled item. An issue missing from the index is fetched from GitHub, without its comment signals.

| Flag | Effect |
|---|---|
| `--no-fetch` | Don't fetch a URL input's page title. |
| `--no-code` | Don't search the checkout. |
| `--related N` | Show up to N related candidates (default 10). |
| `--as-of T` | Use the index as it was at a date (end of day, UTC) or RFC 3339 time. |

The output keeps three kinds of findings apart:

- **Evidence**: signals in the input, with where each was found.
- **Inferences**: guesses with a basis, such as the service and whether the input asks for a new type or a change.
- **Candidates**: existing work, by category, with the signals that matched.

| Category | Meaning |
|---|---|
| `already_implemented` | A merged PR, or code or docs in the checkout, matches a strong signal. |
| `already_tracked` | An open issue or PR matches a strong signal. |
| `previously_considered` | A closed issue or unmerged PR matches a strong signal. |
| `related` | Only weak signals match. |

Strong signals rarely match by chance:

- A non-GitHub URL, such as an announcement.
- An AWS API operation name. After a first `avizor pull`, only names defined in the API models count, so `StartTime` doesn't. Before that, any verb-prefixed PascalCase word counts, with a warning.
- A Terraform name in both titles, when the input asks for a new type.
- In code: a type registration, a docs page, or an operation call. These count only when the input asks for a new type. For a change to an existing type, code is `related`.

Weak signals are title keywords, `service/*` labels, cross-references, GitHub URLs, and Terraform names in bodies. Text output marks strong signals with `*`. Signals found in more than 40 records, like contributing-guide links, are skipped and listed under `warnings`.

Categories are often wrong, so check the signals. On the [eval](eval/README.md) corpus, 78% of confident categories are right. That drops to 30% on duplicate reports, where Avizor finds the duplicate but often marks other work as tracked. In an [experiment](docs/design/avizor.md#step-5-agent-experiment), an agent reading the top 10 candidates found the duplicate in 19 of 20 cases and rejected every wrong confident category.

For an item, the signals are different. A new operation name and the SDK release commit are strong only with the item's service label. The SDK bump PR is always `related`, because it makes the change available but doesn't implement it. Code search covers only new operations in the item's package. An issue or PR that matches only operations the code already calls is `related` too (`existing_code`), since it's usually a bug or enhancement for that code. Titles that ask for or add a new resource or service keep their category.

## Pull

```
avizor pull                                # every source, resuming where the last pull stopped
avizor pull aws-sdk-go-v2 --since 2025-12-01
avizor items --format text                 # the 50 newest items with API changes (--limit 0 for all)
avizor items --status skipped --service codebuild --format text
avizor inspect aws-sdk-go-v2/fsx/release-2025-12-02 --format text
```

The first pull covers the last 30 days. `--since` pulls an earlier window without moving the checkpoint.

### SDK releases

Each AWS SDK for Go v2 release becomes one item per changed service. The item holds the release notes and the API model change: new and removed operations, shapes, members, and enum values, and changed member types. The change is a diff of a local bare clone of [aws/api-models-aws](https://github.com/aws/api-models-aws) (about 180 MB), which gets a commit about an hour before each release.

An item without an API change is `skipped`, with one of these reasons: only docs, traits, or rules changed; a release note had no model change; or the model was removed. Constraint changes, like a new maximum length, are skipped too. They can still matter for provider validation.

With `--no-fetch`, the pull stops at the first release newer than the clone, since its change would look empty.

Pulling a year of releases (230, from 2025-11-30) took 2.5 minutes. It produced 1,487 items and 2,215 skipped.

### Announcements

The `aws-whats-new` source pulls [What's New](https://aws.amazon.com/about-aws/whats-new/) posts. Each pull starts a week before the previous one, to catch late posts. The posts aren't items. Avizor matches each item to the post that most likely announced it, by text similarity, within 3 days before and 7 days after the release. Region-expansion posts are left out, and the best post has to clearly beat the runner-up.

A match is a guess. Of 36 items whose trackers link a post, Avizor matched 26 right, 2 wrong, and 8 not at all. Of 79 matches in a random sample of 300 items, 3 were wrong.

The source reads the JSON endpoint behind the What's New pages. That endpoint isn't documented. The documented RSS feed holds only the latest 100 posts. To read a different endpoint, set `url` in the source config.

## Inspect

`avizor inspect` shows an item and relates it to the target (`--no-enrich` skips that):

- **Service**: the provider package that uses the SDK module, or a service the provider doesn't implement.
- **Availability**: whether the target's `go.mod` requires the item's SDK version, and the commit and PR that brought it in.
- **Work** (inferred): new operations grouped into a likely resource, data source, or other operation; new members as arguments or attributes; enum values; type changes; removals. Each kind lists the provider files that call the operations involved, and for a new argument, whether those files use its likely Terraform name.
- **Announcement** (inferred): the matched post, with its score and the runner-up's.

## Queue

```
avizor queue --format text
avizor queue --since 90d --service s3 --all --format text
```

`avizor queue` enriches and reconciles the items with API changes from the last 30 days and lists the ones that need a decision. Skipped items, including constraint changes, aren't queued even with `--all`; list them with `avizor items --status skipped`. It runs 4 items at a time (`--jobs`), and 30 days takes about 15 seconds. Each entry has the inferred work, a draft title, the announcement, and up to 3 related issues and PRs.

| State | Shown | Meaning |
|---|---|---|
| `resurfaced` | yes | A decision needs another look. |
| `gap` | yes | The work touches code the target has, or adds a resource, and nothing confidently covers it. |
| `pending` | yes | A gap whose SDK version the target doesn't have yet. |
| `new_service` | yes | A new SDK module for a service the target doesn't implement. |
| `partial` | yes | Covered, but some new resource or argument has no code or candidate. |
| `decided` | no | A decision stands. |
| `covered` | no | A confident candidate covers it. |
| `no_code` | no | The work touches no code the target has and adds no resource. |
| `unimplemented` | no | The target doesn't implement the service. |
| `no_work` | no | No work inferred, as for documentation-only changes. |

`--all` shows every state, and `--state gap,covered` picks some. An item counts as `no_code` only if the code search ran. With `--no-code`, nothing is hidden as `no_code` or shown as `partial`.

For `partial`, a new argument counts as present when the files calling its operations use its likely Terraform name. `avizor inspect` shows where. When a confident issue cites the item's release commit, the item stays `covered`, since that issue tracks the whole launch. Of the 13 `partial` items in a year of releases, 12 had real uncovered work.

A `gap` is a guess. In a hand-checked sample of 20 gaps, 8 were real gaps, 8 needed no provider work, and 4 were already covered. 2 of those 4 had the covering issue or PR in the related list, so check that list before drafting.

## Decide

```
avizor decide aws-sdk-go-v2/docdb/release-2026-09-21 created 51234
avizor decide aws-sdk-go-v2/cloudwatch/release-2026-05-01 duplicate 48548
avizor decide aws-sdk-go-v2/cleanrooms/release-2026-07-27 skipped --reason "query-time setting, no resource"
avizor decide aws-sdk-go-v2/rds/release-2026-08-27 deferred --revisit 60d
avizor decide aws-sdk-go-v2/rds/release-2026-08-27 cleared
avizor decisions --format text
```

A decision hides the item from the queue. Avizor records who decided (`--by`, default `$USER`) and what it found at the time. The item comes back as `resurfaced`, with the reason, when:

- the revisit date passes (`--revisit`, an age or date; `deferred` defaults to 30 days);
- for `duplicate`: the covering issue closes as anything but completed, or the PR closes unmerged;
- for `skipped` or `deferred`: the evidence changes, for example the SDK version becomes available, a tracker appears, or the announcement match changes.

Otherwise `created` stands. The queue also checks decided items older than `--since`. `avizor decisions --history` lists every decision, including cleared ones.

## Draft

```
avizor draft aws-sdk-go-v2/kinesis/release-2026-08-31 --format text
avizor draft aws-sdk-go-v2/kinesis/release-2026-08-31 --body | gh issue create --repo hashicorp/terraform-provider-aws --title ... --label ... --body-file -
```

`avizor draft` renders an issue in the format of the provider's trackers. The body holds only facts from the item, so it can be filed as is: the release notes, quoted, then `Requires AWS SDK for Go v2 [release](commit): <bump PR>.` With `--announcement`, the body has the matched post's headline and link in place of the notes. Avizor prints the `gh` command but doesn't run it.

Guesses go in a reviewer section (text) or under `inferred` (JSON), each with its basis, never in the body:

- **Title**: one of
  - ``New resource: `aws_<package>_<noun>` ``;
  - `New data source: ...`;
  - `r/<resource>: ...`, when the new arguments reach only one resource's operations;
  - otherwise the announcement headline, or the first sentence of the notes, with a service prefix if it lacks one.
- **Labels**: `enhancement` and the service label, plus `new-resource` for an inferred resource and `new-service` for a new SDK module or a service the target doesn't implement. For such a service, the service label is a guess from the module name.

The reviewer section also lists warnings and possibly related issues and PRs:

- **Warnings**: confident existing work, an existing docs page for the Terraform name, an SDK version the target lacks, a skipped item, and `--announcement` without a match. A draft warned "already tracked" probably shouldn't be filed.
- **Possibly related issues and PRs**: weak matches for a `Relates` line. Bug reports and the bump PR are left out.

`draft_template` replaces the body template. See the example config.

## Evaluate

`avizor eval` scores reconciliation and the queue against 158 cases labeled from provider history. See [eval/README.md](eval/README.md).

## Develop

`make ci` runs what CI runs: build, gofmt, `go fix`, `go mod tidy`, vet, and tests. Tests need ripgrep. `make help` lists the targets.

To also test parsing the real service map:

```
AVIZOR_NAMES_DATA=~/go/src/github.com/hashicorp/terraform-provider-aws/names/data/names_data.hcl go test ./internal/services
```

## License

[MPL-2.0](LICENSE)
