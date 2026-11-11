# Radar

Radar finds what is already known about new information, gathers supporting evidence, and prepares the next action for review. It gathers evidence and doesn't decide. A human or an AI agent reads its output and chooses what to do.

The first target is the [Terraform AWS Provider](https://github.com/hashicorp/terraform-provider-aws). See [docs/design/radar.md](docs/design/radar.md) for the design and roadmap.

Status: milestone 1 (index and reconcile).

## Install

```
go install github.com/YakDriver/radar/cmd/radar@latest
```

Radar needs [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) for code searches, and a GitHub token from `GITHUB_TOKEN`, `GH_TOKEN`, or `gh auth token`.

## Configure

Copy [radar.example.hcl](radar.example.hcl) to `./radar.hcl` or `$XDG_CONFIG_HOME/radar/radar.hcl` (usually `~/.config/radar/radar.hcl`):

```hcl
target "tfaws" {
  repo     = "hashicorp/terraform-provider-aws"
  checkout = "~/go/src/github.com/hashicorp/terraform-provider-aws"
}
```

`checkout` is optional. Without it, Radar searches only issues and PRs, and can't map service names from `names/data/names_data.hcl`.

State lives in `$XDG_DATA_HOME/radar` (usually `~/.local/share/radar`), one SQLite file per target. Override it with `--state-dir`.

## Sync

```
radar sync                 # incremental after the first run
radar sync --full          # refetch everything and drop records GitHub no longer returns
radar sync --since 90d     # index a recent slice only
```

The first full sync of the Terraform AWS Provider fetches every issue, PR, and comment, which takes a long time. A slice is enough to try Radar: `--since 2025-11-01` took about 9 minutes and indexed about 10,100 issues and PRs and 30,000 comments.

`--since` doesn't read or move the sync checkpoints. After slice-only syncs, a plain `radar sync` starts from the beginning.

The index stores titles, bodies, and labels for full-text search. From comments it stores only extracted signals (URLs, API operation names, Terraform names, and issue references), not comment text. Set `index { comments = "none" }` to skip comments.

## Reconcile

```
radar candidates 45362
radar candidates https://github.com/hashicorp/terraform-provider-aws/issues/45362
radar candidates https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/
radar candidates --text "FSx S3 access point support"
```

Output is JSON by default, for agents. Use `--format text` for people. Other flags:

| Flag | Effect |
|---|---|
| `--no-fetch` | Don't fetch a URL input to read its page title. |
| `--no-code` | Don't search the checkout. |
| `--related N` | Show up to N related candidates (default 10). |

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
- An AWS API operation name, such as `CreateAndAttachS3AccessPoint`.
- A Terraform name in both the input's title and the candidate's title, when the input asks for a new type.
- For code and docs: a type registration, a docs page, or an operation call in non-test code. Existing code is only `related` when the input asks to change a type.

Weak signals are keywords in titles, shared `service/*` labels, cross-references between the input and the candidate, GitHub URLs (often a shared SDK release commit), and Terraform names in bodies. In text output, strong signals are marked with `*`.

Signals found in more than 40 records (contributing guide links, boilerplate) are skipped and listed under `warnings`.

`summary.unmatched` is true when no candidate is better than `related`.

## Develop

`make ci` runs every check CI runs: build, gofmt, `go fix`, `go mod tidy`, vet, and tests. Run `make help` for the individual targets. Tests need ripgrep.

To also test service-map parsing against the real provider file:

```
RADAR_NAMES_DATA=~/go/src/github.com/hashicorp/terraform-provider-aws/names/data/names_data.hcl go test ./internal/services
```

## License

[MPL-2.0](LICENSE)
