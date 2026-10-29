// Package install reads a language's declarative install
// (languages/<lang>/install.json, docs/design.md §19 "Installs as data"),
// says whether this binary can carry it out, plans it for a project and a
// platform, and applies the plan: system packages batched into one
// package-manager call, guarded steps, receipts of what it installed, and
// uninstall from those receipts.
//
// resolve.go is the prototype's resolution of archive, vendor and via
// installs (#72), which the executor doesn't run yet.
package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileName is a definition's install file, beside its config.json. A
// project's own copy replaces the built-in one whole (§7), as an install.sh
// override does.
const FileName = "install.json"

// SchemaVersion is the newest install.json schema this binary reads.
const SchemaVersion = 1

// Definition is an install.json.
type Definition struct {
	Schema int `json:"schema"`
	// Requires names languages installed first (Elixir needs Erlang). Read
	// by the prototype's Resolve; the executor doesn't install them yet.
	Requires  []string   `json:"requires,omitempty"`
	Toolchain Toolchain  `json:"toolchain"`
	Packages  []Packages `json:"packages,omitempty"`
	Tools     []Tool     `json:"tools,omitempty"`
	Steps     []RunStep  `json:"steps,omitempty"`
	Uninstall Uninstall  `json:"uninstall,omitempty"`
	// AptSources are signed apt repositories the packages come from on a
	// distribution whose own are too old (aptsource.go).
	AptSources []AptSource `json:"aptSources,omitempty"`
	// Links are the default version's commands, linked into hecatron's bin
	// directory (LinksDir) for interactive shells.
	Links []string `json:"links,omitempty"`

	keys map[string]json.RawMessage // every key as written, for Unsupported
	raw  []byte                     // the file, for CacheKey
}

// ToolchainsHome is where the executor puts the toolchains it installs
// itself (downloads, not system packages), one directory per language:
// the user's own, so installing needs no root and CI can cache and restore
// it (design.md §11).
const ToolchainsHome = "~/.local/share/hecatron/toolchains"

// InstallsIntoToolchainsHome says whether the executor installs the
// toolchain under ToolchainsHome: versions it installs itself ("single",
// "managed"), not the package manager's ("system").
func (d *Definition) InstallsIntoToolchainsHome() bool {
	return d.Toolchain.Versions == "single" || d.Toolchain.Versions == "managed"
}

// CacheKey identifies what the executor would install for the definition
// and the default version: a short hash of both, for a CI cache's key (a
// project's own pins and install.json are hashed by the CI itself).
func (d *Definition) CacheKey(defaultVersion string) string {
	sum := sha256.Sum256(append(append(append([]byte(nil), d.raw...), 0), defaultVersion...))
	return hex.EncodeToString(sum[:])[:16]
}

// Toolchain is what provides the language: one of its install kinds, and
// how its versions are installed.
type Toolchain struct {
	// Versions: "system" (whatever the package manager has), "single" (the
	// default only) or "managed" (each resolved version, side by side).
	Versions string `json:"versions,omitempty"`
	Spec
}

// Spec is one way of installing a toolchain or tool.
type Spec struct {
	Archive  *Archive            `json:"archive,omitempty"`
	Vendor   *Vendor             `json:"vendor,omitempty"`
	Packages map[string][]string `json:"packages,omitempty"` // package manager → packages
	Via      *Via                `json:"via,omitempty"`
	// Platforms replaces the install on a platform ("windows-aarch64") its
	// usual source doesn't publish for.
	Platforms map[string]Spec `json:"platforms,omitempty"`
}

// Packages are system packages installed when a condition holds: by
// package manager ("apt": [...], "brew": [...]), with "when" (install only
// if it holds for some component) and "unless" (already satisfied: the
// check, instead of asking the package manager).
type Packages struct {
	Managers map[string][]string
	When     *Condition
	Unless   *Condition
	Problem  string // the doctor's message while Unless fails
}

// UnmarshalJSON reads a Packages entry: its managers are its other keys.
func (p *Packages) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Managers = map[string][]string{}
	for key, value := range raw {
		var err error
		switch key {
		case "when":
			err = json.Unmarshal(value, &p.When)
		case "unless":
			err = json.Unmarshal(value, &p.Unless)
		case "problem":
			err = json.Unmarshal(value, &p.Problem)
		default:
			if !contains(knownManagers, key) {
				return fmt.Errorf("packages: unknown key %q", key)
			}
			var names []string
			err = json.Unmarshal(value, &names)
			p.Managers[key] = names
		}
		if err != nil {
			return fmt.Errorf("packages %s: %w", key, err)
		}
	}
	return nil
}

