package project

import (
	"sort"
	"strings"
)

// Shell is what project.py's shell-env gives hecatron's shell commands
// (helpers.sh): the exported HECATRON_* variables, the language lists and
// abbreviations. Building it checks every value shell-env reads, so a
// command that sources helpers.sh fails in Go wherever it would in shell.
type Shell struct {
	Exports       [][2]string // name, value, in shell-env's order
	Known         []string    // every language with a definition
	Added         []string    // the project's languages that have one
	Disabled      []string
	Abbreviations map[string]string
	LintFormat    []string // added languages with format.sh whose format `hecatron lint` checks
	byHook        map[string][]string
}

// HookLanguages are the added languages that have a hook (test.sh,
// build.sh...), project or built-in: those an action covers.
func (s *Shell) HookLanguages(hook string) []string { return s.byHook[hook] }

// Shell builds the shell view of the project.
func (p *Project) Shell() (*Shell, error) {
	defs, err := p.Definitions()
	if err != nil {
		return nil, err
	}
	s := &Shell{Known: defs.Names, Disabled: p.Disabled, Abbreviations: map[string]string{},
		byHook: map[string][]string{}}
	for _, lang := range defs.Names {
		if !shellSafe(lang) {
			return nil, unsupported("language name %q", lang)
		}
		def := defs.Get(lang)
		for _, key := range []string{"dependency", "lintDependency"} {
			if _, err := optionalField(def, key); err != nil {
				return nil, err
			}
		}
		if _, err := p.definitionStrings(lang, "path"); err != nil {
			return nil, err
		}
		abbreviation, err := optionalField(def, "abbreviation")
		if err != nil {
			return nil, err
		}
		if abbreviation != "" {
			if !shellSafe(abbreviation) {
				return nil, unsupported("abbreviation %q", abbreviation)
			}
			s.Abbreviations[lang] = abbreviation
		}
	}
	if _, err := p.Tools(); err != nil {
		return nil, err
	}
	for _, lang := range p.Languages {
		if defs.Get(lang) != nil {
			s.Added = append(s.Added, lang)
		}
	}
	for _, lang := range s.Added {
		for _, hook := range Hooks {
			if p.Resolve(lang, hook) != "" {
				s.byHook[hook] = append(s.byHook[hook], lang)
			}
		}
	}
	for _, lang := range s.byHook["format.sh"] {
		if p.LintChecksFormat(lang) {
			s.LintFormat = append(s.LintFormat, lang)
		}
	}
	s.Exports = [][2]string{
		{"HECATRON_ROOT", p.Root},
		{"HECATRON_PROJECT", p.Name},
		{"HECATRON_COMPONENTS", strings.Join(p.Components, " ")},
		{"HECATRON_IMPL_PATH", p.ImplPath},
		{"HECATRON_LOGS", p.Logs},
		{"HECATRON_METADATA", p.Metadata},
	}
	sort.Strings(s.Disabled)
	return s, nil
}

// shellSafe is a name the shell helpers print unchanged: helpers.sh reads
// language lists a line at a time (trimming spaces), echoes abbreviations
// (an echo takes a leading "-" as an option) and pads them by bytes.
func shellSafe(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") {
		return false
	}
	for _, r := range name {
		if r <= ' ' || r >= 0x7f || r == '\\' {
			return false
		}
	}
	return true
}
