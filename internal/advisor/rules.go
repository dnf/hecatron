// Package advisor is `hct advisor` (#105): where doctor keeps the machine
// working, the advisor reads the project and recommends what each
// language's ecosystem considers good practice, citing that ecosystem's own
// documentation. It reads files only: no network, no toolchain, nothing
// installed, and with --fix only safe, mechanical changes (a pin file at
// the version in use).
//
// Rules are data where they can be (languages/<lang>/advice.json and the
// sets every definition may include from languages/_advice.json); rules
// that need logic (a toolchain past end of life, a lock file that no longer
// matches its manifest, CI coverage, audit acceptances) are Go code here,
// parameterised by that data.
package advisor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

// Categories are what a rule is about, in the order findings are listed.
var Categories = []string{"reproducibility", "currency", "tooling", "security", "performance", "ci"}

// Levels: "recommended" is what the ecosystem asks for; "suggestion" is
// good practice a project may have reasons to skip.
const (
	Recommended = "recommended"
	Suggestion  = "suggestion"
)

// Set is one advice.json, or one named set in _advice.json: rules over
// files, and the parameters of the rules written in Go.
type Set struct {
	// Include names sets of _advice.json this one adds (a language's
	// advice.json only): JavaScript and TypeScript share "node".
	Include []string `json:"include,omitempty"`
	Comment string   `json:"$comment,omitempty"`
	// Toolchain names the toolchain in messages ("Node.js"); the
	// definition's name when absent.
	Toolchain string `json:"toolchain,omitempty"`
	Rules     []Rule `json:"rules,omitempty"`
	// Pin: the toolchain should be pinned in the ecosystem's own file.
	Pin *Pin `json:"pin,omitempty"`
	// EndOfLife: the version in use shouldn't be past its line's support.
	EndOfLife *EndOfLife `json:"endOfLife,omitempty"`
	// Behind: the version in use shouldn't be many lines behind the newest.
	Behind *Behind `json:"behind,omitempty"`
	// LockFresh: a lock file should still match its manifest.
	LockFresh []LockFresh `json:"lockFresh,omitempty"`
	// CI: some CI configuration should run the language.
	CI *CI `json:"ci,omitempty"`

	name string // the namespace of its rules' ids: the language, or the set's name
}

// Rule is a rule over files: where When holds (always, when absent) and
// Expect doesn't, the advisor reports Message.
type Rule struct {
	ID       string     `json:"id"`
	Category string     `json:"category"`
	Level    string     `json:"level"`
	Scope    string     `json:"scope,omitempty"` // "component" (the default) or "root"
	When     *Condition `json:"when,omitempty"`
	Expect   *Condition `json:"expect"`
	Message  string     `json:"message"`
	Why      string     `json:"why"`
	Link     string     `json:"link"`
}

// Pin parameterises the toolchain-pinned rule: where When holds in a
// component and its version comes from no pin file the definition reads
// (pinFiles), --fix can create Create with Content, "{version}" and
// "{line}" (the first LineParts parts) filled in from the version in use.
type Pin struct {
	When      *Condition `json:"when"`
	Level     string     `json:"level"`
	Create    string     `json:"create"`
	Content   string     `json:"content"`
	LineParts int        `json:"lineParts"`
	// MinParts: --fix writes the file only when the version in use has at
	// least this many parts (global.json needs a whole SDK version).
	MinParts int    `json:"minParts,omitempty"`
	Why      string `json:"why"`
	Link     string `json:"link"`
}

// EndOfLife is a release line's support policy, from the line's first
// release in releases.json: a line is past its end of life once Lines
// newer lines exist, or Months after it began; LTS lines (any release of
// the line marked "lts") use LTSLines and LTSMonths instead. Zero means
// that limit doesn't apply.
type EndOfLife struct {
	LineParts int    `json:"lineParts"`
	Lines     int    `json:"lines,omitempty"`
	Months    int    `json:"months,omitempty"`
	LTSLines  int    `json:"ltsLines,omitempty"`
	LTSMonths int    `json:"ltsMonths,omitempty"`
	Why       string `json:"why"`
	Link      string `json:"link"`
}

