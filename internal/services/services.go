// Package services maps text and labels to provider service packages using the provider's names_data.hcl.
package services

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
)

// Service is one provider service package.
type Service struct {
	// Package is the provider Go package, e.g. "fsx".
	Package string `json:"package"`
	// HumanFriendly is the display name, e.g. "FSx".
	HumanFriendly string `json:"human_friendly"`
	// SDKID is the AWS SDK service ID, e.g. "FSx".
	SDKID string `json:"sdk_id,omitempty"`
	// ResourcePrefixes are Terraform type prefixes, e.g. "aws_fsx_".
	ResourcePrefixes []string `json:"resource_prefixes"`
	// Module is the AWS SDK for Go v2 service module the package uses, e.g. "fsx", or "" if it uses none.
	Module string `json:"module,omitempty"`
	// NotImplemented is set for services the provider lists but doesn't implement.
	NotImplemented bool `json:"not_implemented,omitempty"`
	// Names are the strings matched in free text.
	Names []string `json:"-"`
}

// Map is the set of known services.
type Map struct {
	byPackage map[string]*Service
	// byModule includes not-implemented services.
	byModule map[string]*Service
	// prefixes sorted longest first so aws_s3control_ wins over aws_s3_.
	prefixes []prefixEntry
	names    []nameEntry
}

type prefixEntry struct {
	prefix string
	svc    *Service
}

type nameEntry struct {
	re   *regexp.Regexp
	name string
	svc  *Service
}

type namesFile struct {
	Services []serviceBlock `hcl:"service,block"`
	Remain   hcl.Body       `hcl:",remain"`
}

type serviceBlock struct {
	Label          string               `hcl:"name,label"`
	SDK            *sdkBlock            `hcl:"sdk,block"`
	GoPackages     *goPackagesBlock     `hcl:"go_packages,block"`
	Names          *namesBlock          `hcl:"names,block"`
	ResourcePrefix *resourcePrefixBlock `hcl:"resource_prefix,block"`
	NotImplemented bool                 `hcl:"not_implemented,optional"`
	Remain         hcl.Body             `hcl:",remain"`
}

type sdkBlock struct {
	ID     string   `hcl:"id,optional"`
	Remain hcl.Body `hcl:",remain"`
}

type goPackagesBlock struct {
	V2Package *string  `hcl:"v2_package,optional"`
	Remain    hcl.Body `hcl:",remain"`
}

type namesBlock struct {
	Aliases           []string `hcl:"aliases,optional"`
	ProviderNameUpper string   `hcl:"provider_name_upper,optional"`
	HumanFriendly     string   `hcl:"human_friendly,optional"`
	Remain            hcl.Body `hcl:",remain"`
}

type resourcePrefixBlock struct {
	Actual  string   `hcl:"actual,optional"`
	Correct string   `hcl:"correct,optional"`
	Remain  hcl.Body `hcl:",remain"`
}

// Load parses a names_data.hcl file. Services marked not_implemented are only found by ByModule.
func Load(path string) (*Map, error) {
	var f namesFile
	if err := hclsimple.DecodeFile(path, nil, &f); err != nil {
		return nil, fmt.Errorf("loading service names %s: %w", path, err)
	}

	m := &Map{byPackage: map[string]*Service{}, byModule: map[string]*Service{}}
	for _, b := range f.Services {
		// The block label is the provider package, its directory, and its label suffix. provider_package_correct is
		// only the preferred name (for example "es" for "elasticsearch"), so it isn't used.
		svc := &Service{Package: b.Label, Module: b.Label, NotImplemented: b.NotImplemented}
		// go_packages.v2_package overrides the module; an empty one means the service has no v2 module.
		if b.GoPackages != nil && b.GoPackages.V2Package != nil {
			svc.Module = *b.GoPackages.V2Package
		}
		if b.NotImplemented {
			if svc.Module != "" {
				m.byModule[svc.Module] = svc
			}
			continue
		}

		var names []string
		if b.SDK != nil {
			svc.SDKID = b.SDK.ID
			names = append(names, b.SDK.ID)
		}
		if b.Names != nil {
			svc.HumanFriendly = b.Names.HumanFriendly
			names = append(names, splitHumanFriendly(b.Names.HumanFriendly)...)
			names = append(names, b.Names.ProviderNameUpper)
		}
		if b.ResourcePrefix != nil {
			for _, p := range []string{b.ResourcePrefix.Actual, b.ResourcePrefix.Correct} {
				if p != "" && !slices.Contains(svc.ResourcePrefixes, p) {
					svc.ResourcePrefixes = append(svc.ResourcePrefixes, p)
				}
			}
		}
		for _, n := range names {
			n = strings.TrimSpace(n)
			if len(n) >= 2 && !slices.Contains(svc.Names, n) {
				svc.Names = append(svc.Names, n)
			}
		}

		m.add(svc)
	}

	sort.Slice(m.prefixes, func(i, j int) bool { return len(m.prefixes[i].prefix) > len(m.prefixes[j].prefix) })
	return m, nil
}

