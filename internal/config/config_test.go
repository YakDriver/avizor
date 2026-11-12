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
		"path name":  {`target "../x" { repo = "a/b" }`, "target name"},
		"src name":   {`target "x" { repo = "a/b" }` + "\n" + `source "A B" { type = "aws-sdk-go-v2" }`, "source name"},
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

func TestSourceDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeConfig(t, `
target "tfaws" {
  repo = "hashicorp/terraform-provider-aws"
}
`))
	if err != nil {
		t.Fatal(err)
	}
	s, err := cfg.Source("")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != DefaultSourceName || s.ReleasesRepo != DefaultReleasesRepo || s.ModelsURL != DefaultModelsURL ||
		s.ModelsBranch != DefaultModelsBranch || s.Start != DefaultStart || !s.FetchModels() {
		t.Errorf("default source = %+v", s)
	}
	if got, want := s.ModelsPath("/state"), "/state/sources/api-models-aws.git"; got != want {
		t.Errorf("ModelsPath = %q, want %q", got, want)
	}
}

func TestSourceOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeConfig(t, `
state_dir = "/data/radar"

target "tfaws" {
  repo = "hashicorp/terraform-provider-aws"
}

source "sdk" {
  type             = "aws-sdk-go-v2"
  releases_repo    = "me/fork"
  models_url       = "https://example.com/models.git"
  models_dir       = "/srv/models.git"
  models_branch    = "dev"
  fetch            = false
  start            = "2026-01-02"
  services         = ["fsx"]
  exclude_services = ["ec2"]
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StateDir != "/data/radar" {
		t.Errorf("state_dir = %q", cfg.StateDir)
	}
	s, err := cfg.Source("sdk")
	if err != nil {
		t.Fatal(err)
	}
	if s.ReleasesRepo != "me/fork" || s.ModelsURL != "https://example.com/models.git" || s.ModelsPath("/state") != "/srv/models.git" ||
		s.ModelsBranch != "dev" || s.FetchModels() || s.Start != "2026-01-02" || s.Services[0] != "fsx" || s.ExcludeServices[0] != "ec2" {
		t.Errorf("source = %+v", s)
	}
}

func TestSourceErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ body, want string }{
		"type":  {`source "x" { type = "rss" }`, "unknown type"},
		"start": {`source "x" {` + "\n" + `type = "aws-sdk-go-v2"` + "\n" + `start = "soon"` + "\n}", "invalid time"},
		"repo":  {`source "x" {` + "\n" + `type = "aws-sdk-go-v2"` + "\n" + `releases_repo = "nope"` + "\n}", "owner/name"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(writeConfig(t, `target "t" { repo = "a/b" }`+"\n"+tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestIndexPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A target named "items" can't collide with the items database.
	if IndexPath(dir, "items") == ItemsPath(dir) {
		t.Error("index for target \"items\" collides with items.db")
	}

	// A legacy index and its WAL file move to the index directory.
	for _, suffix := range []string{"", "-wal"} {
		if err := os.WriteFile(filepath.Join(dir, "tfaws.db"+suffix), []byte("x"+suffix), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := MigrateIndex(dir, "tfaws"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal"} {
		b, err := os.ReadFile(IndexPath(dir, "tfaws") + suffix)
		if err != nil || string(b) != "x"+suffix {
			t.Errorf("migrated %s = %q, %v", suffix, b, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "tfaws.db"+suffix)); err == nil {
			t.Errorf("legacy %s still present", suffix)
		}
	}
	// Migrating again, or with nothing to migrate, is a no-op.
	if err := MigrateIndex(dir, "tfaws"); err != nil {
		t.Error(err)
	}
	if err := MigrateIndex(dir, "none"); err != nil {
		t.Error(err)
	}
}

func TestTargetOperations(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ body, want, err string }{
		"default": {body: `target "x" { repo = "a/b" }`, want: DefaultSourceName},
		"named": {body: `target "x" {
  repo       = "a/b"
  operations = "sdk"
}
source "sdk" { type = "aws-sdk-go-v2" }`, want: "sdk"},
		"none":    {body: `target "x" {` + "\n" + `repo = "a/b"` + "\n" + `operations = "none"` + "\n}", want: OperationsNone},
		"unknown": {body: `target "x" {` + "\n" + `repo = "a/b"` + "\n" + `operations = "nope"` + "\n}", err: "unknown source"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(writeConfig(t, tc.body))
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Targets[0].Operations; got != tc.want {
				t.Errorf("operations = %q, want %q", got, tc.want)
			}
		})
	}
}
