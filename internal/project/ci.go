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
	Secrets bool     // ci.secrets: a job running hct audit secrets
}

// CISettings reads generated.ci and the "ci" section, and says what's wrong
// with them; what's wrong falls back to the default. The jobs' time limits
// (ci.timeoutMinutes, ci.languages) are CITimeoutMinutes'.
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
		case "secrets":
			if secrets, isBool := value.(bool); isBool {
				settings.Secrets = secrets
			} else {
				problems = append(problems, "ci.secrets must be true or false")
			}
		case "image":
			if image, isString := value.(string); isString && strings.TrimSpace(image) != "" {
				settings.Image = image
			} else {
				problems = append(problems, "ci.image must be an image name (ubuntu:24.04)")
			}
		case "timeoutMinutes":
			if _, ok := ciMinutes(value); !ok {
				problems = append(problems, "ci.timeoutMinutes must be a whole number of minutes, 1 or more")
			}
		case "languages":
			problems = append(problems, ciLanguagesProblems(value)...)
		default:
			problems = append(problems, fmt.Sprintf("ci.%s is not a setting (%s)", key, ciKeys))
		}
	}
	return settings, problems
}

// ciKeys are the "ci" section's settings, as its problems list them.
const ciKeys = "cache, image, secrets, timeoutMinutes, languages"

// ciLanguagesProblems are what's wrong with ci.languages: an object of
// languages to minutes. Absent or null is fine.
func ciLanguagesProblems(value any) []string {
	if value == nil {
		return nil
	}
	languages, ok := value.(*Object)
	if !ok {
		return []string{`ci.languages must be an object of languages to minutes ({"pascal": 120})`}
	}
	var problems []string
	for _, lang := range languages.Keys {
		if _, ok := ciMinutes(languages.Values[lang]); !ok {
			problems = append(problems, fmt.Sprintf("ci.languages.%s must be a whole number of minutes, 1 or more", lang))
		}
	}
	return problems
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

// Default paths of the dependency update configuration.
const (
	DefaultDependabotPath = ".github/dependabot.yml"
	DefaultRenovatePath   = "renovate.json"
)

// UpdateSettings are config.json's settings for the dependency update
// configuration hecatron generates (only the Go core reads them).
type UpdateSettings struct {
	Dependabot string // generated.dependabot: where it goes, "" for none
	Renovate   string // generated.renovate: where it goes, "" for none
	// DependabotOptions go into every Dependabot "updates" entry (the
	// "dependabot" section but its "ecosystems"); DependabotEcosystems
	// into those of one package ecosystem.
	DependabotOptions    *Object
	DependabotEcosystems *Object
	RenovateOptions      *Object // the "renovate" section, at the top of renovate.json
}

// renovateGenerated are the renovate.json keys hecatron writes itself.
var renovateGenerated = []string{"enabledManagers", "includePaths"}

// UpdateSettings reads generated.dependabot, generated.renovate and the
// "dependabot" and "renovate" sections, and says what's wrong with them.
func (p *Project) UpdateSettings() (UpdateSettings, []string) {
	var settings UpdateSettings
	var problems []string
	for _, entry := range []struct {
		key, fallback string
		dest          *string
	}{{"dependabot", DefaultDependabotPath, &settings.Dependabot}, {"renovate", DefaultRenovatePath, &settings.Renovate}} {
		value, present := p.GeneratedValue(entry.key)
		switch v := value.(type) {
		case bool:
			if v {
				*entry.dest = entry.fallback
			}
		case string:
			if strings.TrimSpace(v) != "" {
				*entry.dest = v
				continue
			}
			problems = append(problems, fmt.Sprintf("generated.%s must be true or a path (%s)", entry.key, entry.fallback))
		case nil:
			if present {
				problems = append(problems, fmt.Sprintf("generated.%s must be true or a path (%s)", entry.key, entry.fallback))
			}
		default:
			problems = append(problems, fmt.Sprintf("generated.%s must be true or a path (%s)", entry.key, entry.fallback))
		}
	}
	if section, present := p.config.Get("dependabot"); present {
		object, ok := section.(*Object)
		if !ok {
			problems = append(problems, `"dependabot" must be an object of settings for each "updates" entry`)
		} else {
			settings.DependabotOptions = &Object{Values: map[string]any{}}
			for _, key := range object.Keys {
				value := object.Values[key]
				switch key {
				case "package-ecosystem", "directory", "directories":
					problems = append(problems, fmt.Sprintf("dependabot.%s is generated", key))
				case "ecosystems":
					ecosystems, ok := value.(*Object)
					if !ok {
						problems = append(problems, "dependabot.ecosystems must be an object of package ecosystems to settings")
						continue
					}
					for _, name := range ecosystems.Keys {
						if _, ok := ecosystems.Values[name].(*Object); !ok {
							problems = append(problems, fmt.Sprintf("dependabot.ecosystems.%s must be an object of settings", name))
						}
					}
					settings.DependabotEcosystems = ecosystems
				default:
					settings.DependabotOptions.set(key, value)
				}
			}
		}
	}
	if section, present := p.config.Get("renovate"); present {
		object, ok := section.(*Object)
		if !ok {
			problems = append(problems, `"renovate" must be an object of Renovate settings`)
		} else {
			for _, key := range renovateGenerated {
				if _, set := object.Get(key); set {
					problems = append(problems, fmt.Sprintf("renovate.%s is generated", key))
				}
			}
			settings.RenovateOptions = object
		}
	}
	return settings, problems
}

// EcosystemRule is one of a definition's "dependabot" entries: the package
// ecosystem a component's directory is updated as, when one of files is
// there (or always, without files) and none of unless is.
type EcosystemRule struct {
	Ecosystem string
	Files     []string
	Unless    []string
}

// DependabotRules are a language's "dependabot": a name, or a list of
// rules.
func (p *Project) DependabotRules(lang string) ([]EcosystemRule, error) {
	value, err := p.DefinitionValue(lang, "dependabot")
	if err != nil || value == nil {
		return nil, err
	}
	if name, ok := value.(string); ok {
		return []EcosystemRule{{Ecosystem: name}}, nil
	}
	bad := &Error{lang + `: "dependabot" must be a package ecosystem or a list of {"ecosystem", "files", "unless"}`}
	list, ok := value.([]any)
	if !ok {
		return nil, bad
	}
	var rules []EcosystemRule
	for _, raw := range list {
		object, ok := raw.(*Object)
		if !ok {
			return nil, bad
		}
		var rule EcosystemRule
		name, _ := object.Get("ecosystem")
		if rule.Ecosystem, ok = name.(string); !ok || rule.Ecosystem == "" {
			return nil, bad
		}
		for _, field := range []struct {
			key  string
			dest *[]string
		}{{"files", &rule.Files}, {"unless", &rule.Unless}} {
			if raw, present := object.Get(field.key); present {
				if *field.dest, err = toStrings(raw, lang+" dependabot "+field.key); err != nil {
					return nil, bad
				}
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// RenovateManagers are a language's "renovate": the Renovate managers that
// update its dependencies.
func (p *Project) RenovateManagers(lang string) ([]string, error) {
	return p.definitionStrings(lang, "renovate")
}

// NewObject is an empty JSON object.
func NewObject() *Object { return &Object{Values: map[string]any{}} }

// Set sets a key, keeping its place if it is already there.
func (o *Object) Set(key string, value any) { o.set(key, value) }