// New builds a map directly; used by tests.
func New(svcs ...*Service) *Map {
	m := &Map{byPackage: map[string]*Service{}, byModule: map[string]*Service{}}
	for _, s := range svcs {
		m.add(s)
	}
	sort.Slice(m.prefixes, func(i, j int) bool { return len(m.prefixes[i].prefix) > len(m.prefixes[j].prefix) })
	return m
}

func (m *Map) add(svc *Service) {
	if svc.Module != "" {
		m.byModule[svc.Module] = svc
	}
	if svc.NotImplemented {
		return
	}
	m.byPackage[svc.Package] = svc
	for _, p := range svc.ResourcePrefixes {
		m.prefixes = append(m.prefixes, prefixEntry{prefix: p, svc: svc})
	}
	for _, n := range svc.Names {
		// Case-sensitive, whole-word match. Names are proper nouns ("FSx", "S3 Control"), so case sensitivity avoids most
		// collisions with ordinary words ("config", "batch").
		re := regexp.MustCompile(`(?:^|[^A-Za-z0-9])` + regexp.QuoteMeta(n) + `(?:$|[^A-Za-z0-9])`)
		m.names = append(m.names, nameEntry{re: re, name: n, svc: svc})
	}
}

// splitHumanFriendly turns "AMP (Managed Prometheus)" into ["AMP", "Managed Prometheus"] and drops leading "AWS"/"Amazon".
func splitHumanFriendly(s string) []string {
	var out []string
	main, paren, _ := strings.Cut(s, "(")
	for _, part := range []string{main, strings.TrimSuffix(paren, ")")} {
		part = strings.TrimSpace(part)
		for _, p := range []string{"AWS ", "Amazon "} {
			part = strings.TrimPrefix(part, p)
		}
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ByPackage returns the service for a provider package, or nil.
func (m *Map) ByPackage(pkg string) *Service {
	return m.byPackage[pkg]
}

// ByModule returns the service using an AWS SDK for Go v2 service module, including services marked not implemented,
// or nil.
func (m *Map) ByModule(module string) *Service {
	return m.byModule[module]
}

// ByTFName returns the service owning a Terraform type name such as aws_fsx_file_system, or nil.
func (m *Map) ByTFName(name string) *Service {
	for _, p := range m.prefixes {
		if strings.HasPrefix(name, p.prefix) {
			return p.svc
		}
	}
	return nil
}

// Match is a detected service with the text that triggered it.
type Match struct {
	Service *Service
	Name    string
}

// DetectInText returns services whose names appear in text, in package order.
func (m *Map) DetectInText(text string) []Match {
	found := map[string]Match{}
	for _, e := range m.names {
		if _, ok := found[e.svc.Package]; ok {
			continue
		}
		if e.re.MatchString(text) {
			found[e.svc.Package] = Match{Service: e.svc, Name: e.name}
		}
	}
	out := make([]Match, 0, len(found))
	for _, v := range found {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service.Package < out[j].Service.Package })
	return out
}

// Len returns the number of services.
func (m *Map) Len() int {
	return len(m.byPackage)
}
