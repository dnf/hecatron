package project

import (
	"fmt"
	"strings"
)

// CI systems hecatron generates configuration for (generated.ci).
const (
	CIGitHub = "github"
	CIGitLab = "gitlab"
)

// CISystems are the CI systems generated.ci may name.
var CISystems = []string{CIGitHub, CIGitLab}

// DefaultCIImage is the image a generated GitLab pipeline runs in.
const DefaultCIImage = "ubuntu:24.04"

// CISettings are config.json's settings for the generated CI (only the Go
// core reads them): which CI systems generated.ci names, and the "ci"
// section.
type CISettings struct {
	Systems []string // generated.ci: github (the default), gitlab or both
	Cache   bool     // ci.cache: cache toolchains and dependencies
	Image   string   // ci.image: the GitLab jobs' image
}

// CISettings reads generated.ci and the "ci" section, and says what's wrong
// with them; what's wrong falls back to the default.
func (p *Project) CISettings() (CISettings, []string) {
	settings := CISettings{Systems: []string{CIGitHub}, Image: DefaultCIImage}
	var problems []string
	ciProblem := fmt.Sprintf("generated.ci must be %s, or a list of them", strings.Join(CISystems, " or "))
	if generated, ok := p.config.Get("generated"); ok {
		if object, isObject := generated.(*Object); isObject {
			if value, present := object.Get("ci"); present {
				if systems, ok := ciSystems(value); ok {
					settings.Systems = systems
				} else {
					problems = append(problems, ciProblem)
				}
			}
		}
	}
	section, present := p.config.Get("ci")
	if !present {
		return settings, problems
	}
	object, ok := section.(*Object)
	if !ok {
		return settings, append(problems, `"ci" must be an object`)
	}
	for _, key := range object.Keys {
		value := object.Values[key]
		switch key {
		case "cache":
			if cache, isBool := value.(bool); isBool {
				settings.Cache = cache
			} else {
				problems = append(problems, "ci.cache must be true or false")
			}
		case "image":
			if image, isString := value.(string); isString && strings.TrimSpace(image) != "" {
				settings.Image = image
			} else {
				problems = append(problems, "ci.image must be an image name (ubuntu:24.04)")
			}
		default:
			problems = append(problems, fmt.Sprintf("ci.%s is not a setting (cache, image)", key))
		}
	}
	return settings, problems
}

// ciSystems reads generated.ci: a name or a list of names, each once.
func ciSystems(value any) ([]string, bool) {
	var names []any
	switch v := value.(type) {
	case string:
		names = []any{v}
	case []any:
		names = v
	default:
		return nil, false
	}
	var systems []string
	for _, name := range names {
		s, ok := name.(string)
		if !ok || !containsString(CISystems, s) || containsString(systems, s) {
			return nil, false
		}
		systems = append(systems, s)
	}
	return systems, len(systems) > 0
}

func containsString(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}

// CacheSpec is a definition's "cache": the directories CI caches for the
// language. A directory starting with ~/ is under the home directory; any
// other is relative to each component's directory.
type CacheSpec struct {
	Toolchain    []string // where its installer puts the toolchain
	Dependencies []string // where its package manager keeps what it downloads
	LockFiles    []string // what the dependencies are keyed on, in a component's directory
	// KeyFiles are what the toolchain is keyed on: the definition's pin and
	// support files that sit in a directory (not .github/workflows/*.yml).
	KeyFiles []string
}

// Empty reports whether nothing is cached.
func (c CacheSpec) Empty() bool { return len(c.Toolchain) == 0 && len(c.Dependencies) == 0 }

// Cache is a language's CacheSpec, from its merged definition.
func (p *Project) Cache(lang string) (CacheSpec, error) {
	defs, err := p.Definitions()
	if err != nil {
		return CacheSpec{}, err
	}
	definition := defs.Get(lang)
	var spec CacheSpec
	if value, ok := definition.Get("cache"); ok && value != nil {
		object, isObject := value.(*Object)
		if !isObject {
			return CacheSpec{}, &Error{lang + `: "cache" must be an object`}
		}
		for _, field := range []struct {
			key  string
			dest *[]string
		}{{"toolchain", &spec.Toolchain}, {"dependencies", &spec.Dependencies}, {"lockFiles", &spec.LockFiles}} {
			raw, present := object.Get(field.key)
			if !present {
				continue
			}
			list, err := toStrings(raw, lang+" cache."+field.key)
			if err != nil {
				return CacheSpec{}, &Error{fmt.Sprintf("%s: cache.%s must be a list of strings", lang, field.key)}
			}
			*field.dest = list
		}
	}
	for _, key := range []string{"pinFiles", "supportFiles"} {
		entries, _ := definition.Get(key)
		list, _ := entries.([]any)
		for _, entry := range list {
			object, _ := entry.(*Object)
			file, _ := object.Get("file")
			name, ok := file.(string)
			if ok && name != "" && !strings.ContainsAny(name, "/\\") && !containsString(spec.KeyFiles, name) {
				spec.KeyFiles = append(spec.KeyFiles, name)
			}
		}
	}
	return spec, nil
}

// ConfigValue is a top-level config.json key as decoded (nil, bool,
// json.Number, string, []any or *Object), for sections only the Go core
// reads.
func (p *Project) ConfigValue(key string) (any, bool) {
	return p.config.Get(key)
}

// GeneratedValue is a "generated" entry as decoded, for entries that may
// be other than a path (generated.dependabot: true).
func (p *Project) GeneratedValue(key string) (any, bool) {
	generated, _ := p.config.Get("generated")
	object, ok := generated.(*Object)
	if !ok {
		return nil, false
	}
	return object.Get(key)
}