var knownManagers = []string{"apt", "dnf", "brew", "winget", "scoop"}

// Tool is a program the language's hooks need besides the toolchain
// (ShellCheck, zef): installed when its "when" holds and its check fails,
// and reported by the doctor the same way.
type Tool struct {
	Name string `json:"name"`
	// Use names a shared tool (languages/_tools.json, a later stage).
	Use      string              `json:"use,omitempty"`
	Version  string              `json:"version,omitempty"`
	When     *Condition          `json:"when,omitempty"`
	Packages map[string][]string `json:"packages,omitempty"`
	Archive  *Archive            `json:"archive,omitempty"`
	// Check says the tool is there (its command on PATH, a module loads).
	Check *Condition `json:"check,omitempty"`
}

// RunStep is a command run after the packages are in, when "when" holds
// and its check fails; problem is what the doctor says while it fails.
// Its run and check may use {data}, hecatron's data directory, and its run
// {file}, its download.
type RunStep struct {
	Name    string     `json:"name,omitempty"`
	Run     []string   `json:"run"`
	When    *Condition `json:"when,omitempty"`
	Check   *Condition `json:"check"`
	Problem string     `json:"problem"`
	// Download is a file the step runs (an installer script), fetched into
	// the downloads cache and verified against its SHA-256.
	Download *StepDownload `json:"download,omitempty"`
	// Creates is the directory the step makes, inside hecatron's data
	// directory: removed before the step runs again (an interrupted run's
	// leftovers) and by uninstall.
	Creates string `json:"creates,omitempty"`
}

// StepDownload is a step's file: its URL and its mandatory SHA-256.
type StepDownload struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Uninstall adjusts what uninstall derives from the receipts.
type Uninstall struct {
	// Keep are packages never removed, though the receipt lists them:
	// shared with languages whose installers are still scripts, whose
	// packages no data names.
	Keep    []string `json:"keep,omitempty"`
	Because string   `json:"because,omitempty"`
}

// Archive is a release archive to unpack into the version's directory.
type Archive struct {
	URL  string            `json:"url,omitempty"` // template: {version} {os} {arch} {ext}
	OS   map[string]string `json:"os,omitempty"`  // hecatron's OS name → the URL's
	Arch map[string]string `json:"arch,omitempty"`
	Ext  map[string]string `json:"ext,omitempty"` // by OS; "default" for the rest
	// A download is verified against one of: SHA256, inline by platform
	// ("linux-x86_64"); Checksum, a URL template serving it (a sidecar:
	// its first word); ChecksumIndex, a JSON index listing it.
	SHA256        map[string]string `json:"sha256,omitempty"`
	Checksum      string            `json:"checksum,omitempty"`
	ChecksumIndex *ChecksumIndex    `json:"checksumIndex,omitempty"`
	// Resolver names Go code that finds the URL and checksum where a
	// template can't (a release's build number, a paged API).
	Resolver string            `json:"resolver,omitempty"`
	Format   string            `json:"format,omitempty"` // tar.gz, tar.xz, zip; else from the URL
	Strip    int               `json:"strip,omitempty"`  // leading path components to drop
	Bin      map[string]string `json:"bin,omitempty"`    // by OS ("default"): the directory to put on PATH
	// Check says the unpacked toolchain works ({home}, {bin}): a restored
	// or interrupted one that doesn't is installed again.
	Check *Condition `json:"check,omitempty"`
}

// ChecksumIndex is a JSON document listing downloads' SHA-256s: Path is the
// keys leading to one, each a template (["{version}", "{arch}-{os}",
// "shasum"]), since a key can hold dots. URLPath, when set, leads to the
// download's own URL in the same index, which then replaces the archive's
// template: a project that renamed its downloads (Zig's before 0.14.1 are
// zig-linux-x86_64-*, later ones zig-x86_64-linux-*) still lists them all.
type ChecksumIndex struct {
	URL     string   `json:"url"`
	Path    []string `json:"path"`
	URLPath []string `json:"urlPath,omitempty"`
}

// Vendor is an installer the toolchain's own project publishes.
type Vendor struct {
	URL  map[string]string   `json:"url"` // by OS ("unix", "windows")
	Run  map[string][]string `json:"run"` // by OS; {file} is the downloaded installer
	Arch map[string]string   `json:"arch,omitempty"`
	Env  map[string]string   `json:"env,omitempty"`
	Bin  string              `json:"bin,omitempty"`
}

// Via installs the language with a tool the definition lists.
type Via struct {
	Tool string   `json:"tool"`
	Run  []string `json:"run"`
}

