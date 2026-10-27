// Package install resolves a language's install, declared in its
// config.json (docs/design.md §19), into what to download and run on one
// platform: an archive, a vendor's installer, packages from the system's
// package manager, or a command of another tool hecatron installs.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Spec is a language's "install".
type Spec struct {
	Archive  *Archive            `json:"archive,omitempty"`
	Vendor   *Vendor             `json:"vendor,omitempty"`
	Packages map[string][]string `json:"packages,omitempty"` // package manager → packages
	Via      *Via                `json:"via,omitempty"`
	Requires []string            `json:"requires,omitempty"` // languages installed first
	// Tools are other programs the language's installs and hooks need, by
	// name, each installed the same ways.
	Tools map[string]Spec `json:"tools,omitempty"`
	// Version is a tool's pinned version (a language's comes from version
	// resolution).
	Version string `json:"version,omitempty"`
	// Platforms replaces the install on a platform ("windows-aarch64") its
	// usual source doesn't publish for.
	Platforms map[string]Spec `json:"platforms,omitempty"`
}

// Archive is a release archive to unpack into the version's directory.
type Archive struct {
	URL      string            `json:"url,omitempty"` // template: {version} {os} {arch} {ext}
	OS       map[string]string `json:"os,omitempty"`  // hecatron's OS name → the URL's
	Arch     map[string]string `json:"arch,omitempty"`
	Ext      map[string]string `json:"ext,omitempty"` // by OS; "default" for the rest
	Checksum string            `json:"checksum,omitempty"`
	// Resolver names Go code that finds the URL and checksum where a
	// template can't (a release's build number, a paged API).
	Resolver string            `json:"resolver,omitempty"`
	Strip    int               `json:"strip,omitempty"` // leading path components to drop
	Bin      map[string]string `json:"bin,omitempty"`   // by OS ("default"): the directory to put on PATH
}

// Vendor is an installer the toolchain's own project publishes.
type Vendor struct {
	URL  map[string]string   `json:"url"` // by OS ("unix", "windows")
	Run  map[string][]string `json:"run"` // by OS; {file} is the downloaded installer
	Arch map[string]string   `json:"arch,omitempty"`
	Env  map[string]string   `json:"env,omitempty"`
	Bin  string              `json:"bin,omitempty"`
}

// Via installs the language with another tool hecatron installs first.
type Via struct {
	Tool string   `json:"tool"`
	Run  []string `json:"run"`
}

// Load reads a language's install spec from languages/<lang>/config.json.
func Load(languages, lang string) (*Spec, string, error) {
	data, err := os.ReadFile(filepath.Join(languages, lang, "config.json"))
	if err != nil {
		return nil, "", err
	}
	var config struct {
		Version string `json:"version"`
		Install *Spec  `json:"install"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, "", fmt.Errorf("%s: %w", lang, err)
	}
	if config.Install == nil {
		return nil, "", fmt.Errorf("%s: no install", lang)
	}
	return config.Install, config.Version, nil
}
