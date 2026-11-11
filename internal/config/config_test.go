package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "radar.hcl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeConfig(t, `
target "tfaws" {
  repo     = "hashicorp/terraform-provider-aws"
  checkout = "/src/tfaws"
}
`))
	if err != nil {
		t.Fatal(err)
	}
	tg, err := cfg.Target("")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Index.Comments != CommentsURLsAndIdentifiers {
		t.Errorf("comments = %q", tg.Index.Comments)
	}
	if tg.ServiceLabelPrefix != "service/" {
		t.Errorf("label prefix = %q", tg.ServiceLabelPrefix)
	}
	if got, want := tg.NamesDataPath(), "/src/tfaws/names/data/names_data.hcl"; got != want {
		t.Errorf("NamesDataPath = %q, want %q", got, want)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ body, want string }{
		"no targets": {``, "at least one target"},
		"bad repo":   {`target "x" { repo = "nope" }`, "owner/name"},
		"duplicate":  {`target "x" { repo = "a/b" }` + "\n" + `target "x" { repo = "a/c" }`, "duplicate target"},
		"comments":   {`target "x" {` + "\n" + `repo = "a/b"` + "\n" + `index { comments = "all" }` + "\n}", "index.comments"},
		"unknown":    {`target "x" {` + "\n" + `repo = "a/b"` + "\n" + `bogus = 1` + "\n}", "Unsupported argument"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(writeConfig(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestTargetSelection(t *testing.T) {
	t.Parallel()

	cfg := &Config{Targets: []Target{{Name: "a"}, {Name: "b"}}}
	if _, err := cfg.Target(""); err == nil {
		t.Error("expected error choosing among several targets")
	}
	if tg, err := cfg.Target("b"); err != nil || tg.Name != "b" {
		t.Errorf("Target(b) = %v, %v", tg, err)
	}
	if _, err := cfg.Target("c"); err == nil {
		t.Error("expected error for unknown target")
	}
}