// Parse reads an install.json. Unknown keys are kept for Unsupported, so a
// definition written for a newer binary is declined, not misread.
func Parse(data []byte) (*Definition, error) {
	def := &Definition{raw: data}
	if err := json.Unmarshal(data, &def.keys); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(def); err != nil {
		return nil, err
	}
	if def.Schema == 0 {
		return nil, errors.New(`no "schema"`)
	}
	return def, def.validate()
}

// LoadFile reads the install.json at path.
func LoadFile(path string) (*Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	def, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return def, nil
}

func (d *Definition) validate() error {
	for i, tool := range d.Tools {
		if tool.Name == "" && tool.Use == "" {
			return fmt.Errorf("tools[%d]: no name", i)
		}
		if tool.Use == "" && tool.Check == nil {
			return fmt.Errorf(`tools[%d]: a tool needs a "check"`, i)
		}
	}
	for i, step := range d.Steps {
		if len(step.Run) == 0 || step.Check == nil || step.Problem == "" {
			return fmt.Errorf(`steps[%d]: a step needs "run", "check" and "problem"`, i)
		}
		if step.Download != nil && (step.Download.URL == "" || !hexSHA256.MatchString(step.Download.SHA256)) {
			return fmt.Errorf(`steps[%d]: a download needs a "url" and its "sha256"`, i)
		}
		if step.Creates != "" && !strings.HasPrefix(step.Creates, "{data}/") {
			return fmt.Errorf(`steps[%d]: "creates" must be inside {data}`, i)
		}
	}
	for i := range d.AptSources {
		if err := d.AptSources[i].validate(); err != nil {
			return fmt.Errorf("aptSources[%d]: %w", i, err)
		}
	}
	conditions := []*Condition{}
	for i := range d.Packages {
		conditions = append(conditions, d.Packages[i].When, d.Packages[i].Unless)
	}
	for i := range d.Tools {
		conditions = append(conditions, d.Tools[i].When, d.Tools[i].Check)
	}
	for i := range d.Steps {
		conditions = append(conditions, d.Steps[i].When, d.Steps[i].Check)
	}
	for _, condition := range conditions {
		if condition == nil {
			continue
		}
		if err := condition.validate(); err != nil {
			return err
		}
	}
	return nil
}

// supportedKeys are the install.json keys this binary carries out. The
// others ("family", "links", "installed", "deps", "hook", "installOf"...)
// belong to later stages.
var supportedKeys = []string{"schema", "toolchain", "packages", "tools", "steps", "uninstall", "links", "aptSources"}

