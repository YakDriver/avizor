# Avizor

Avizor finds what is already known about new information, gathers supporting evidence, and prepares the next action for review. It gathers evidence and doesn't decide. A human or an AI agent reads its output and chooses what to do.

The first target is the [Terraform AWS Provider](https://github.com/hashicorp/terraform-provider-aws). See [docs/design/avizor.md](docs/design/avizor.md) for the design and roadmap.

Status: milestone 3 in progress (SDK source).

## Install

```
go install github.com/YakDriver/avizor/cmd/avizor@latest
```

Avizor needs [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) for code searches, and a GitHub token from `GITHUB_TOKEN`, `GH_TOKEN`, or `gh auth token`.

## Configure

Copy [avizor.example.hcl](avizor.example.hcl) to `./avizor.hcl` or `$XDG_CONFIG_HOME/avizor/avizor.hcl` (usually `~/.config/avizor/avizor.hcl`):

```hcl
target "tfaws" {
  repo     = "hashicorp/terraform-provider-aws"
  checkout = "~/go/src/github.com/hashicorp/terraform-provider-aws"
}
```

`checkout` is optional. Without it, Avizor searches only issues and PRs, and can't map service names from `names/data/names_data.hcl`.

State lives in `$XDG_DATA_HOME/avizor` (usually `~/.local/share/avizor`): one index per target under `index/`, `items.db` for pulled items, and source clones under `sources/`. An index from an earlier version at `<state>/<target>.db` is moved to `index/` automatically. Override it with `state_dir` in the config or `--state-dir`.

## Sync

```
avizor sync                 # incremental after the first run
avizor sync --full          # refetch everything and drop records GitHub no longer returns
avizor sync --since 90d     # index a recent slice only
```

The first full sync of the Terraform AWS Provider fetches every issue, PR, and comment, which takes a long time. A slice is enough to try Avizor: `--since 2025-11-01` took about 9 minutes and indexed about 10,100 issues and PRs and 30,000 comments.

`--since` doesn't read or move the sync checkpoints. After slice-only syncs, a plain `avizor sync` starts from the beginning.

The index stores titles, bodies, and labels for full-text search. From comments it stores only extracted signals (URLs, API operation names, Terraform names, and issue references), not comment text. Set `index { comments = "none" }` to skip comments.

## Reconcile

```
avizor candidates 45362
avizor candidates https://github.com/hashicorp/terraform-provider-aws/issues/45362
avizor candidates https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/
avizor candidates --text "FSx S3 access point support"
avizor candidates --as-of 2025-12-02 45362
avizor candidates aws-sdk-go-v2/fsx/release-2025-12-02   # a pulled item; see Pull SDK changes
```

Output is JSON by default, for agents. Use `--format text` for people. Other flags:

| Flag | Effect |
|---|---|
| `--no-fetch` | Don't fetch a URL input to read its page title. |
| `--no-code` | Don't search the checkout. |
| `--related N` | Show up to N related candidates (default 10). |
| `--as-of T` | Reconcile against the index as it was at a date (end of day, UTC) or RFC 3339 time. |

An issue that isn't in the index is fetched from GitHub. Its comment signals are unavailable in that case.

### Output

The result separates kinds of findings:

- **Evidence**: signals observed in the input, with where each came from (`input.title`, `input.body`, `input.comments`).
- **Inferences**: guesses with a basis, such as the service (`fsx`, from label `service/fsx`) and the request kind (`new_type`, `change`, or `unknown`).
- **Candidates**: existing work, grouped by category, with every signal that matched.

| Category | Meaning |
|---|---|
| `already_implemented` | A merged PR, or code or docs in the checkout, matches on a strong signal. |
| `already_tracked` | An open issue or PR matches on a strong signal. |
| `previously_considered` | A closed issue or unmerged PR matches on a strong signal. |
| `related` | Only weak signals matched. |

Categories are provisional. Check the signals before acting on them.

Strong signals are specific anchors that rarely match by coincidence:

- A non-GitHub URL, such as an announcement or API reference page.
- An AWS API operation name, such as `CreateAndAttachS3AccessPoint`. Only names that are operations in the API models count, so a field name like `StartTime` doesn't. This uses the models clone from `avizor pull`; without one, any verb-prefixed PascalCase word counts, with a warning.
- A Terraform name in both the input's title and the candidate's title, when the input asks for a new type.
- For code and docs: a type registration, a docs page, or an operation call in non-test code. Existing code is only `related` when the input asks to change a type.

Weak signals are keywords in titles, shared `service/*` labels, cross-references between the input and the candidate, GitHub URLs (often a shared SDK release commit), and Terraform names in bodies. In text output, strong signals are marked with `*`.

Signals found in more than 40 records (contributing guide links, boilerplate) are skipped and listed under `warnings`.

`summary.unmatched` is true when no candidate is better than `related`.

## Pull SDK changes

`avizor pull` turns AWS SDK for Go v2 releases into items: one per service per release. Each item records the release notes and the API model change for that service: new and removed operations, shapes, members of existing shapes, and enum values, plus members whose type or enum value changed.

```
avizor pull                          # resume after the last release pulled (first run: the last 30 days)
avizor pull --since 2025-12-01       # a specific window; doesn't move the checkpoint
avizor items --format text           # items with API changes, newest first
avizor items --status skipped --service codebuild --format text
avizor inspect aws-sdk-go-v2/fsx/release-2025-12-02 --format text
```

The model change comes from a local bare clone of [aws/api-models-aws](https://github.com/aws/api-models-aws) (about 180 MB), made on the first pull and fetched on later ones. Models are committed about an hour before each SDK release, so a release's change is the diff between the last models commit before the previous release and the last one before this release.

Items with no API change are stored as `skipped`, with a reason:

- **No API change.** The model changed only in documentation, traits, or rules. This covers protocol additions (Smithy RPC v2 CBOR), serialization traits, and constraint changes such as a new maximum length. A member retargeted to another shape of the same type, for example one string shape to another with a different length limit, is kept on the item as evidence but doesn't count as an API change.
- **Release note without a model change.** For example, "Enable schema-based (de)serialization for this service."
- **Service model removed.**

Constraint changes can matter to a provider (for example, its validation), so check skipped items before assuming they need no work.

If the models clone isn't fetched (`--no-fetch` or `fetch = false`), it may not have the models for recent releases, and their changes would look like no change. So a pull without a fetch stops at the first release published after the clone's newest commit, with a warning, and leaves the checkpoint there for a later pull with a fetch.

Pulling the 230 releases from 2025-11-30 to 2026-11-11 took about 2.5 minutes and produced 1,487 items with API changes and 2,215 skipped.

All source settings have defaults and can be changed in the config. See [avizor.example.hcl](avizor.example.hcl).

### Announcements

`avizor pull` also pulls [AWS What's New](https://aws.amazon.com/about-aws/whats-new/) announcements. With no source name it pulls every source; `avizor pull aws-whats-new --since 2025-11-25` pulls only announcements, back to a date. Announcements don't become items. Enrichment matches each item to the announcement it most likely belongs to, and `avizor inspect`, `avizor draft`, and `avizor queue` show it.

A match is an inference from text similarity between the item's release notes and the announcement, within 3 days before to 7 days after the release. Region-expansion posts are left out, and the best match must clearly beat the next. Avizor uses the matched headline for draft titles. It puts the announcement in the body only with `avizor draft --announcement`, which quotes the headline and links the post in place of the release notes, as maintainer trackers do.

Of 36 items that maintainer trackers link to an announcement, Avizor matched 26 to the linked one, 2 to a different one, and left 8 unmatched. In a random sample of 300 other items it matched 79, and 3 of those were wrong by hand check.

What's New has no documented API beyond an RSS feed of the latest 100 posts. Avizor reads the JSON directory endpoint the What's New pages use, which pages back through history; `url` in the source config overrides it.

### Relate items to the target

`avizor inspect` also relates the item to the target (`--no-enrich` skips this):

- **Service**: the provider package that uses the item's SDK module, from `names_data.hcl`. It may also be a service marked not implemented, or none.
- **Availability**: the module version the target's `go.mod` requires, at the checkout's `ref` (default `HEAD`), compared with the item's version. When the target already has it, the commit and PR that brought it in.
- **Work**, as inferences: new operations grouped by noun into a likely new resource, data source, or other operation; new members as arguments (an existing operation's input reaches the shape) or attributes (only outputs do); new enum values; type changes; removals. Each lists the provider files that call the operations involved.

`avizor candidates <item-id>` reconciles the item. Its signals differ from an issue's:

- **Strong, with the item's service label**: a new operation name, and the release commit (`https://github.com/aws/aws-sdk-go-v2/commit/...`) in a title or body. Maintainers cite the release commit in trackers. Without the label, both are weak: other services have operations of the same name, and one release covers many services.
- **Weak**: existing operations the change touches, and keywords from the release notes.
- **The bump PR** that brought the SDK version into the target is only related, with an `sdk_bump` match, however strongly it matches. It makes the change available without implementing it.
- **Code** is searched only for new operations, in the item's package.

For `aws-sdk-go-v2/fsx/release-2025-12-02`, the ONTAP tracker #45362 is already tracked, and the open PR #50917 that references it follows.

### Review the queue

```
avizor queue --format text
avizor queue --since 90d --service s3 --all --format text
```

`avizor queue` enriches and reconciles every new item from the last 30 days (`--since`) and lists the ones that need a decision, newest first, with the work Avizor inferred, the draft title, and possibly related issues and PRs. Each item gets a state:

| State | Shown | Meaning |
|---|---|---|
| `gap` | yes | The work touches code the target has, or adds a new resource, and nothing confidently covers it. |
| `pending` | yes | A gap whose SDK version the target doesn't require yet. |
| `new_service` | yes | A new SDK module for a service the target doesn't implement. |
| `covered` | no | A confident candidate (already tracked, implemented, or previously considered) matches it. |
| `no_code` | no | The work touches no code the target has and adds no new resource. |
| `unimplemented` | no | The service isn't one the target implements, and it isn't a new SDK module. |
| `no_work` | no | Avizor inferred no work, as for documentation-only model changes. |
| `resurfaced` | yes | A decided item that needs another look. See [Record decisions](#record-decisions). |
| `decided` | no | An item whose recorded decision still stands. |

`--all` shows the hidden states too, and `--state gap,covered` picks states. A `gap` is still a guess. In a hand-checked sample, the queue showed 20 items as gaps: 8 were real gaps, 8 needed no provider work, and 4 were already implemented or tracked. 2 of those 4 had the covering issue or PR in the related list. Check the related list before drafting. An item is `no_code` only when its model loaded and the code search ran, so with `--no-code`, or when either fails, nothing is hidden as `no_code`; the row's reason says why. The queue needs the target's checkout for its service map.

The queue runs 4 items at a time (`--jobs`); 30 days, about 180 items, takes about 15 seconds.

### Record decisions

```
avizor decide aws-sdk-go-v2/docdb/release-2026-09-21 created 51234
avizor decide aws-sdk-go-v2/cloudwatch/release-2026-05-01 duplicate 48548
avizor decide aws-sdk-go-v2/cleanrooms/release-2026-07-27 skipped --reason "query-time setting, no resource"
avizor decide aws-sdk-go-v2/rds/release-2026-08-27 deferred --revisit 60d
avizor decide aws-sdk-go-v2/rds/release-2026-08-27 cleared
avizor decisions --format text
```

A decision takes the item off the default queue. Avizor records who decided (`--by`, default `$USER`) and what it found at the time, and the item comes back as `resurfaced`, with the reason, when:

- its revisit date passes (`--revisit`, an age or a date; `deferred` defaults to 30 days);
- it's a `duplicate` and the covering issue was closed other than as completed, or the covering PR was closed unmerged;
- it's `skipped` or `deferred` and what Avizor finds changed, such as the SDK version becoming available or a new tracking issue.

`created` stands otherwise. Decided items published before `--since` are checked too, so a deferral comes back when it's due; they're listed only if they re-surface or `--state decided` asks for them. `--state decided` lists decided items, and `avizor decisions --history` lists every decision, including cleared ones.

### Draft an issue

```
avizor draft aws-sdk-go-v2/kinesis/release-2026-08-31 --format text
avizor draft aws-sdk-go-v2/kinesis/release-2026-08-31 --target=tfaws --body | gh issue create --repo hashicorp/terraform-provider-aws --title ... --label ... --body-file -
```

`avizor draft` renders a new issue for an item in the format of the provider's tracking issues: the release notes quoted in full, then `Requires AWS SDK for Go v2 [Release ...](<release commit>): <bump PR>.` The body holds only what the item shows, so it can be filed as is. Avizor prints the `gh` command, with the flags needed to regenerate the same body, but doesn't run it.

Everything Avizor guessed is listed for the reviewer, with its basis, in a reviewer section (text) or under `inferred` (JSON), not in the body:

- **Title.** ``New resource: `aws_<package>_<noun>` `` for one inferred new resource, `New data source: ...` for one new read-only noun, `r/<resource>: <first sentence>` when new arguments affect operations only one resource calls, and otherwise `<service>: <first sentence>`.
- **Labels.** `enhancement`, the service label, `new-resource`, and `new-service` for a new SDK module or a service the target doesn't implement.
- **Headline.** The first sentence of the release notes, used in titles. If there's an announcement, add its headline and link to the body.

The reviewer section also has:

- **Warnings**: existing work Avizor matched with a confident category (`already tracked`, `already implemented`), a Terraform name that already has docs, an SDK version the target doesn't have yet, and skipped items. A draft with an "already tracked" warning probably shouldn't be filed.
- **Possibly related** issues and PRs: weak matches in the same service that share title keywords, for a `Relates` line if they fit. Bug reports and the SDK bump PR are left out.

A target can replace the body template with `draft_template`. See [avizor.example.hcl](avizor.example.hcl).

## Evaluate

`avizor eval` measures reconciliation against a labeled corpus of provider history. See [eval/README.md](eval/README.md) for the corpus, metrics, and current baseline.

## Develop

`make ci` runs every check CI runs: build, gofmt, `go fix`, `go mod tidy`, vet, and tests. Run `make help` for the individual targets. Tests need ripgrep.

To also test service-map parsing against the real provider file:

```
AVIZOR_NAMES_DATA=~/go/src/github.com/hashicorp/terraform-provider-aws/names/data/names_data.hcl go test ./internal/services
```

## License

[MPL-2.0](LICENSE)
