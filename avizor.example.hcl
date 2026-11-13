# Example Avizor configuration. Copy to ./avizor.hcl or $XDG_CONFIG_HOME/avizor/avizor.hcl and adjust paths.

target "tfaws" {
  repo = "hashicorp/terraform-provider-aws"

  # Optional local clone, used for code and docs searches and the service map.
  checkout = "~/go/src/github.com/hashicorp/terraform-provider-aws"

  # Checkout ref whose go.mod and first-parent history show which AWS SDK module versions the target uses and which
  # commit brought each in. The default is HEAD; set it to "main" if you work on other branches in this checkout.
  # ref = "HEAD"

  # Relative to checkout. This is the default.
  # names_data = "names/data/names_data.hcl"

  # Prefix of per-service labels. This is the default.
  # service_label_prefix = "service/"

  # Source whose API models define real operation names. Only those count as operation signals, so field names like
  # StartTime don't. Defaults to the first aws-sdk-go-v2 source; "none" turns the check off. The names are read from
  # the source's models clone (created by `avizor pull`) and cached next to it.
  # operations = "aws-sdk-go-v2"

  # Terraform type prefix, used to name new resources in drafts. This is the default.
  # type_prefix = "aws_"

  # Go text/template file for `avizor draft` issue bodies, relative to this file. The default follows the provider's
  # tracking issues: quoted release notes and the required SDK release with its bump PR. The template sees
  # draft.Data: .Notes (full release notes), .Headline (first sentence), .Release, .ReleaseURL, .BumpPRURL, .Service,
  # .Work, and .Item.
  # draft_template = "templates/issue.md.tmpl"

  index {
    # "urls-and-identifiers" (default) scans comments for URLs, API operation names, Terraform names, and issue
    # references, without storing comment text. "none" skips comments.
    comments = "urls-and-identifiers"
  }
}

# Optional: where indexes, items, and source clones live. Defaults to $XDG_DATA_HOME/avizor (usually
# ~/.local/share/avizor). --state-dir overrides it.
# state_dir = "~/.local/share/avizor"

# Sources feed `avizor pull`. With no source blocks, Avizor uses one aws-sdk-go-v2 source and one aws-whats-new source
# with these defaults, so these blocks are only needed to change them. Configuring any source replaces both defaults.
source "aws-sdk-go-v2" {
  type = "aws-sdk-go-v2"

  # GitHub repo whose releases are read.
  # releases_repo = "aws/aws-sdk-go-v2"

  # Smithy models repository, cloned (bare) on the first pull. A relative models_dir is under the state directory.
  # models_url    = "https://github.com/aws/api-models-aws.git"
  # models_dir    = "sources/api-models-aws.git"
  # models_branch = "main"

  # Fetch the models clone before each pull.
  # fetch = true

  # Where the first pull starts: a date, RFC 3339 time, or age. Later pulls resume after the last release pulled.
  # start = "30d"

  # Limit items to some SDK service modules, or drop some.
  # services         = ["fsx", "s3control"]
  # exclude_services = ["connect"]
}

# AWS What's New announcements, matched to items by enrichment. With no source blocks, Avizor adds this one too.
source "aws-whats-new" {
  type = "aws-whats-new"

  # The directory search endpoint behind the What's New pages. It isn't a documented API.
  # url = "https://aws.amazon.com/api/dirs/items/search?item.directoryId=whats-new-v2&sort_by=item.additionalFields.postDateTime&sort_order=desc&item.locale=en_US"

  # Where the first pull starts. Later pulls start a week before the last one, for late and back-dated posts.
  # start = "30d"
}