// Behind flags a version more than Lines release lines behind the newest.
type Behind struct {
	LineParts int    `json:"lineParts"`
	Lines     int    `json:"lines"`
	Why       string `json:"why"`
	Link      string `json:"link"`
}

// LockFresh compares a lock file with its manifest, in a format the Go
// code knows ("npm": package-lock.json's root package against
// package.json's dependencies, which npm ci refuses to install when they
// differ).
type LockFresh struct {
	Manifest string `json:"manifest"`
	Lock     string `json:"lock"`
	Format   string `json:"format"`
	Why      string `json:"why"`
	Link     string `json:"link"`
}

// CI lists what in a CI configuration shows it runs the language: an
// action, a tool's command.
type CI struct {
	Markers []string `json:"markers"`
	Why     string   `json:"why"`
	Link    string   `json:"link"`
}

// Library is every rule the advisor knows: each language's set, with the
// shared sets it includes.
type Library struct {
	Languages map[string]*Set // by language; nil for a language without advice
	Shared    map[string]*Set // _advice.json's sets, by name
	Repo      *Set            // _advice.json's "repo" set: rules for the whole repository
}

// sharedFile holds the sets definitions share.
type sharedFile struct {
	Comment string          `json:"$comment,omitempty"`
	Sets    map[string]*Set `json:"sets"`
}

