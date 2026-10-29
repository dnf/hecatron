// Package licenses is hct licenses (docs/guide/licenses.md): every dependency's
// license, in every language and component, from OSV-Scanner's report on
// the lock files (its licenses come from deps.dev) and, where that has
// none, the lock file itself (Composer's); normalised to SPDX expressions,
// rolled up per license, and checked against the project's policy.
package licenses

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/spdx"
)

// Ecosystems are the package registries (OSV-Scanner's names) whose
// packages a language's lock files list.
var Ecosystems = map[string][]string{
	"typescript": {"npm"}, "javascript": {"npm"},
	"python": {"PyPI"},
	"go":     {"Go"},
	"rust":   {"crates.io"},
	"java":   {"Maven"}, "kotlin": {"Maven"}, "scala": {"Maven"}, "groovy": {"Maven"}, "clojure": {"Maven"},
	"csharp": {"NuGet"}, "fsharp": {"NuGet"}, "vbnet": {"NuGet"},
	"ruby":   {"RubyGems"},
	"php":    {"Packagist"},
	"elixir": {"Hex"}, "erlang": {"Hex"}, "gleam": {"Hex"},
	"dart":    {"Pub"},
	"swift":   {"SwiftURL"},
	"r":       {"CRAN"},
	"haskell": {"Hackage"},
	"c":       {"ConanCenter"}, "cpp": {"ConanCenter"},
}

// covered are the registries whose licenses hecatron reads: deps.dev's
// (through OSV-Scanner), and Packagist's from composer.lock.
var covered = map[string]bool{
	"npm": true, "PyPI": true, "Go": true, "crates.io": true, "Maven": true, "NuGet": true, "RubyGems": true,
	"Packagist": true,
}

var registryNames = map[string]string{
	"Hex": "Hex", "Pub": "pub.dev", "SwiftURL": "Swift packages", "CRAN": "CRAN", "Hackage": "Hackage",
	"ConanCenter": "Conan",
}

// notCoveredReason is why a language's licenses aren't read, or "".
func notCoveredReason(lang string) string {
	ecosystems := Ecosystems[lang]
	if len(ecosystems) == 0 {
		return "no package registry hecatron reads licenses from"
	}
	for _, ecosystem := range ecosystems {
		if covered[ecosystem] {
			return ""
		}
	}
	return "deps.dev, where OSV-Scanner finds licenses, has none for " + registryNames[ecosystems[0]]
}

// Covers says whether hecatron reads the language's licenses.
func Covers(lang string) bool { return notCoveredReason(lang) == "" }

// Target is one language in one component: what to report on.
type Target struct {
	Lang, Component, Dir string // Dir is absolute
}

// Package is one dependency, as one component in one language uses it.
type Package struct {
	Ecosystem  string   `json:"ecosystem"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	License    string   `json:"license"`    // a canonical SPDX expression; "" when unknown
	Declared   []string `json:"declared"`   // what the registry or lock file says
	Dependency string   `json:"dependency"` // direct, transitive, or "" when the lock file doesn't say
	Dev        bool     `json:"dev"`        // a development or test dependency
	Language   string   `json:"language"`
	Component  string   `json:"component"`
	Source     string   `json:"source"` // the lock file or manifest, relative to the project root

	expression *spdx.Expression
}

// Key is the package as licenses.ignore names it: its ecosystem in lower
// case, a colon, its name (npm:caniuse-lite).
func (p Package) Key() string { return strings.ToLower(p.Ecosystem) + ":" + p.Name }

// Unknown says whether the package's license is unknown: no metadata, or
// not an SPDX expression.
func (p Package) Unknown() bool { return p.License == "" }

// NotCovered is a language, or one component of it, whose licenses aren't
// read, and why.
type NotCovered struct {
	Language  string `json:"language"`
	Component string `json:"component,omitempty"` // "" for every component
	Reason    string `json:"reason"`
}

// Report is what hct licenses found.
type Report struct {
	Packages   []Package    `json:"packages"`
	Licenses   []Rollup     `json:"licenses"`
	NotCovered []NotCovered `json:"notCovered"`
	Warnings   []string     `json:"warnings"`
	Policy     *Verdict     `json:"policy,omitempty"`
}

// Rollup is one license and who uses it.
type Rollup struct {
	License    string   `json:"license"`
	Packages   int      `json:"packages"`
	Languages  []string `json:"languages"`
	Components []string `json:"components"`
}

// Scanner runs OSV-Scanner's license report on a directory: its JSON, or
// nil when it finds no lock file or manifest to read.
type Scanner func(dir string) ([]byte, error)

type osvReport struct {
	Results []struct {
		Source struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"source"`
		Packages []struct {
			Package struct {
				Name      string `json:"name"`
				Version   string `json:"version"`
				Ecosystem string `json:"ecosystem"`
			} `json:"package"`
			Licenses         []string `json:"licenses"`
			DependencyGroups []string `json:"dependency_groups"`
		} `json:"packages"`
	} `json:"results"`
}

