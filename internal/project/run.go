package project

import (
	"strings"
)

// Dependency is the command a language (or a project tool) needs, from its
// definition's "dependency" (or the tool's): "" when it names none.
func (p *Project) Dependency(name string) (string, error) {
	defs, err := p.Definitions()
	if err != nil {
		return "", err
	}
	if tools, err := p.Tools(); err != nil {
		return "", err
	} else if dependency, ok := tools[name]; ok {
		return dependency, nil
	}
	return optionalField(defs.Get(name), "dependency")
}

// StaticPath are the existing directories of a language's "path" entries
// that need no resolved version (no {home}...), "~" expanded and globs
// matched, as project.py's _static_path finds them.
func (p *Project) StaticPath(lang string) ([]string, error) {
	patterns, err := p.definitionStrings(lang, "path")
	if err != nil {
		return nil, err
	}
	home, haveHome := p.lookupEnv("HOME")
	var dirs []string
	for _, pattern := range patterns {
		if strings.Contains(pattern, "{") {
			continue
		}
		expanded, err := pyExpandUser(pattern, home, haveHome)
		if err != nil {
			return nil, err
		}
		matches, err := pyGlob(expanded)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, sortedDirs(matches)...)
	}
	return dirs, nil
}

// DefinitionValue is a key of a language's merged definition as decoded
// (nil, bool, json.Number, string, []any or *Object), for keys only the Go
// core reads ("errorPatterns").
func (p *Project) DefinitionValue(lang, key string) (any, error) {
	defs, err := p.Definitions()
	if err != nil {
		return nil, err
	}
	value, _ := defs.Get(lang).Get(key)
	return value, nil
}
