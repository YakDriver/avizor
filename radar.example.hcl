# Example Radar configuration. Copy to ./radar.hcl or $XDG_CONFIG_HOME/radar/radar.hcl and adjust paths.

target "tfaws" {
  repo = "hashicorp/terraform-provider-aws"

  # Optional local clone, used for code and docs searches and the service map.
  checkout = "~/go/src/github.com/hashicorp/terraform-provider-aws"

  # Relative to checkout. This is the default.
  # names_data = "names/data/names_data.hcl"

  # Prefix of per-service labels. This is the default.
  # service_label_prefix = "service/"

  index {
    # "urls-and-identifiers" (default) scans comments for URLs, API operation names, Terraform names, and issue
    # references, without storing comment text. "none" skips comments.
    comments = "urls-and-identifiers"
  }
}

# Optional: where indexes, items, and source clones live. Defaults to $XDG_DATA_HOME/radar (usually
# ~/.local/share/radar). --state-dir overrides it.
# state_dir = "~/.local/share/radar"

# Sources produce items for `radar pull`. With no source blocks, Radar uses one aws-sdk-go-v2 source with these
# defaults, so this block is only needed to change them.
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
