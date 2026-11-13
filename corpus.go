// Package avizor holds files embedded in the avizor binary.
package avizor

import _ "embed"

// DefaultCorpusName is the path of the default evaluation corpus in the repository.
const DefaultCorpusName = "eval/tfaws.hcl"

// DefaultCorpus is the default evaluation corpus, so an installed binary can run `avizor eval` without a source checkout.
//
//go:embed eval/tfaws.hcl
var DefaultCorpus []byte