// Load reads the advice for the given languages: each one's advice.json as
// the project resolves it (its own definition's, else the built-in one),
// and the built-in _advice.json.
func Load(p *project.Project, languages []string) (*Library, error) {
	lib := &Library{Languages: map[string]*Set{}, Shared: map[string]*Set{}}
	shared := filepath.Join(p.Home.Languages, "_advice.json")
	if data, err := os.ReadFile(shared); err == nil {
		var file sharedFile
		if err := decodeStrict(data, &file); err != nil {
			return nil, fmt.Errorf("%s: %v", shared, err)
		}
		for name, set := range file.Sets {
			set.name = name
			if err := set.validate(shared, false); err != nil {
				return nil, err
			}
		}
		lib.Shared = file.Sets
		lib.Repo = file.Sets["repo"]
		delete(lib.Shared, "repo")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for _, lang := range languages {
		path := p.Resolve(lang, "advice.json")
		if path == "" {
			lib.Languages[lang] = nil
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		set := &Set{name: lang}
		if err := decodeStrict(data, set); err != nil {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
		if err := set.validate(path, true); err != nil {
			return nil, err
		}
		for _, name := range set.Include {
			if lib.Shared[name] == nil {
				return nil, fmt.Errorf("%s: includes %q, which _advice.json doesn't define", path, name)
			}
		}
		lib.Languages[lang] = set
	}
	return lib, nil
}

// sets are a language's sets: its own, then those it includes.
func (lib *Library) sets(lang string) []*Set {
	own := lib.Languages[lang]
	if own == nil {
		return nil
	}
	sets := []*Set{own}
	for _, name := range own.Include {
		sets = append(sets, lib.Shared[name])
	}
	return sets
}

// IDs are every rule id the library can report, sorted: the data rules',
// and those of the rules in Go each set parameterises.
func (lib *Library) IDs() []string {
	seen := map[string]bool{}
	add := func(set *Set) {
		if set == nil {
			return
		}
		for _, r := range set.Rules {
			seen[set.name+"/"+r.ID] = true
		}
		if set.Pin != nil {
			seen[set.name+"/"+idPinned] = true
		}
		if set.EndOfLife != nil {
			seen[set.name+"/"+idEndOfLife] = true
		}
		if set.Behind != nil {
			seen[set.name+"/"+idBehind] = true
		}
		if len(set.LockFresh) > 0 {
			seen[set.name+"/"+idLockFresh] = true
		}
		if set.CI != nil {
			seen[set.name+"/"+idCICovers] = true
		}
	}
	for _, set := range lib.Languages {
		add(set)
	}
	for _, set := range lib.Shared {
		add(set)
	}
	add(lib.Repo)
	for _, id := range builtinIDs {
		seen[id] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func decodeStrict(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

var ruleID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (s *Set) validate(path string, language bool) error {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", path, fmt.Sprintf(format, args...))
	}
	if !language && len(s.Include) > 0 {
		return fail("set %s: only a language's advice.json includes sets", s.name)
	}
	seen := map[string]bool{}
	for _, r := range s.Rules {
		if !ruleID.MatchString(r.ID) {
			return fail("rule id %q: lowercase words joined by hyphens", r.ID)
		}
		if seen[r.ID] {
			return fail("rule %s is defined twice", r.ID)
		}
		seen[r.ID] = true
		if !contains(Categories, r.Category) {
			return fail("rule %s: category must be one of %s", r.ID, strings.Join(Categories, ", "))
		}
		if err := checkLevel(r.Level); err != nil {
			return fail("rule %s: %v", r.ID, err)
		}
		if r.Scope != "" && r.Scope != "component" && r.Scope != "root" {
			return fail("rule %s: scope must be component or root", r.ID)
		}
		if r.Expect == nil {
			return fail("rule %s needs an expect condition", r.ID)
		}
		for _, c := range []*Condition{r.When, r.Expect} {
			if err := c.validate(); err != nil {
				return fail("rule %s: %v", r.ID, err)
			}
		}
		if err := checkText(r.Message, r.Why, r.Link); err != nil {
			return fail("rule %s: %v", r.ID, err)
		}
	}
	if pin := s.Pin; pin != nil {
		if err := checkLevel(pin.Level); err != nil {
			return fail("pin: %v", err)
		}
		if err := pin.When.validate(); err != nil || pin.When == nil {
			return fail("pin needs a valid when condition")
		}
		if pin.Create == "" || pin.Content == "" || strings.ContainsAny(pin.Create, `/\`) {
			return fail("pin needs a file to create in the component's directory, and its content")
		}
		if err := checkText("-", pin.Why, pin.Link); err != nil {
			return fail("pin: %v", err)
		}
	}
	if eol := s.EndOfLife; eol != nil {
		if eol.LineParts < 1 || eol.Lines+eol.Months+eol.LTSLines+eol.LTSMonths == 0 {
			return fail("endOfLife needs lineParts and a limit")
		}
		if err := checkText("-", eol.Why, eol.Link); err != nil {
			return fail("endOfLife: %v", err)
		}
	}
	if b := s.Behind; b != nil {
		if b.LineParts < 1 || b.Lines < 1 {
			return fail("behind needs lineParts and lines")
		}
		if err := checkText("-", b.Why, b.Link); err != nil {
			return fail("behind: %v", err)
		}
	}
	for _, l := range s.LockFresh {
		if lockFormats[l.Format] == nil || l.Manifest == "" || l.Lock == "" {
			return fail("lockFresh needs a manifest, a lock and a known format")
		}
		if err := checkText("-", l.Why, l.Link); err != nil {
			return fail("lockFresh: %v", err)
		}
	}
	if ci := s.CI; ci != nil {
		if len(ci.Markers) == 0 {
			return fail("ci needs markers")
		}
		if err := checkText("-", ci.Why, ci.Link); err != nil {
			return fail("ci: %v", err)
		}
	}
	return nil
}

func checkLevel(level string) error {
	if level != Recommended && level != Suggestion {
		return fmt.Errorf("level must be %s or %s", Recommended, Suggestion)
	}
	return nil
}

func checkText(message, why, link string) error {
	switch {
	case strings.TrimSpace(message) == "":
		return fmt.Errorf("needs a message")
	case strings.TrimSpace(why) == "":
		return fmt.Errorf("needs a why")
	case !strings.HasPrefix(link, "https://"):
		return fmt.Errorf("needs an https link to the ecosystem's documentation")
	}
	return nil
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}
