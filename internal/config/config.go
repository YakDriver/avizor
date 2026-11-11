// Package config loads Radar's HCL configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// Config is the top-level configuration file.
type Config struct {
	Targets []Target `hcl:"target,block"`
}

// Target is a repository Radar reconciles against.
type Target struct {
	Name string `hcl:"name,label"`
	// Repo is "owner/name" on GitHub.
	Repo string `hcl:"repo"`
	// Checkout is an optional local clone used for code and docs searches.
	Checkout string `hcl:"checkout,optional"`
	// NamesData is the provider's service metadata file, relative to Checkout.
	NamesData string `hcl:"names_data,optional"`
	// ServiceLabelPrefix is the prefix of per-service labels.
	ServiceLabelPrefix string `hcl:"service_label_prefix,optional"`

	Index *Index `hcl:"index,block"`
}

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