// Gather reports on targets: OSV-Scanner once per directory, each lock
// file's packages given to the deepest target directory holding it and to
// the language there whose registry they come from. scan may be nil when
// no target is covered.
func Gather(root string, targets []Target, scan Scanner) (*Report, error) {
	return gatherPackages(root, targets, scan, false)
}

// GatherAll is Gather for a bill of materials (hct sbom): the packages of
// the registries whose licenses hecatron doesn't read are listed too, their
// licenses unknown. Languages without a package registry still aren't.
func GatherAll(root string, targets []Target, scan Scanner) (*Report, error) {
	return gatherPackages(root, targets, scan, true)
}

// read says whether a language's lock files are read: those whose licenses
// hecatron reads, or, for a bill of materials, any with a registry.
func read(lang string, all bool) bool {
	return Covers(lang) || (all && len(Ecosystems[lang]) > 0)
}

func gatherPackages(root string, targets []Target, scan Scanner, all bool) (*Report, error) {
	report := &Report{}
	var dirs []string
	byDir := map[string][]Target{}
	reported := map[string]bool{}
	for _, t := range targets {
		if reason := notCoveredReason(t.Lang); reason != "" {
			if !reported[t.Lang] {
				reported[t.Lang] = true
				report.NotCovered = append(report.NotCovered, NotCovered{Language: t.Lang, Reason: reason})
			}
			if !read(t.Lang, all) {
				continue
			}
		}
		if _, seen := byDir[t.Dir]; !seen {
			dirs = append(dirs, t.Dir)
		}
		byDir[t.Dir] = append(byDir[t.Dir], t)
	}
	found := map[Target]bool{}
	approximate := map[string]bool{}
	scans := scanAll(dirs, scan)
	for i, dir := range dirs {
		data, err := scans[i].data, scans[i].err
		if err != nil {
			return nil, err
		}
		var osv osvReport
		if len(data) > 0 {
			if err := json.Unmarshal(data, &osv); err != nil {
				return nil, fmt.Errorf("couldn't read OSV-Scanner's report on %s: %v", relative(root, dir), err)
			}
		}
		for _, result := range osv.Results {
			source := result.Source.Path
			if !filepath.IsAbs(source) {
				source = filepath.Join(dir, source)
			}
			if deepest(source, dirs) != dir {
				continue // that directory's own scan reports it
			}
			lock := readLock(source, result.Source.Type)
			for _, p := range result.Packages {
				target, ok := targetFor(byDir[dir], p.Package.Ecosystem, all)
				if !ok || lock.isRoot(p.Package.Name, p.Package.Version) {
					continue
				}
				found[target] = true
				if result.Source.Type != "lockfile" {
					approximate[target.Lang+"\x00"+target.Component+"\x00"+relative(root, source)] = true
				}
				declared := p.Licenses
				switch {
				case p.Package.Ecosystem == "Packagist":
					declared = lock.licenses[p.Package.Name]
				case !covered[p.Package.Ecosystem]:
					declared = nil // deps.dev has none for it, only "UNKNOWN"
				}
				pkg := Package{Ecosystem: p.Package.Ecosystem, Name: p.Package.Name, Version: p.Package.Version,
					Declared: append([]string{}, declared...), Dependency: lock.dependency(p.Package.Name, result.Source.Type),
					Dev: isDev(p.DependencyGroups), Language: target.Lang, Component: target.Component, Source: relative(root, source)}
				pkg.expression = Normalize(declared)
				if pkg.expression != nil {
					pkg.License = pkg.expression.String()
				}
				report.Packages = append(report.Packages, pkg)
			}
		}
	}
	for _, t := range targets {
		if read(t.Lang, all) && !found[t] {
			report.NotCovered = append(report.NotCovered, NotCovered{Language: t.Lang, Component: t.Component,
				Reason: "no lock file OSV-Scanner reads in " + relative(root, t.Dir)})
		}
	}
	var keys []string
	for key := range approximate {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"%s: %s: %s isn't a lock file, so its dependencies are the versions it resolves to today: approximate",
			parts[1], parts[0], parts[2]))
	}
	report.Packages = merge(report.Packages)
	report.Licenses = rollup(report.Packages)
	return report, nil
}

