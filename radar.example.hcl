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
