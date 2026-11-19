# Avizor

Avizor (ah-bee-THOR, as in Spain: /aβiˈθoɾ/) turns upstream changes into evidence for a decision. For each new AWS SDK change, it finds existing issues, PRs, and code that cover the change, then queues the ones that need a decision and drafts the issues for real gaps. It doesn't decide or write to GitHub. A person or an AI agent does.

The first target is the [Terraform AWS Provider](https://github.com/hashicorp/terraform-provider-aws). The design, decisions, and measurements are in [docs/design/avizor.md](docs/design/avizor.md).

Status: pre-alpha.

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
avizor brief <item-id> --format text              # everything needed to decide an item
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

Check the signals, not just the category. On the [eval](eval/README.md) corpus, 94% of confident categories are right, but 41% on duplicate reports, where Avizor finds the duplicate and also marks other work as tracked. An agent reading the top 10 candidates catches most of these ([experiment](docs/design/avizor.md#step-5-agent-experiment)).

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

Each AWS SDK for Go v2 release becomes one item per changed service. The item holds the release notes and the API model change: new and removed operations, shapes, members, and enum values, and changed member types. The change is a diff of a local bare clone of [aws/api-models-aws](https://github.com/aws/api-models-aws) (about 180 MB), which gets a commit about an hour before each release. Sometimes the models land after the release: 7 of 358 releases since 2025-06-01, up to 3 hours after. A commit after a release, and less than 4 hours after it and before the next release, counts for the release when it adds or changes a model of a module the release notes list and the next release's notes don't. The newest release waits until those 4 hours pass when no listed module has a model change yet, or when a commit after it could belong to it. Re-pulling a window with `--since` recomputes it and removes items it no longer produces.

The diff covers the shapes the SDK is generated from: the service's namespace, and other namespaces' shapes the service reaches. Other namespaces' unreachable shapes, like operations no service binds or CloudFormation helper shapes, are ignored.

An item without an API change is `skipped`, with one of these reasons: only docs, traits, or rules changed; a release note had no model change; or the model was removed. Constraint changes, like a new maximum length, are skipped too. They can still matter for provider validation.

With `--no-fetch`, the pull stops at the first release newer than the clone, since its change would look empty.

A year of releases takes about 2.5 minutes to pull.

### Announcements

The `aws-whats-new` source pulls [What's New](https://aws.amazon.com/about-aws/whats-new/) posts. Each pull starts a week before the previous one, to catch late posts. The posts aren't items. Avizor matches each item to the post that most likely announced it, by text similarity, within 3 days before and 7 days after the release. Region-expansion posts are left out, and the best post has to clearly beat the runner-up.

A match is a guess ([measurements](docs/design/avizor.md#step-4-announcements)).

The source reads the undocumented JSON endpoint behind the What's New pages, because the RSS feed holds only the latest 100 posts. Set `url` in the source config to read another endpoint.

### Blog posts

The `aws-blogs` source pulls [AWS blog](https://aws.amazon.com/blogs/) posts from the same endpoint: title, link, creation time, excerpt, and tags. The tags give the post type (`GLOBAL#content-type#launch-announcement`) and product categories (`blog-posts#category-database-amazon-aurora#dsql`). The directory has no post bodies. Like announcements, posts aren't items, and each pull starts a week before the previous one. An issue with a related blog post should link it. `avizor brief` lists the posts near an item's release, and `avizor draft --blog <url>` links the one you pick.

A year of posts (4,352 up to 2026-11-18) takes about 8 seconds to pull:

```
avizor pull aws-blogs --since 365d
```

## Inspect

`avizor inspect` shows an item and relates it to the target (`--no-enrich` skips that):

- **Service**: the provider package that uses the SDK module, or a service the provider doesn't implement.
- **Availability**: whether the target's `go.mod` requires the item's SDK version, and the commit and PR that brought it in.
- **Work** (inferred): new operations grouped into a likely resource, data source, or other operation; new members as arguments or attributes; enum values; type changes; removals. Each kind lists the provider files that call the operations involved, and for a new argument, whether those files use its likely Terraform name.
- **Spec**, for each inferred resource and data source, from the model:
  - operations by role (create, read, update, delete, list, other) and the identifier (the read's required input);
  - arguments from the create input, with likely Terraform names, required or optional, create-only (no update operation takes them), constraints, enum values, and nested members three levels deep;
  - attributes: read result members that aren't arguments;
  - the status member, with its values grouped as pending, target, failed, and delete;
  - the delete operation, and whether it takes a list of IDs (`DeleteNetworkingFlexCostAllocationPolicies`, up to 50);
  - the update shape: fields, add and remove lists, or a documented full replacement;
  - tagging: `Tags`, EC2 `TagSpecifications` with new resource types, or the service's `TagResource`;
  - documentation sentences that state design facts ("can't change", "ignores this field", "If you omit"), quoted with their source.

  Create-only and the names are inferred; the rest is what the model says.
- **Provider facts** for new members and enum values, from the code:
  - for a new nested argument or attribute, where its parent block is defined in each file that calls its operations: the Terraform path (`configuration.vpc_egress_configuration`), the schema kind (`ListNestedBlock`, `TypeList (MaxItems 1)`), the plugin library, and the nearest block on the path that forces replacement;
  - for a new enum value, the package's uses of the SDK type, split into uses that validate with it (`enum.Validate[awstypes.RoutingStrategy]`, `fwtypes.StringEnum[...]`), which accept the new value without a code change, and other uses. Whether a new value matters is left to the reader; the state doesn't change.
- **Announcement** (inferred): the matched post, with its score and the runner-up's.

## Brief

```
avizor brief aws-sdk-go-v2/dsql/release-2026-11-16 aws-sdk-go-v2/ec2/release-2026-11-13 --format text
```

`avizor brief` is one call per item, or several, with what deciding it needs:

- the release, its notes, the module version, and availability;
- the queue state and reason, and the state once the SDK version is available (`pending` is a gap waiting on the SDK, and issues can be filed before it lands);
- the current decision, if any, and whether it resurfaced;
- the draft's title and labels;
- the work with its spec and provider facts, as `inspect` shows them;
- the candidates, as `candidates` gives them (in text, up to 5 related);
- every announcement posted within 7 days of the release, scored against the item, best first (`--announcements`, default 10, 0 for all). The matcher's pick is marked, and so is each post whose text names the service. The ranking is for the reader to pick from: in the 2026-11-18 pass, the matcher's one pick was wrong or missing for 4 of 10 items, and the right post was first in this list for the DSQL and EC2 ones.
- the blog posts created from 3 days before the release to 21 days after, scored the same way against the item and the matched announcement's headline, best first (`--blog-posts`, default 5, 0 for all). Posts often come days after the release. Each is marked if it's tagged as a launch announcement, and if its product tags name the service, and those raise its rank (1.5 and 1.25 times the score). On the 51 `blog` eval cases, the linked post is first for 90% and in the top 5 for 98%. An issue with a related post should link it; pick one and draft with `--blog <url>`.

## Queue

```
avizor queue --format text
avizor queue --since 90d --service s3 --all --format text
```

`avizor queue` enriches and reconciles the items with API changes from the last 30 days and lists the ones that need a decision. Skipped items, including constraint changes, aren't queued even with `--all`; list them with `avizor items --status skipped`. It runs 4 items at a time (`--jobs`), and 30 days takes about 10 seconds. Each entry has the inferred work, a draft title, the announcement, and up to 3 related issues and PRs.

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

For `partial`, a new argument counts as present when the files calling its operations use its likely Terraform name. `avizor inspect` shows where. When a confident issue cites the item's release commit, the item stays `covered`, since that issue tracks the whole launch.

A `gap` is a guess. Many need no provider work, and covering work can be in the related list, so check that list before drafting.

## Decide

```
avizor decide aws-sdk-go-v2/docdb/release-2026-09-21 created 51234
avizor decide aws-sdk-go-v2/sagemaker/release-2026-11-16 created 51090 51091 51092
avizor decide aws-sdk-go-v2/cloudwatch/release-2026-05-01 duplicate 48548
avizor decide aws-sdk-go-v2/ecs/release-2026-11-16 duplicate 50573 \
  --comment https://github.com/hashicorp/terraform-provider-aws/pull/50896#issuecomment-6986111563
avizor decide aws-sdk-go-v2/cleanrooms/release-2026-07-27 skipped --reason "query-time setting, no resource"
avizor decide aws-sdk-go-v2/rds/release-2026-08-27 deferred --revisit 60d
avizor decide aws-sdk-go-v2/rds/release-2026-08-27 cleared
avizor decisions --format text
```

An item whose work needs several issues (one per resource) records all of them; the first is the item's main issue. `--comment` (repeatable) records comments made on existing issues or PRs instead of filing new ones; it takes the comment's URL.

A decision hides the item from the queue. Avizor records who decided (`--by`, default `$USER`) and what it found at the time. The item comes back as `resurfaced`, with the reason, when:

- the revisit date passes (`--revisit`, an age or date; `deferred` defaults to 30 days);
- for `duplicate`: any covering issue closes as anything but completed, or any covering PR closes unmerged;
- for `skipped` or `deferred`: the evidence changes, for example the SDK version becomes available, a tracker appears, or the announcement match changes.

Otherwise `created` stands. The queue also checks decided items older than `--since`. `avizor decisions --history` lists every decision, including cleared ones.

## Draft

```
avizor draft aws-sdk-go-v2/kinesis/release-2026-08-31 --format text
avizor draft aws-sdk-go-v2/kinesis/release-2026-08-31 --body | gh issue create --repo hashicorp/terraform-provider-aws --title ... --label ... --body-file -
```

`avizor draft` renders an issue in the format of the provider's trackers. Avizor prints the `gh` command but doesn't run it. The body has:

- the release notes, quoted, and with `--announcement`, a link to the matched What's New post titled with its headline. With `--blog <url>`, a `([blog post](<url>))` link follows it, or a `[Blog post](<url>)` line when there's no announcement;
- `` Requires `service/<module>` <version> ([Release <date>](<commit>)). ``, and the bump PR if the target has merged one. Reconciliation takes an issue citing the release commit as tracking the whole release, so a `partial` item's draft cites the release tag instead;
- a Scope section rendered from the model's spec and the provider's facts:
  - for a new resource or data source, its operations, its arguments (required or optional, forces replacement, type, constraints, and nested fields), and its attributes, then notes: status values to wait on, how update and delete work, how tags are set, documentation sentences that state design facts, and other operations;
  - for an existing resource, each new argument or attribute with the resource it's in, its type and constraints, the API members it comes from, and the outputs that return it, then notes on the parent block. When every line is about one resource, the title names it and the lines leave it out. When the release notes name every operation a line's members come from, the line leaves out the members from the schema (its kind, and whether it forces replacement). Members of one noun's create, update, and read shapes are one field; the create input's describes it;
  - new enum values and whether the provider validates with the SDK's type;
- for a `partial` item, `Relates #N` lines for the issues and PRs that cover the rest.

The body says what the model shows. Where it can't settle a choice, the line is marked `<!-- decide: ... -->`, and the choices are listed under `decide` (JSON) or in the reviewer section (text):

- a list of structures with one scalar field: a set of strings, or a nested block;
- an optional argument that read returns, with a default or a list: optional, or optional and computed;
- a name that abbreviates the resource's name (`FcaPolicyId` for `NetworkingFlexCostAllocationPolicy`);
- other operations, such as an associate: one resource per association, an argument, or none;
- a new read-only group: a data source, part of another resource, or none;
- a new argument that's in no resource, or in several, or under several schema blocks.

A `partial` item's draft covers only the work `queue` reports as uncovered, so it doesn't repeat the tracked work.

Guesses go in a reviewer section (text) or under `inferred` (JSON), each with its basis:

- **Title**: one of
  - ``New resource: `aws_<package>_<noun>` ``;
  - ``New resources: `<a>` and `<b>` ``, or `` `<a>`, `<b>`, and 2 more ``, for several new resources and no new data source;
  - `New data source: ...`;
  - ``r/`<resource>`: Add `<argument>` ``, when the new arguments (or, with none, attributes) are in one resource;
  - otherwise the announcement headline, or the first sentence of the notes, with a service prefix if it lacks one.
- **Labels**: `enhancement` and the service label, plus `new-resource` for an inferred resource and `new-service` for a new SDK module or a service the target doesn't implement. For such a service, the service label is a guess from the module name.

The reviewer section also lists warnings and possibly related issues and PRs:

- **Warnings**: confident existing work, an existing docs page for the Terraform name, an SDK version the target lacks, a skipped item, `--announcement` without a match, and for a `partial` item, what the draft covers. Check the existing work before filing a draft warned "already tracked".
- **Possibly related issues and PRs**: weak matches for a `Relates` line. Bug reports and the bump PR are left out.

`draft_template` replaces the body template. See the example config.

## Evaluate

`avizor eval` scores reconciliation and queue states against 207 cases labeled from provider history, and blog post ranking against 51 more. See [eval/README.md](eval/README.md).

## Develop

`make ci` runs what CI runs: build, gofmt, `go fix`, `go mod tidy`, vet, and tests. Tests need ripgrep and git. `make help` lists the targets.

To also test parsing the real service map:

```
AVIZOR_NAMES_DATA=~/go/src/github.com/hashicorp/terraform-provider-aws/names/data/names_data.hcl go test ./internal/services
```

## License

[MPL-2.0](LICENSE)
