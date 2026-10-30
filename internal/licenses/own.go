package licenses

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The project's own packages: OSV-Scanner lists what a lock file names,
// and lock files name the project itself and its workspace members (uv's
// editable root, npm's workspaces, Cargo's members, a Gemfile.lock's PATH
// gems, Go's replace with a local path...), whose licenses deps.dev
// doesn't have: they're reported apart, not as unknown dependencies.

// ownNames are the packages the components' manifests name, by ecosystem
// (names as packageName normalises them): package.json's and its
// workspaces', pyproject.toml's, Cargo.toml's, go.mod's module,
// composer.json's.
func ownNames(targets []Target) map[string]map[string]bool {
	names := map[string]map[string]bool{}
	add := func(ecosystem, name string) {
		if name == "" {
			return
		}
		if names[ecosystem] == nil {
			names[ecosystem] = map[string]bool{}
		}
		names[ecosystem][packageName(ecosystem, name)] = true
	}
	for _, t := range targets {
		for _, name := range npmWorkspaceNames(t.Dir) {
			add("npm", name)
		}
		add("PyPI", tomlString(filepath.Join(t.Dir, "pyproject.toml"), "project", "name"))
		add("PyPI", tomlString(filepath.Join(t.Dir, "pyproject.toml"), "tool.poetry", "name"))
		add("crates.io", tomlString(filepath.Join(t.Dir, "Cargo.toml"), "package", "name"))
		add("Go", goModule(filepath.Join(t.Dir, "go.mod")))
		var composer struct {
			Name string `json:"name"`
		}
		if readJSON(filepath.Join(t.Dir, "composer.json"), &composer) {
			add("Packagist", composer.Name)
		}
	}
	return names
}

var pypiSeparators = regexp.MustCompile(`[-_.]+`)

// packageName is a name as its registry compares them: PyPI's normalised
// (PEP 503), NuGet's and Packagist's in lower case.
func packageName(ecosystem, name string) string {
	switch ecosystem {
	case "PyPI":
		return pypiSeparators.ReplaceAllString(strings.ToLower(name), "-")
	case "NuGet", "Packagist":
		return strings.ToLower(name)
	}
	return name
}

// isLocalPath says whether a Go module name is a path: what OSV-Scanner
// names a requirement go.mod replaces with a directory (../shared).
func isLocalPath(name string) bool {
	return strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/")
}

// npmWorkspaceNames are the names of the package.json in dir and of its
// workspaces' (package.json's "workspaces", or pnpm-workspace.yaml's
// packages).
func npmWorkspaceNames(dir string) []string {
	var manifest struct {
		Name       string          `json:"name"`
		Workspaces json.RawMessage `json:"workspaces"`
	}
	var names, patterns []string
	if readJSON(filepath.Join(dir, "package.json"), &manifest) {
		names = append(names, manifest.Name)
		var list []string
		var object struct {
			Packages []string `json:"packages"`
		}
		switch {
		case json.Unmarshal(manifest.Workspaces, &list) == nil:
			patterns = list
		case json.Unmarshal(manifest.Workspaces, &object) == nil:
			patterns = object.Packages
		}
	}
	patterns = append(patterns, pnpmWorkspaces(filepath.Join(dir, "pnpm-workspace.yaml"))...)
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(pattern, "**", "*"))))
		for _, member := range matches {
			var m struct {
				Name string `json:"name"`
			}
			if readJSON(filepath.Join(member, "package.json"), &m) && m.Name != "" {
				names = append(names, m.Name)
			}
		}
	}
	return names
}

// pnpmWorkspaces are pnpm-workspace.yaml's packages patterns.
func pnpmWorkspaces(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var patterns []string
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "packages:"):
			inside = true
		case inside && strings.HasPrefix(trimmed, "- "):
			patterns = append(patterns, strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), `"'`))
		case inside && trimmed != "" && !strings.HasPrefix(trimmed, "#"):
			inside = false
		}
	}
	return patterns
}

// tomlString is a string key's value in a TOML file's [section], or "".
// Enough for a manifest's name; not a TOML parser.
func tomlString(path, section, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	current := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			current = strings.Trim(line, "[] ")
			continue
		}
		if current != section {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(name) == key {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}

func goModule(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if module, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(module), `"`)
		}
	}
	return ""
}

// npmLockOwn are package-lock.json's own packages: its workspaces (the
// entries outside node_modules) and what node_modules links to them.
func npmLockOwn(path string) map[string]string {
	var lock struct {
		Packages map[string]struct {
			Name string `json:"name"`
			Link bool   `json:"link"`
		} `json:"packages"`
	}
	if !readJSON(path, &lock) {
		return nil
	}
	own := map[string]string{}
	for key, entry := range lock.Packages {
		_, inModules, isModule := strings.Cut("/"+key, "/node_modules/")
		switch {
		case key == "":
		case !isModule && entry.Name != "":
			own[entry.Name] = ""
		case isModule && entry.Link:
			if i := strings.LastIndex(inModules, "node_modules/"); i >= 0 {
				inModules = inModules[i+len("node_modules/"):]
			}
			own[inModules] = ""
		}
	}
	return own
}

// lockPackagesFromDirectories are a uv.lock's or poetry.lock's packages
// from the project's own directories: uv's editable, virtual and
// directory sources (the project and its workspace members), Poetry's
// directory sources.
func lockPackagesFromDirectories(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	own := map[string]string{}
	name, version, section := "", "", ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "[[package]]":
			name, version, section = "", "", "package"
		case strings.HasPrefix(line, "["):
			section = strings.Trim(line, "[] ")
		case section == "package" && strings.HasPrefix(line, "name = "):
			name = strings.Trim(strings.TrimPrefix(line, "name = "), `"`)
		case section == "package" && strings.HasPrefix(line, "version = "):
			version = strings.Trim(strings.TrimPrefix(line, "version = "), `"`)
		case section == "package" && strings.HasPrefix(line, "source = {"):
			for _, kind := range []string{"editable", "virtual", "directory"} {
				if strings.Contains(line, "{ "+kind+" =") || strings.Contains(line, "{"+kind+" =") {
					own[name] = version
				}
			}
		case section == "package.source" && line == `type = "directory"`:
			own[name] = version
		}
	}
	return own
}

// gemfilePath are a Gemfile.lock's PATH gems: the gemspec's own, and gems
// from the project's directories.
func gemfilePath(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	own := map[string]string{}
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case line == "PATH":
			inside = true
		case inside && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      "):
			fields := strings.Fields(line)
			own[fields[0]] = strings.Trim(strings.Join(fields[1:], ""), "()")
		case inside && line != "" && !strings.HasPrefix(line, " "):
			inside = false
		}
	}
	return own
}

// composerPathPackages are composer.lock's packages from a path
// repository: the project's own.
func composerPathPackages(path string) map[string]string {
	var lock struct {
		Packages    []composerEntry `json:"packages"`
		PackagesDev []composerEntry `json:"packages-dev"`
	}
	if !readJSON(path, &lock) {
		return nil
	}
	own := map[string]string{}
	for _, p := range append(lock.Packages, lock.PackagesDev...) {
		if p.Dist.Type == "path" {
			own[p.Name] = ""
		}
	}
	return own
}

type composerEntry struct {
	Name string `json:"name"`
	Dist struct {
		Type string `json:"type"`
	} `json:"dist"`
}
