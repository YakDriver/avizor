package avizor_test

import (
	"testing"

	"github.com/YakDriver/avizor"
	"github.com/YakDriver/avizor/internal/eval"
)

// TestDefaultCorpus checks that the built-in corpus parses and binds to the provider repo, so `avizor eval` works from
// an installed binary.
func TestDefaultCorpus(t *testing.T) {
	t.Parallel()

	c, err := eval.Parse(avizor.DefaultCorpusName, avizor.DefaultCorpus)
	if err != nil {
		t.Fatal(err)
	}
	if c.Target != "tfaws" || len(c.Cases) < 50 {
		t.Errorf("target %q with %d cases", c.Target, len(c.Cases))
	}
	if err := c.Bind("hashicorp/terraform-provider-aws"); err != nil {
		t.Fatal(err)
	}
}
