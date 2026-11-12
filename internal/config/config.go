// Package config loads Radar's HCL configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// Comment indexing modes.
const (
	CommentsNone               = "none"
	CommentsURLsAndIdentifiers = "urls-and-identifiers"
	defaultNamesData           = "names/data/names_data.hcl"
	defaultServiceLabelPrefix  = "service/"
	defaultCommentsMode        = CommentsURLsAndIdentifiers
	configFileName             = "radar.hcl"
	stateDirName               = "radar"
)

var (
	repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	// namePattern limits target and source names, which become file names and item ID prefixes.
	namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// Config is the top-level configuration file.
type Config struct {
	// StateDir overrides the default state directory. The --state-dir flag overrides it in turn.
	StateDir string   `hcl:"state_dir,optional"`
	Targets  []Target `hcl:"target,block"`
	Sources  []Source `hcl:"source,block"`
}

// Source types.
const (
	SourceAWSSDKGoV2 = "aws-sdk-go-v2"
)

// Source defaults.
const (
	DefaultSourceName   = "aws-sdk-go-v2"
	DefaultReleasesRepo = "aws/aws-sdk-go-v2"
	DefaultModelsURL    = "https://github.com/aws/api-models-aws.git"
	DefaultModelsDir    = "sources/api-models-aws.git"
	DefaultModelsBranch = "main"
	DefaultStart        = "30d"
)

// Source produces items. Every setting except type has a default; a config with no source blocks gets one
// aws-sdk-go-v2 source with all defaults.
type Source struct {
	Name string `hcl:"name,label"`
	Type string `hcl:"type"`
	// ReleasesRepo is the GitHub repo ("owner/name") whose releases are read.
	ReleasesRepo string `hcl:"releases_repo,optional"`
	// ModelsURL is the Smithy models repository cloned into ModelsDir.
	ModelsURL string `hcl:"models_url,optional"`
	// ModelsDir is the models clone. A relative path is under the state directory.
	ModelsDir    string `hcl:"models_dir,optional"`
	ModelsBranch string `hcl:"models_branch,optional"`
	// Fetch updates the models clone before each pull. Default true.
	Fetch *bool `hcl:"fetch,optional"`
	// Start is where the first pull begins: a date (2026-01-02), RFC 3339 time, or age (30d, 12h). Later pulls resume
	// from the last release pulled.
	Start string `hcl:"start,optional"`
	// Services, if set, limits items to these SDK service modules (e.g. "fsx", "s3control").
	Services []string `hcl:"services,optional"`
	// ExcludeServices drops items for these modules.
	ExcludeServices []string `hcl:"exclude_services,optional"`
}

// FetchModels reports whether to fetch the models clone before pulling.
func (s *Source) FetchModels() bool {
	return s.Fetch == nil || *s.Fetch
}

// ModelsPath returns the models clone directory, resolving a relative ModelsDir against stateDir.
func (s *Source) ModelsPath(stateDir string) string {
	if filepath.IsAbs(s.ModelsDir) {
		return s.ModelsDir
	}
	return filepath.Join(stateDir, s.ModelsDir)
}

func (s *Source) setDefaults() error {
	switch s.Type {
	case SourceAWSSDKGoV2:
	default:
		return fmt.Errorf("source %q: unknown type %q (supported: %q)", s.Name, s.Type, SourceAWSSDKGoV2)
	}
	if s.ReleasesRepo == "" {
		s.ReleasesRepo = DefaultReleasesRepo
	}
	if !repoPattern.MatchString(s.ReleasesRepo) {
		return fmt.Errorf("source %q: releases_repo must be owner/name, got %q", s.Name, s.ReleasesRepo)
	}
	if s.ModelsURL == "" {
		s.ModelsURL = DefaultModelsURL
	}
	if s.ModelsDir == "" {
		s.ModelsDir = DefaultModelsDir
	}
	s.ModelsDir = expandHome(s.ModelsDir)
	if s.ModelsBranch == "" {
		s.ModelsBranch = DefaultModelsBranch
	}
	if s.Start == "" {
		s.Start = DefaultStart
	}
	if _, err := ParseSince(s.Start, time.Now()); err != nil {
		return fmt.Errorf("source %q: start: %w", s.Name, err)
	}
	return nil
}

// Source returns the named source. If name is empty and there is exactly one source, it returns that one.
func (c *Config) Source(name string) (*Source, error) {
	if name == "" {
		if len(c.Sources) == 1 {
			return &c.Sources[0], nil
		}
		names := make([]string, 0, len(c.Sources))
		for _, s := range c.Sources {
			names = append(names, s.Name)
		}
		return nil, fmt.Errorf("multiple sources configured (%s); name one", strings.Join(names, ", "))
	}
	for i := range c.Sources {
		if c.Sources[i].Name == name {
			return &c.Sources[i], nil
		}
	}
	return nil, fmt.Errorf("no source named %q", name)
}

var ageRE = regexp.MustCompile(`^(\d+)([dh])$`)

// ParseSince parses a date (2026-01-02, start of day UTC), RFC 3339 time, or age before now (90d, 12h).
func ParseSince(s string, now time.Time) (time.Time, error) {
	if m := ageRE.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := time.Hour
		if m[2] == "d" {
			unit = 24 * time.Hour
		}
		return now.Add(-time.Duration(n) * unit), nil
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use a date (2026-01-02), RFC 3339 time, or age (90d, 12h)", s)
}

// Target is a repository Radar reconciles against.
type Target struct {
	Name string `hcl:"name,label"`
	// Repo is "owner/name" on GitHub.
	Repo string `hcl:"repo"`
	// Checkout is an optional local clone used for code and docs searches.
	Checkout string `hcl:"checkout,optional"`
	// Ref is the checkout ref whose go.mod and history show which SDK module versions the target uses. Empty means
	// HEAD.
	Ref string `hcl:"ref,optional"`
	// NamesData is the provider's service metadata file, relative to Checkout.
	NamesData string `hcl:"names_data,optional"`
	// ServiceLabelPrefix is the prefix of per-service labels.
	ServiceLabelPrefix string `hcl:"service_label_prefix,optional"`
	// Operations names the source whose API models define real operation names, so only those count as operation
	// signals. It defaults to the first aws-sdk-go-v2 source; "none" turns the check off.
	Operations string `hcl:"operations,optional"`

	Index *Index `hcl:"index,block"`
}

// OperationsNone turns off checking operation names against API models.
const OperationsNone = "none"

// Index controls what the sync stores.
type Index struct {
	Comments string `hcl:"comments,optional"`
}

// Load reads and validates a configuration file.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := hclsimple.DecodeFile(path, nil, &cfg); err != nil {
		return nil, fmt.Errorf("loading config %s: %w", path, err)
	}

	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("config %s: at least one target block is required", path)
	}

	seen := map[string]bool{}
	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		if seen[t.Name] {
			return nil, fmt.Errorf("config %s: duplicate target %q", path, t.Name)
		}
		seen[t.Name] = true

		if !namePattern.MatchString(t.Name) {
			return nil, fmt.Errorf("config %s: target name %q must be lowercase letters, digits, dashes, and underscores", path, t.Name)
		}
		if !repoPattern.MatchString(t.Repo) {
			return nil, fmt.Errorf("target %q: repo must be owner/name, got %q", t.Name, t.Repo)
		}
		if t.Checkout != "" {
			t.Checkout = expandHome(t.Checkout)
		}
		if t.NamesData == "" {
			t.NamesData = defaultNamesData
		}
		if t.ServiceLabelPrefix == "" {
			t.ServiceLabelPrefix = defaultServiceLabelPrefix
		}
		if t.Index == nil {
			t.Index = &Index{}
		}
		switch t.Index.Comments {
		case "":
			t.Index.Comments = defaultCommentsMode
		case CommentsNone, CommentsURLsAndIdentifiers:
		default:
			return nil, fmt.Errorf("target %q: index.comments must be %q or %q, got %q", t.Name, CommentsNone, CommentsURLsAndIdentifiers, t.Index.Comments)
		}
	}

	if cfg.StateDir != "" {
		cfg.StateDir = expandHome(cfg.StateDir)
	}
	if len(cfg.Sources) == 0 {
		cfg.Sources = []Source{{Name: DefaultSourceName, Type: SourceAWSSDKGoV2}}
	}
	seenSources := map[string]bool{}
	for i := range cfg.Sources {
		s := &cfg.Sources[i]
		if seenSources[s.Name] {
			return nil, fmt.Errorf("config %s: duplicate source %q", path, s.Name)
		}
		seenSources[s.Name] = true
		if !namePattern.MatchString(s.Name) {
			return nil, fmt.Errorf("config %s: source name %q must be lowercase letters, digits, dashes, and underscores", path, s.Name)
		}
		if err := s.setDefaults(); err != nil {
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
	}

	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		switch {
		case t.Operations == OperationsNone:
		case t.Operations == "":
			for _, s := range cfg.Sources {
				if s.Type == SourceAWSSDKGoV2 {
					t.Operations = s.Name
					break
				}
			}
			if t.Operations == "" {
				t.Operations = OperationsNone
			}
		case !seenSources[t.Operations]:
			return nil, fmt.Errorf("target %q: operations names unknown source %q", t.Name, t.Operations)
		}
	}

	return &cfg, nil
}