type scanned struct {
	data []byte
	err  error
}

// scanAll runs the scanner on each directory, four at a time (each waits
// on deps.dev more than it computes).
func scanAll(dirs []string, scan Scanner) []scanned {
	results := make([]scanned, len(dirs))
	limit := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, dir := range dirs {
		wg.Add(1)
		limit <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-limit }()
			data, err := scan(dir)
			results[i] = scanned{data, err}
		}()
	}
	wg.Wait()
	return results
}

// Normalize is the license a package declares, as one SPDX expression:
// several declared licenses are alternatives (RubyGems', npm's and
// Packagist's lists mean the user may choose); nil when there's none or one
// isn't an SPDX expression.
func Normalize(declared []string) *spdx.Expression {
	var terms []*spdx.Expression
	for _, text := range declared {
		e, err := spdx.Parse(text)
		if err != nil {
			return nil
		}
		terms = append(terms, e)
	}
	switch len(terms) {
	case 0:
		return nil
	case 1:
		return terms[0]
	}
	text := make([]string, len(terms))
	for i, term := range terms {
		text[i] = "(" + term.String() + ")"
	}
	e, err := spdx.Parse(strings.Join(text, " OR "))
	if err != nil {
		return nil
	}
	return e
}

func isDev(groups []string) bool {
	for _, group := range groups {
		if group == "dev" || group == "test" {
			return true
		}
	}
	return false
}

func targetFor(targets []Target, ecosystem string, all bool) (Target, bool) {
	for _, t := range targets {
		for _, e := range Ecosystems[t.Lang] {
			if e == ecosystem && (covered[e] || all) {
				return t, true
			}
		}
	}
	return Target{}, false
}

// deepest is the directory among dirs that holds path most closely.
func deepest(path string, dirs []string) string {
	best := ""
	for _, dir := range dirs {
		if (path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, string(filepath.Separator))+string(filepath.Separator))) &&
			len(dir) > len(best) {
			best = dir
		}
	}
	return best
}

