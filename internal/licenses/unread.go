package licenses

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// lockFiles are the lock files of each registry whose licenses hecatron
// reads, by name (requirements files and pom.xml aren't: a requirements
// file is as often a tool's as a component's).
var lockFiles = map[string][]string{
	"npm":       {"package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "pnpm-lock.yaml", "bun.lock"},
	"PyPI":      {"uv.lock", "poetry.lock", "Pipfile.lock", "pdm.lock"},
	"Go":        {"go.mod"},
	"crates.io": {"Cargo.lock"},
	"Maven":     {"gradle.lockfile"},
	"NuGet":     {"packages.lock.json"},
	"RubyGems":  {"Gemfile.lock", "gems.locked"},
	"Packagist": {"composer.lock"},
}

// skippedDirs hold lock files that aren't the project's: installed
// packages, build output, test fixtures.
var skippedDirs = map[string]bool{
	"node_modules": true, "vendor": true, "target": true, "dist": true, "build": true, "venv": true,
	"__pycache__": true, "testdata": true, "fixtures": true,
}

// maxUnread is how many lock files outside the components a warning names.
const maxUnread = 5

// WarnUnread warns of the lock files under root, of the languages of all
// (every component's targets, not only those reported on), that are in no
// component directory of their language: hct licenses doesn't read them.
func (r *Report) WarnUnread(root string, all []Target) {
	dirsOf := map[string][]string{}
	langOf := map[string][]string{} // lock file name to the languages it belongs to
	for _, t := range all {
		if !Covers(t.Lang) {
			continue
		}
		if _, seen := dirsOf[t.Lang]; !seen {
			for _, ecosystem := range Ecosystems[t.Lang] {
				for _, name := range lockFiles[ecosystem] {
					langOf[name] = append(langOf[name], t.Lang)
				}
			}
		}
		dirsOf[t.Lang] = append(dirsOf[t.Lang], t.Dir)
	}
	if len(langOf) == 0 {
		return
	}
	unread := map[string][]string{}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || skippedDirs[entry.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		for _, lang := range langOf[entry.Name()] {
			if deepest(path, dirsOf[lang]) == "" {
				unread[lang] = append(unread[lang], relative(root, path))
			}
		}
		return nil
	})
	langs := make([]string, 0, len(unread))
	for lang := range unread {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	for _, lang := range langs {
		files := unread[lang]
		named := strings.Join(files[:min(len(files), maxUnread)], ", ")
		if len(files) > maxUnread {
			named += fmt.Sprintf(" and %d more", len(files)-maxUnread)
		}
		r.Warnings = append(r.Warnings, fmt.Sprintf(
			"%s: %s %s outside every %s component's directory, so %s packages aren't listed (hct list shows the components)",
			lang, named, isOrAre(len(files)), lang, itsOrTheir(len(files))))
	}
}

func isOrAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

func itsOrTheir(n int) string {
	if n == 1 {
		return "its"
	}
	return "their"
}