// Target returns the named target. If name is empty and there is exactly one target, it returns that one.
func (c *Config) Target(name string) (*Target, error) {
	if name == "" {
		if len(c.Targets) == 1 {
			return &c.Targets[0], nil
		}
		names := make([]string, 0, len(c.Targets))
		for _, t := range c.Targets {
			names = append(names, t.Name)
		}
		return nil, fmt.Errorf("multiple targets configured (%s); use --target", strings.Join(names, ", "))
	}
	for i := range c.Targets {
		if c.Targets[i].Name == name {
			return &c.Targets[i], nil
		}
	}
	return nil, fmt.Errorf("no target named %q", name)
}

// NamesDataPath returns the absolute path of the names data file, or "" if there is no checkout.
func (t *Target) NamesDataPath() string {
	if t.Checkout == "" {
		return ""
	}
	if filepath.IsAbs(t.NamesData) {
		return t.NamesData
	}
	return filepath.Join(t.Checkout, t.NamesData)
}

// DefaultPath finds the config file: ./radar.hcl, then $XDG_CONFIG_HOME/radar/radar.hcl.
func DefaultPath() (string, error) {
	if _, err := os.Stat(configFileName); err == nil {
		return configFileName, nil
	}
	dir, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, stateDirName, configFileName)
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no config found at ./%s or %s; use --config", configFileName, p)
		}
		return "", err
	}
	return p, nil
}

// DefaultStateDir returns $XDG_DATA_HOME/radar, usually ~/.local/share/radar.
func DefaultStateDir() (string, error) {
	dir, err := xdgDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateDirName), nil
}

func xdgDir(env, fallback string) (string, error) {
	if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, fallback), nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// IndexPath returns where a target's index lives under stateDir: index/<target>.db. Indexes have their own directory so
// target names can't collide with other state files such as items.db.
func IndexPath(stateDir, target string) string {
	return filepath.Join(stateDir, "index", target+".db")
}

// ItemsPath returns where pulled items live under stateDir.
func ItemsPath(stateDir string) string {
	return filepath.Join(stateDir, "items.db")
}

// MigrateIndex moves a target's index from its pre-index-directory location, <state>/<target>.db, if it exists there
// and not at IndexPath. SQLite's -wal and -shm files move with it.
func MigrateIndex(stateDir, target string) error {
	legacy := filepath.Join(stateDir, target+".db")
	dst := IndexPath(stateDir, target)
	if _, err := os.Stat(legacy); err != nil {
		return nil
	}
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Rename(legacy+suffix, dst+suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("moving index %s to %s: %w", legacy+suffix, dst+suffix, err)
		}
	}
	return nil
}
