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
avizor posts --launch --format text               # blog posts that nothing covers
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

The diff covers the service's namespace and the shapes the service reaches in other namespaces. Other namespaces' unreachable shapes, like operations no service binds or CloudFormation helper shapes, are ignored.

An item without an API change is `skipped`, with one of these reasons: only docs, traits, or rules changed; a release note had no model change; or the model was removed. Constraint changes, like a new maximum length, are skipped too. They can still matter for provider validation.

With `--no-fetch`, the pull stops at the first release newer than the clone, since its change would look empty.

A year of releases takes about 2.5 minutes to pull.

### Announcements

The `aws-whats-new` source pulls [What's New](https://aws.amazon.com/about-aws/whats-new/) posts. Each pull starts a week before the previous one, to catch late posts. The posts aren't items. Avizor matches each item to the post that most likely announced it, by text similarity, within 3 days before and 7 days after the release. Region-expansion posts are left out, and the best post has to clearly beat the runner-up.

A match is a guess ([measurements](docs/design/avizor.md#step-4-announcements)).

The source reads the undocumented JSON endpoint behind the What's New pages, because the RSS feed holds only the latest 100 posts. Set `url` in the source config to read another endpoint.

### Blog posts

The `aws-blogs` source pulls [AWS blog](https://aws.amazon.com/blogs/) posts from the same endpoint: title, link, creation time, excerpt, and tags. The tags give the post type (`GLOBAL#content-type#launch-announcement`) and product categories (`blog-posts#category-database-amazon-aurora#dsql`). The directory has no post bodies. Like announcements, posts aren't items, and each pull starts a week before the previous one. An issue with a related blog post should link it. `avizor brief` lists the posts near an item's release, and `avizor draft --blog <url>` links the ones you pick.

A year of posts (4,352 up to 2026-11-18) takes about 8 seconds to pull:

```
avizor pull aws-blogs --since 365d
```

## Inspect

`avizor inspect` shows an item and relates it to the target (`--no-enrich` skips that):

- **Service**: the provider package that uses the SDK module, or a service the provider doesn't implement.
- **Availability**: whether the target's `go.mod` requires the item's SDK version, and the commit and PR that brought it in.
- **Work** (inferred): new operations grouped into a likely resource, data source, ephemeral resource, or other operation; new members as arguments or attributes; enum values; type changes; removals. Each kind lists the provider files that call the operations involved, and for a new argument, whether those files use its likely Terraform name. A group whose only operation creates or gets a value (Create, Get, Generate, Issue) and returns a secret is an ephemeral resource, so the value stays out of state: a member with the sensitive trait, within three levels, named like a token, secret, password, credentials, or key (`CreateBearerToken`'s `accessToken`, `GetScopedCredentials`'s `Credentials.SecretAccessKey`). The sensitive trait alone also marks personal data, such as email addresses, so the name has to match too. Its arguments are the operation's input and its attributes the output's members, listed with their fields, with secrets marked sensitive. A Get is its read and any other verb its create, and an operation of the model that reads, updates, deletes, cancels, or lists the same noun makes it a resource instead. Levels count as fields do, and paging members such as a sensitive `NextToken` don't count. For `partial`, an ephemeral resource no code calls and no candidate names is uncovered, though it has no delete. Over the 120 days to 2026-11-24, 8 items each have one ephemeral resource, and all of them return tokens or credentials.
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
- the blog posts created from 3 days before the release to 45 days after, scored the same way against the item and the matched announcement's headline, best first (`--blog-posts`, default 5, 0 for all). Posts often come days after the release, and one came 28 days after. Each is marked if it's tagged as a launch announcement, and if its product tags name the service, and those raise its rank (1.5 and 1.25 times the score). On the 51 `blog` eval cases, the linked post is first for 90% and in the top 5 for 98%. An issue with a related post should link it: `draft --announcement` links the posts that match the announcement, and `--blog <url>` adds others.

## Queue

```
avizor queue --format text
avizor queue --since 90d --service s3 --all --format text
```

`avizor queue` enriches and reconciles the items with API changes from the last 30 days and lists the ones that need a decision. Skipped items, including constraint changes, aren't queued even with `--all`; list them with `avizor items --status skipped`. It runs 4 items at a time (`--jobs`), and 30 days takes about 10 seconds. Each entry has the inferred work, a draft title, the announcement, and up to 3 related issues and PRs.

| State | Shown | Meaning |
|---|---|---|
| `resurfaced` | yes | A decision needs another look. |
| `gap` | yes | The work touches code the target has, or adds a resource or ephemeral resource, and nothing confidently covers it. |
| `pending` | yes | A gap whose SDK version the target doesn't have yet. |
| `new_service` | yes | A new SDK module for a service the target doesn't implement. |
| `partial` | yes | Covered, but some new resource or argument has no code or candidate. |
| `decided` | no | A decision stands. |
| `covered` | no | A confident candidate covers it. |
| `no_code` | no | The work touches no code the target has and adds no resource. |
| `unimplemented` | no | The target doesn't implement the service. |
| `no_work` | no | No work inferred, as for documentation-only changes. |

`--all` shows every state, and `--state gap,covered` picks some. An item counts as `no_code` only if the code search ran. With `--no-code`, nothing is hidden as `no_code` or shown as `partial`.

For `partial`, a new argument counts as present when the files calling its operations use its likely Terraform name. `avizor inspect` shows where. It counts as tracked when a confident candidate's title names it, or its body does and the candidate isn't about another resource: its title names no Terraform type, or one registered in those files. A mention in a paragraph that puts it out of scope ("not part of this request", "separate change") doesn't count. On a copy of the 2026-11-23 state with item decisions removed, this makes 1 of 787 items over 120 days `partial` instead of `covered`: observabilityadmin 2026-08-03, whose telemetry rule `kms_key_arn` only a centralization rule PR names. When a confident issue cites the item's release commit, the item stays `covered`, since that issue tracks the whole launch.

A `gap` is a guess. Many need no provider work, and covering work can be in the related list, so check that list before drafting.

## Posts

```
avizor posts --format text
avizor posts --since 2026-11-01 --launch --all --format text
avizor posts --significant --format text
```

`avizor posts` checks whether existing issues and PRs cover each blog post from the last 30 days, newest first. AWS writes blog posts for the bigger launches, so every post with Terraform work should have an issue. The check doesn't need an SDK release: it reconciles the post's title, excerpt, and URL, plus the URLs of the What's New announcements that match it, as `candidates --text` would. An announcement matches when it was posted from 45 days before the post to 3 days after and scores at least 0.35, and 75% of the best. An open issue or PR, or a merged PR, with a strong signal covers the post. Usually that signal is a link to the post or to one of its announcements. An API operation the post names is strong only when a pulled item released from a year before the post to a week after it added the operation. An older one, like the `InvokeModel` in a model availability post, is named by issues and PRs about any feature that calls it. With no pulled items, every operation counts. On a copy of the 2026-11-24 state with post decisions removed, this opens 4 of the 188 posts covered over a year, all decided as no work, and changes no other post's covering issues. The Bedrock Claude post had been covered by 16 Bedrock issues and PRs through `InvokeModel` and references to them.

| State | Shown | Meaning |
|---|---|---|
| `open` | yes | Nothing covers the post, and it has no decision. |
| `covered` | no | A decision names the covering issues, a confident issue or PR covers it, or a decision on one of its items covers it. |
| `no_work` | no | Decided as having no Terraform work (`skipped`). |
| `deferred` | no | Deferred, and the revisit date hasn't passed. |

A post is significant when it's likely to have Terraform work. It needs one of these signals: it's tagged as a launch announcement, it's on the AWS News Blog, or it matches a What's New announcement. Some kinds of post aren't significant whatever their signals:

- roundups ("Weekly Roundup", "Monthly Roundup", "ICYMI", "This Month in");
- event guides (the events tag or "re:Invent" in the title);
- developer tools (the developer tools tags or the Developer Tools Blog);
- customer and partner stories (their post-type tags or the APN Blog);
- model availability ("now available in Amazon Bedrock", "Amazon Bedrock expands ... model availability", or SageMaker JumpStart in the title);
- AWS Transform ("AWS Transform" or "ATX" in the title);
- the AWS Open Source Blog;
- new Regions and Local Zones ("Now Open", "Local Zones in", "new Region").

The summary reports coverage for all posts and for significant ones, and `--significant` lists only significant posts. On the 112 posts decided in the 2026-11-19 coverage pass, 34 of the 36 with work are significant, and 24 of the 76 without work are left out. Over the 90 days to 2026-11-23, the last four kinds and the "This Month in" roundups leave out 25 of 329 significant posts, all decided as no work. Over a year, they leave out 51 of 908, none covered by an issue.

`--launch` keeps only posts tagged as launch announcements. Each post is marked if it's on the AWS News Blog (`news`), a launch (`launch`), or significant (`sig`), and the text output has its URL for `avizor decide`. A post older than `--since` is checked too when its decision's revisit date has passed, and listed if it reopens. A revisit date on any decision ends it: the post is reconciled again. `--all` shows every state, `--state open,covered` picks some, and `--as-of` reads the index as it was at a date. The summary lines report coverage: covered posts as a share of covered and open ones. The 104 launch posts from 2026-11-02 to 2026-11-19 take about 4 seconds.

Each post lists up to 3 pulled SDK items to draft its issue from (`items` in JSON; in text, for open posts). Items that match one of the post's announcements come first. The rest are ranked by text similarity between the post's title, excerpt, and announcement headlines and the item's release notes and the names of the operations, shapes, and members it adds. There's no blog window: any item released up to a year before the post, or a week after, can match, since one post came 109 days after its release. An item released more than 45 days before the post counts half, and one whose service the post's tags name counts 1.25 times. On the 32 labeled posts with an item, the right one is first for 23 and listed for 25. Draft from it with `avizor brief <item>` and `avizor draft --blog <post-url> <item>`. With no item listed, or no right one, write the issue by hand.

A `created` or `duplicate` decision on an item also covers a post with no decision of its own, when the item matches one of the post's announcements and that announcement scores at least 0.5 against the post, and the post isn't a roundup or customer story. The text output says `covered by #N (item <id>)`, and the JSON has `covered_by`. Filing from an item then needs one `decide`. With the 24 post decisions that repeat an item's issue removed, from a copy of the 2026-11-20 state, 15 of the posts are covered again, 5 by their items, against 10 without this; no other post is covered by an item. A weaker announcement match is often another launch of the service, which covered two posts wrongly at 0.43 and 0.45.

An issue that doesn't link the post or its announcement, and isn't on a matching item, isn't found, so record it on the post: `avizor decide <post-url> duplicate 51064`. Record a post with no Terraform work as `skipped`, with a reason.

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

A blog post's URL works in place of an item ID, as in `avizor decide https://aws.amazon.com/blogs/... skipped --reason "customer story"`. Avizor records no evidence for a post, since `avizor posts` reconciles it on each run.

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

- the release notes, quoted;
- a list of links: with `--announcement`, the matched What's New post titled with its headline; the blog posts given with `--blog <url>` (repeatable), each titled with its stored title; and with `--announcement`, up to 2 blog posts near the release that match the same announcement, leaving out roundups and customer stories. On the 51 `blog` eval cases, 35 with an announcement, an expected post is linked for 30, and 31 of the 40 linked posts are expected;
- `` Requires `service/<module>` <version> ([Release <date>](<commit>)), which the provider has. ``, the last part when the target has merged the bump. It's the last item of the list, or its own line when there are no links. Reconciliation takes an issue citing the release commit as tracking the whole release, so a `partial` item's draft cites the release tag instead;
- a Scope section rendered from the model's spec and the provider's facts:
  - for a new resource or data source, its operations, its arguments (required or optional, forces replacement, type, constraints, and nested fields), and its attributes, then notes: status values to wait on, how update and delete work, how tags are set, documentation sentences that state design facts, and other operations;
  - for an existing resource or data source, each new argument or attribute with the resource it's in, its type and constraints, the API members it comes from, and the outputs that return it, then notes on the parent block. When every line is about one resource, the title names it and the lines leave it out. When the release notes name every operation a line's members come from, the line leaves out the members from the schema (its kind, and whether it forces replacement). Members of one noun's create, update, and read shapes are one field; the create input's describes it. So are arguments of the same name, blocks, and shape that one resource file writes, from inputs of different nouns (`CreateHostedZoneRequest` and `UpdateHostedZoneFeaturesRequest`) or a block's create and update shapes (`AgentAccessConfig` and `AgentAccessConfigForUpdate`). A query's request member, a data source's filter, isn't merged with them. Members the release notes, announcement headline, or blog post titles name come first;
  - new enum values and whether the provider validates with the SDK's type;

  A member's resource is one registered in the files that call its operations: a resource rather than a data source, except for a request member of query operations (Get, Describe, List, BatchGet, Search), which is a data source's argument. Names registered by consecutive annotations are one resource, the last name (`aws_lb_listener`, not `aws_alb_listener`). Of several resources, those whose names best match one of the operations' nouns are kept: for each operation, the fewest words its noun lacks, then the most it has, not counting the service word or words like `configuration` (`aws_rds_cluster`, not `aws_rds_global_cluster`, for `ModifyDBCluster`). Of those, for a member of a nested shape, the ones whose names have the most of the shape's words are kept (`aws_fsx_ontap_volume` for `CreateOntapVolumeConfiguration`, and `aws_fsx_openzfs_snapshot`, not every FSx resource, for `Snapshot`). A member in several blocks of its resource is in each; blocks in files that register only other resources are left out, and blocks in schema helper files, which register none, are kept. An attribute joins the argument of the same name and blocks when read returns it: its operations return the argument, or the fields have the same shape (or the same scalar type and values) and the files that read it also write the argument. A summary or list result attribute joins the full shape's when the same files call both, not counting list resources and sweepers, which only list. When only those call it, it joins if it's a summary of the field's noun (`ChannelSummary` of `Channel`) or its resource's list resource calls it (`document_list.go` for `document.go`). A member of a Put or other operation input with no block joins the same-named argument in the block the input's noun names, when a file calls both (`PutConfigurationSetDeliveryOptionsRequest.SharedPoolDeliveryMode` is `delivery_options.shared_pool_delivery_mode`). An attribute of read operations named like the end of a new resource's name, when that resource has no read operation, is a note on the resource saying where to read it (`GetAccountResponse.DeliveryAttributes` for `aws_sesv2_account_delivery_attributes`), unless a calling resource is named exactly for the read's noun. A list of at most one, or one documented as having exactly one entry (Kinesis's `StreamConfigurationList`), is a single block, and a recursive structure or union (DynamoDB's `AttributeValue`) is named instead of listed. So is a structure below the third level ("structure `OutputTextFormat`, fields not listed"), and a structure with no members is an "empty structure". A parent block in an XML-wrapped list is found under the wrapper's name (CloudFront's `origin.custom_origin_config`, in `Origins.Items`). A new resource whose release adds no read uses a `Get` or `Describe` of its noun, or its plural, that the model already has, and a resource with no read at all has a note saying so.

  Some work is left out and listed under `inferred` as `left_out`, with why. A new data source whose operations only list or search is always left out: it depends on the API, not the provider. When the code search ran on an implemented service, these are left out too: an attribute of operations no provider file calls, such as a game session's status; a request member of query operations no data source calls; a `Reset` flag; and an update's `Add` and `Remove` members, which a note covers. An argument of operations no provider file calls is in a note naming them, since its resource doesn't exist yet. With no ARN of its own in the create or read output (`Arn`, or named for the end of its noun, like `AssignmentArn`), a new resource isn't tagged with `TagResource`. A policy assignment's `PolicyArn` is what it assigns;
- for a `partial` item, `Relates #N` lines for the issues and PRs that cover the rest.

The body says what the model shows. Where it can't settle a choice, the line is marked `<!-- decide: ... -->`, and the choices are listed under `decide` (JSON) or in the reviewer section (text):

- a list of structures with one scalar field: a set of strings, or a nested block;
- an optional argument that read returns, with a default or a list: optional, or optional and computed;
- a name that abbreviates the resource's name (`FcaPolicyId` for `NetworkingFlexCostAllocationPolicy`);
- other operations, such as an associate: one resource per association, an argument, or none;
- a new read-only group: a data source, part of another resource, or none;
- a new argument that's in no resource, or in several.

A `partial` item's draft covers only the work `queue` reports as uncovered, so it doesn't repeat the tracked work.

Guesses go in a reviewer section (text) or under `inferred` (JSON), each with its basis:

- **Title**: one of
  - ``New resource: `aws_<package>_<noun>` ``;
  - ``New resources: `<a>` and `<b>` ``, or `` `<a>`, `<b>`, and 2 more ``, for several new resources and no new data source;
  - `New data source: ...`;
  - ``New ephemeral resource: `aws_<package>_<noun>` ``, or `New ephemeral resources: ...`, when there's no new resource;
  - ``r/`<resource>`: Add `<argument>` ``, when the new arguments (or, with none, attributes) are in one resource, or ``r/`<a>`, r/`<b>`: Add ...`` for two resources and no announcement;
  - otherwise the announcement headline, or the first sentence of the notes, with a service prefix if it lacks one.
- **Labels**: `enhancement` and the service label, plus `new-resource` for an inferred resource, `new-ephemeral-resource` for an ephemeral resource, and `new-service` for a new SDK module or a service the target doesn't implement. For such a service, the service label is a guess from the module name.

The reviewer section also lists warnings and possibly related issues and PRs:

- **Warnings**: confident existing work, an existing docs page for the Terraform name, an SDK version the target lacks, a skipped item, `--announcement` without a match, and for a `partial` item, what the draft covers. Check the existing work before filing a draft warned "already tracked".
- **Possibly related issues and PRs**: weak matches for a `Relates` line. Bug reports and the bump PR are left out.

`draft_template` replaces the body template. See the example config.

## Evaluate

`avizor eval` scores reconciliation and queue states against 207 cases labeled from provider history, blog post ranking against 51 more, blog post coverage against 116 posts, and significance and items against 112 of them. See [eval/README.md](eval/README.md).

## Develop

`make ci` runs what CI runs: build, gofmt, `go fix`, `go mod tidy`, vet, and tests. Tests need ripgrep and git. `make help` lists the targets.

To also test parsing the real service map:

```
AVIZOR_NAMES_DATA=~/go/src/github.com/hashicorp/terraform-provider-aws/names/data/names_data.hcl go test ./internal/services
```

## License

[MPL-2.0](LICENSE)