func relative(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// merge makes each package one entry per language and component: direct
// if any lock file says so, dev only if every one does.
func merge(packages []Package) []Package {
	index := map[string]int{}
	var out []Package
	for _, p := range packages {
		key := strings.Join([]string{p.Language, p.Component, p.Ecosystem, p.Name, p.Version}, "\x00")
		i, seen := index[key]
		if !seen {
			index[key] = len(out)
			out = append(out, p)
			continue
		}
		existing := &out[i]
		if p.Dependency == "direct" || existing.Dependency == "" {
			existing.Dependency = p.Dependency
		}
		existing.Dev = existing.Dev && p.Dev
		if existing.Unknown() && !p.Unknown() {
			existing.License, existing.Declared, existing.expression = p.License, p.Declared, p.expression
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ea, eb := strings.ToLower(a.Ecosystem), strings.ToLower(b.Ecosystem); ea != eb {
			return ea < eb
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		if a.Language != b.Language {
			return a.Language < b.Language
		}
		return a.Component < b.Component
	})
	return out
}

func rollup(packages []Package) []Rollup {
	type acc struct {
		packages              map[string]bool
		languages, components map[string]bool
	}
	byLicense := map[string]*acc{}
	for _, p := range packages {
		if p.Unknown() {
			continue
		}
		a := byLicense[p.License]
		if a == nil {
			a = &acc{map[string]bool{}, map[string]bool{}, map[string]bool{}}
			byLicense[p.License] = a
		}
		a.packages[p.Key()+"@"+p.Version] = true
		a.languages[p.Language] = true
		a.components[p.Component] = true
	}
	var out []Rollup
	for license, a := range byLicense {
		out = append(out, Rollup{license, len(a.packages), sortedKeys(a.languages), sortedKeys(a.components)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Packages != out[j].Packages {
			return out[i].Packages > out[j].Packages
		}
		return out[i].License < out[j].License
	})
	return out
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Unknown are the packages whose license is unknown.
func (r *Report) Unknown() []Package {
	var out []Package
	for _, p := range r.Packages {
		if p.Unknown() {
			out = append(out, p)
		}
	}
	return out
}

// lockInfo is what a lock file or its manifest says beyond OSV-Scanner's
// report: which packages are direct dependencies, the project's own
// packages (Cargo's workspace members), and Composer's licenses.
type lockInfo struct {
	direct   map[string]bool // nil when the lock file doesn't say
	directs  map[string]string
	manifest bool // a manifest OSV-Scanner resolved: what it lists itself is direct
	roots    map[string]string
	licenses map[string][]string
}

func (l lockInfo) isRoot(name, version string) bool {
	v, ok := l.roots[name]
	return ok && (v == "" || v == version)
}

// dependency is direct or transitive, or "" when the lock file doesn't say.
func (l lockInfo) dependency(name, sourceType string) string {
	switch {
	case l.directs != nil:
		return l.directs[name]
	case l.direct != nil:
		if l.direct[name] {
			return "direct"
		}
		return "transitive"
	case l.manifest:
		if sourceType == "lockfile" {
			return "direct"
		}
		return "transitive"
	}
	return ""
}

// readLock reads what hct licenses needs from a lock file or manifest
// beyond OSV-Scanner's report; nothing for one it doesn't know.
func readLock(path, sourceType string) lockInfo {
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	switch {
	case name == "package-lock.json" || name == "npm-shrinkwrap.json" || name == "yarn.lock" ||
		name == "pnpm-lock.yaml" || name == "bun.lock":
		return lockInfo{direct: jsonKeys(filepath.Join(dir, "package.json"),
			"dependencies", "devDependencies", "optionalDependencies", "peerDependencies")}
	case name == "composer.lock":
		info := lockInfo{direct: jsonKeys(filepath.Join(dir, "composer.json"), "require", "require-dev")}
		info.licenses = composerLicenses(path)
		return info
	case name == "go.mod":
		return lockInfo{directs: goModDirect(path)}
	case name == "Cargo.lock":
		roots, direct := cargoRoots(path)
		return lockInfo{roots: roots, direct: direct}
	case name == "Gemfile.lock" || name == "gems.locked":
		return lockInfo{direct: gemfileDirect(path)}
	case name == "packages.lock.json":
		return lockInfo{directs: nugetDirect(path)}
	case name == "pom.xml" || strings.HasPrefix(name, "requirements") || name == "build.gradle" ||
		name == "build.gradle.kts" || name == "pyproject.toml" || name == "Pipfile":
		return lockInfo{manifest: true}
	}
	return lockInfo{}
}

func readJSON(path string, into any) bool {
	data, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(data, into) == nil
}

// jsonKeys are the keys of a JSON file's objects named sections; nil when
// the file can't be read.
func jsonKeys(path string, sections ...string) map[string]bool {
	var data map[string]json.RawMessage
	if !readJSON(path, &data) {
		return nil
	}
	keys := map[string]bool{}
	for _, section := range sections {
		var object map[string]any
		if json.Unmarshal(data[section], &object) == nil {
			for key := range object {
				keys[key] = true
			}
		}
	}
	return keys
}

// composerLicenses are composer.lock's "license" lists, by package.
func composerLicenses(path string) map[string][]string {
	var lock struct {
		Packages    []composerPackage `json:"packages"`
		PackagesDev []composerPackage `json:"packages-dev"`
	}
	if !readJSON(path, &lock) {
		return nil
	}
	out := map[string][]string{}
	for _, p := range append(lock.Packages, lock.PackagesDev...) {
		out[p.Name] = p.License
	}
	return out
}

type composerPackage struct {
	Name    string   `json:"name"`
	License []string `json:"license"`
}

// goModDirect is go.mod's requirements: direct, or transitive when marked
// "// indirect".
func goModDirect(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "require ("):
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		case !inBlock:
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kind := "direct"
		if strings.Contains(line, "// indirect") {
			kind = "transitive"
		}
		out[fields[0]] = kind
	}
	return out
}

// cargoRoots are Cargo.lock's own packages (without a source: the
// workspace's) and the packages they depend on.
func cargoRoots(path string) (roots map[string]string, direct map[string]bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	roots, direct = map[string]string{}, map[string]bool{}
	type entry struct {
		name, version string
		source        bool
		deps          []string
	}
	var entries []entry
	var current *entry
	inDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "[[package]]":
			entries = append(entries, entry{})
			current = &entries[len(entries)-1]
			inDeps = false
		case current == nil:
		case strings.HasPrefix(line, "name = "):
			current.name = strings.Trim(strings.TrimPrefix(line, "name = "), `"`)
		case strings.HasPrefix(line, "version = "):
			current.version = strings.Trim(strings.TrimPrefix(line, "version = "), `"`)
		case strings.HasPrefix(line, "source = "):
			current.source = true
		case strings.HasPrefix(line, "dependencies = ["):
			inDeps = !strings.HasSuffix(line, "]")
			for _, dep := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "dependencies = ["), "]"), ",") {
				if dep = strings.Trim(strings.TrimSpace(dep), `"`); dep != "" {
					current.deps = append(current.deps, strings.Fields(dep)[0])
				}
			}
		case inDeps && line == "]":
			inDeps = false
		case inDeps:
			if dep := strings.Trim(strings.TrimSuffix(line, ","), `"`); dep != "" {
				current.deps = append(current.deps, strings.Fields(dep)[0])
			}
		}
	}
	for _, e := range entries {
		if !e.source {
			roots[e.name] = e.version
			for _, dep := range e.deps {
				direct[dep] = true
			}
		}
	}
	return roots, direct
}

