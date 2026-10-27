package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Hooks are the files that resolve to a built-in definition's hooks/
// directory rather than its top level (docs/design.md §9).
var Hooks = []string{"test.sh", "lint.sh", "build.sh", "clean.sh", "audit.sh", "format.sh", "deps.sh", "outdated.sh"}

// Definitions are every known language's config.json: the built-in keys
// with the project's on top (docs/design.md §7).
type Definitions struct {
	Names  []string // sorted
	byName map[string]*Object
}

// Get is a language's merged definition, nil when the language is unknown.
func (d *Definitions) Get(lang string) *Object { return d.byName[lang] }

// Definitions reads every language definition, built-in and the project's.
func (p *Project) Definitions() (*Definitions, error) {
	if p.definitions != nil {
		return p.definitions, nil
	}
	seen := map[string]bool{}
	for _, base := range []string{p.Home.Languages, p.projectLanguages()} {
		entries, err := os.ReadDir(base)
		if err != nil {
			if isDir(base) {
				return nil, unsupported("%v", err)
			}
			continue
		}
		for _, entry := range entries {
			if isFile(pyJoin(base, entry.Name(), "config.json")) {
				seen[entry.Name()] = true
			}
		}
	}
	defs := &Definitions{byName: map[string]*Object{}}
	for name := range seen {
		defs.Names = append(defs.Names, name)
	}
	sort.Strings(defs.Names)
	for _, lang := range defs.Names {
		merged := &Object{Values: map[string]any{}}
		for _, dir := range []string{pyJoin(p.Home.Languages, lang), pyJoin(p.projectLanguages(), lang)} {
			layer, err := readJSON(pyJoin(dir, "config.json"))
			if err != nil {
				return nil, err
			}
			if layer == nil {
				if isFile(pyJoin(dir, "config.json")) {
					return nil, unsupported("%s/config.json is null", dir)
				}
				continue
			}
			object, ok := layer.(*Object)
			if !ok {
				return nil, unsupported("%s/config.json is not an object", dir)
			}
			for _, key := range object.Keys {
				merged.set(key, object.Values[key])
			}
		}
		defs.byName[lang] = merged
	}
	p.definitions = defs
	return defs, nil
}

func (p *Project) projectLanguages() string { return pyJoin(p.Dir, "languages") }

// HasProjectDefinition reports whether the project has a directory for the
// language (`hecatron list --all` calls its definition "project").
func (p *Project) HasProjectDefinition(lang string) bool {
	return isDir(pyJoin(p.projectLanguages(), lang))
}

// Resolve is the file a language uses for filename: the project's, else the
// built-in one (hooks from its hooks/ directory), else "".
func (p *Project) Resolve(lang, filename string) string {
	projectFile := pyJoin(p.projectLanguages(), lang, filename)
	if isFile(projectFile) {
		return projectFile
	}
	sub := ""
	for _, hook := range Hooks {
		if filename == hook {
			sub = "hooks"
		}
	}
	builtin := pyJoin(pyJoin(p.Home.Languages, lang), sub, filename)
	if isFile(builtin) {
		return builtin
	}
	return ""
}

// definitionStrings is a definition's list-of-strings key (dirNames,
// detectUnless, path): empty when missing.
func (p *Project) definitionStrings(lang, key string) ([]string, error) {
	defs, err := p.Definitions()
	if err != nil {
		return nil, err
	}
	value, ok := defs.Get(lang).Get(key)
	if !ok {
		return nil, nil
	}
	return toStrings(value, lang+" "+key)
}

// DirNames are the names a language's directory goes by, its own first
// (C#'s dotnet/): the definition's "dirNames".
func (p *Project) DirNames(lang string) ([]string, error) {
	names, err := p.definitionStrings(lang, "dirNames")
	if err != nil {
		return nil, err
	}
	result := []string{lang}
	for _, name := range names {
		if name != lang {
			result = append(result, name)
		}
	}
	return result, nil
}

// findImplDir is the directory a component's code in a language is found
// in ("" when none), and the one to fall back to. A directory found by a
// language's name is its only when none of its "detectUnless" files sit
// there: a javascript/ holding a tsconfig.json is TypeScript's. A template
// without {lang} names its directory outright.
func (p *Project) findImplDir(component, lang string) (found, fallback string, err error) {
	template := strings.ReplaceAll(p.ImplTemplate(lang), "{component}", component)
	names, err := p.DirNames(lang)
	if err != nil {
		return "", "", err
	}
	var unless []string
	if strings.Contains(template, "{lang}") {
		if unless, err = p.definitionStrings(lang, "detectUnless"); err != nil {
			return "", "", err
		}
	}
	for i, name := range names {
		candidate := filepath.Clean(pyJoin(p.Root, strings.ReplaceAll(template, "{lang}", name)))
		if i == 0 {
			fallback = candidate
		}
		if found == "" {
			own, err := isOwnDir(candidate, unless)
			if err != nil {
				return "", "", err
			}
			if own {
				found = candidate
			}
		}
	}
	return found, fallback, nil
}

func isOwnDir(directory string, unless []string) (bool, error) {
	if !isDir(directory) {
		return false, nil
	}
	entries, err := os.ReadDir(directory) // read even with nothing to match, as project.py does
	if err != nil {
		return false, unsupported("%v", err)
	}
	for _, entry := range entries {
		for _, pattern := range unless {
			matched, err := fnmatch(entry.Name(), pattern)
			if err != nil {
				return false, err
			}
			if matched {
				return false, nil
			}
		}
	}
	return true, nil
}

// ImplDir is where a component's code in a language lives: the template
// filled with the first of the language's directory names that is the
// language's, else with its own name.
func (p *Project) ImplDir(component, lang string) (string, error) {
	found, fallback, err := p.findImplDir(component, lang)
	if found != "" {
		return found, err
	}
	return fallback, err
}

// ComponentDir is one component's directory for a language.
type ComponentDir struct {
	Component string
	Dir       string
}

// ImplDirs are the components implemented in a language and their
// directories, in component order.
func (p *Project) ImplDirs(lang string) ([]ComponentDir, error) {
	var result []ComponentDir
	for _, component := range p.Components {
		found, _, err := p.findImplDir(component, lang)
		if err != nil {
			return nil, err
		}
		if found != "" {
			result = append(result, ComponentDir{component, found})
		}
	}
	return result, nil
}

// ResolveName is a language's canonical name from its name or abbreviation
// (`ts`, `rs`), or "".
func (p *Project) ResolveName(name string) (string, error) {
	defs, err := p.Definitions()
	if err != nil {
		return "", err
	}
	name = strings.ToLower(name)
	if defs.Get(name) != nil {
		return name, nil
	}
	for _, lang := range defs.Names {
		if abbreviation, _ := defs.Get(lang).Get("abbreviation"); abbreviation == name {
			return lang, nil
		}
	}
	return "", nil
}
