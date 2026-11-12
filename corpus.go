// Package radar holds files embedded in the radar binary.
package radar

import _ "embed"

// DefaultCorpusName is the path of the default evaluation corpus in the repository.
const DefaultCorpusName = "eval/tfaws.hcl"

// DefaultCorpus is the default evaluation corpus, so an installed binary can run `radar eval` without a source checkout.
//
//go:embed eval/tfaws.hcl
var DefaultCorpus []byte