// gemfileDirect is Gemfile.lock's DEPENDENCIES: the Gemfile's own gems.
func gemfileDirect(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case line == "DEPENDENCIES":
			inside = true
		case inside && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    "):
			out[strings.TrimSuffix(strings.Fields(line)[0], "!")] = true
		case inside:
			inside = false
		}
	}
	return out
}

// nugetDirect is packages.lock.json's "type" of each package: Direct or
// Transitive (or Project, a project reference, read as direct).
func nugetDirect(path string) map[string]string {
	var lock struct {
		Dependencies map[string]map[string]struct {
			Type string `json:"type"`
		} `json:"dependencies"`
	}
	if !readJSON(path, &lock) {
		return nil
	}
	out := map[string]string{}
	for _, packages := range lock.Dependencies {
		for name, p := range packages {
			kind := "transitive"
			if p.Type == "Direct" || p.Type == "Project" || out[name] == "direct" {
				kind = "direct"
			}
			out[name] = kind
		}
	}
	return out
}

// ProjectTargets are the targets for a project's languages: each one's
// components (those named, or all).
func ProjectTargets(p *project.Project, langs, components []string) ([]Target, error) {
	var targets []Target
	for _, lang := range langs {
		dirs, err := p.ImplDirs(lang)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			if len(components) > 0 && !containsString(components, dir.Component) {
				continue
			}
			abs := dir.Dir
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(p.Root, abs)
			}
			targets = append(targets, Target{Lang: lang, Component: dir.Component, Dir: filepath.Clean(abs)})
		}
	}
	return targets, nil
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