// Unsupported says why this binary can't carry out the definition on
// manager's platform ("" when it can): a newer schema, a key or install
// kind it doesn't run yet, or no package for the platform's package
// manager. Its caller then runs install.sh.
func (d *Definition) Unsupported(manager string, platform Platform) string {
	if d.Schema > SchemaVersion {
		return fmt.Sprintf("schema %d is newer than this binary reads", d.Schema)
	}
	var unknown []string
	for key := range d.keys {
		if !contains(supportedKeys, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Sprintf("it uses %s, which this binary can't install yet", quoteAll(unknown))
	}
	if extra := d.unknownNestedKeys(); len(extra) > 0 {
		return fmt.Sprintf("it uses %s, which this binary can't install yet", quoteAll(extra))
	}
	t := d.Toolchain
	switch {
	case t.Vendor != nil, t.Via != nil, t.Platforms != nil:
		return "its toolchain isn't system packages or a release download, which this binary can't install yet"
	case t.Archive != nil:
		if why := t.Archive.unsupported(t.Versions, platform); why != "" {
			return why
		}
	case t.Versions != "system":
		return `a toolchain from system packages has "versions": "system"`
	case len(t.Packages[manager]) == 0 && manager != "":
		return "it has no " + manager + " packages"
	}
	if manager == "" && len(d.allPackages()) > 0 {
		return "this platform has no package manager hecatron uses"
	}
	for _, tool := range d.Tools {
		if tool.Use != "" || tool.Archive != nil || tool.Version != "" {
			return fmt.Sprintf("tool %s isn't system packages, which this binary can't install yet", tool.label())
		}
	}
	return ""
}

// allPackages are every package the definition names, for any manager.
func (d *Definition) allPackages() []string {
	var names []string
	for _, list := range d.Toolchain.Packages {
		names = append(names, list...)
	}
	for _, entry := range d.Packages {
		for _, list := range entry.Managers {
			names = append(names, list...)
		}
	}
	for _, tool := range d.Tools {
		for _, list := range tool.Packages {
			names = append(names, list...)
		}
	}
	return names
}

// archiveKeys are the keys this binary reads in an archive.
var archiveKeys = []string{"url", "os", "arch", "ext", "sha256", "checksum", "checksumIndex", "resolver", "format",
	"strip", "bin", "check"}

// unsupported says why this binary can't install the archive on platform.
func (a *Archive) unsupported(versions string, platform Platform) string {
	switch {
	case versions != "single" && versions != "managed":
		return `a release download has "versions": "single" or "managed"`
	case a.Resolver != "":
		return "its download needs resolver " + a.Resolver + ", which this binary doesn't run yet"
	case a.OS[platform.OS] == "" || a.Arch[platform.Arch] == "":
		return "it has no download for " + platform.Key()
	case a.SHA256[platform.Key()] == "" && a.Checksum == "" && a.ChecksumIndex == nil:
		return "its download for " + platform.Key() + " has no checksum"
	case a.Check == nil:
		return `its download has no "check" saying the toolchain works`
	}
	if _, err := a.format(platform); err != nil {
		return err.Error()
	}
	return ""
}

// format is how the download unpacks: tar.gz, tar.xz or zip.
func (a *Archive) format(platform Platform) (string, error) {
	format := a.Format
	if format == "" {
		url := a.URL
		if ext := byOS(a.Ext, platform.OS); ext != "" {
			url = strings.ReplaceAll(url, "{ext}", ext)
		}
		for _, known := range []string{"tar.gz", "tgz", "tar.xz", "zip"} {
			if strings.HasSuffix(url, "."+known) {
				format = known
			}
		}
	}
	switch format {
	case "tgz":
		return "tar.gz", nil
	case "tar.gz", "tar.xz", "zip":
		return format, nil
	}
	return "", fmt.Errorf("its download's format %q isn't one this binary unpacks", format)
}

// nestedKeys are the keys this binary reads inside the toolchain, a tool
// and a step.
var nestedKeys = map[string][]string{
	"toolchain":  {"versions", "packages", "archive", "vendor", "via", "platforms"},
	"tools":      {"name", "use", "version", "when", "packages", "archive", "check"},
	"steps":      {"name", "run", "when", "check", "problem", "download", "creates"},
	"aptSources": {"name", "distro", "uri", "suite", "components", "key", "fingerprint", "packages", "replaces"},
}

func (d *Definition) unknownNestedKeys() []string {
	var unknown []string
	for section, known := range nestedKeys {
		raw, ok := d.keys[section]
		if !ok {
			continue
		}
		var entries []map[string]json.RawMessage
		if section == "toolchain" {
			var entry map[string]json.RawMessage
			_ = json.Unmarshal(raw, &entry)
			entries = append(entries, entry)
		} else {
			_ = json.Unmarshal(raw, &entries)
		}
		for _, entry := range entries {
			for key := range entry {
				if !contains(known, key) && !contains(unknown, section+"."+key) {
					unknown = append(unknown, section+"."+key)
				}
			}
		}
	}
	if raw, ok := d.keys["toolchain"]; ok {
		var toolchain struct {
			Archive map[string]json.RawMessage `json:"archive"`
		}
		_ = json.Unmarshal(raw, &toolchain)
		for key := range toolchain.Archive {
			if !contains(archiveKeys, key) {
				unknown = append(unknown, "toolchain.archive."+key)
			}
		}
	}
	sort.Strings(unknown)
	return unknown
}

func (t Tool) label() string {
	if t.Name != "" {
		return t.Name
	}
	return t.Use
}

func quoteAll(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = `"` + item + `"`
	}
	return strings.Join(quoted, ", ")
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}

// Load reads a built-in language's install for the prototype's Resolve:
// its install.json, else the "install" key the prototype put in
// config.json (Elixir's, until its rewrite settles), with the default
// version from config.json.
func Load(languages, lang string) (*Definition, string, error) {
	data, err := os.ReadFile(filepath.Join(languages, lang, "config.json"))
	if err != nil {
		return nil, "", err
	}
	var config struct {
		Version string          `json:"version"`
		Install json.RawMessage `json:"install"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, "", fmt.Errorf("%s: %w", lang, err)
	}
	def, err := LoadFile(filepath.Join(languages, lang, FileName))
	if err == nil {
		return def, config.Version, nil
	}
	if !errors.Is(err, os.ErrNotExist) || config.Install == nil {
		return nil, "", err
	}
	var prototype struct {
		Spec
		Requires []string `json:"requires"`
	}
	if err := json.Unmarshal(config.Install, &prototype); err != nil {
		return nil, "", fmt.Errorf("%s: %w", lang, err)
	}
	return &Definition{Schema: 1, Requires: prototype.Requires, Toolchain: Toolchain{Spec: prototype.Spec}}, config.Version, nil
}
