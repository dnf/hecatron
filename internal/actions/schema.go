// Package actions plans a language's default hooks (test, lint, build,
// clean...) from its definition: the rules in its config.json, or the Go
// family it names for build systems several languages share (docs/design.md
// §19).
package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Cond is a test on the directory a step runs in. Exactly one field is set.
type Cond struct {
	File     string   `json:"file,omitempty"`     // a file matching the glob exists
	Missing  string   `json:"missing,omitempty"`  // nothing matches the glob
	Contains []string `json:"contains,omitempty"` // [file, text]: the file contains the text
	Glob     string   `json:"glob,omitempty"`     // a file named like this is here or below (FindName)
	All      []Cond   `json:"all,omitempty"`
	Any      []Cond   `json:"any,omitempty"`
	Not      *Cond    `json:"not,omitempty"`
}

// Step is one thing an action does, when its condition holds. Exactly one of
// Run, Remove, Skip, Unknown and Use is set.
type Step struct {
	If      *Cond             `json:"if,omitempty"`
	Run     []string          `json:"run,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Remove  []string          `json:"remove,omitempty"` // globs; ** matches any depth
	Keep    []string          `json:"keep,omitempty"`   // directories Remove never descends into
	Skip    string            `json:"skip,omitempty"`   // nothing to do here: say why and stop
	Unknown string            `json:"unknown,omitempty"`
	Use     string            `json:"use,omitempty"`  // the definition's shared steps of that name
	Args    []Arg             `json:"args,omitempty"` // added to Run's, in order, where they apply
}

// Arg is arguments a step's command takes where their condition holds.
type Arg struct {
	If  *Cond    `json:"if,omitempty"`
	Add []string `json:"add"`
}

// Action is how a language does one action: where its build files are, and
// the steps to take there.
type Action struct {
	// Find says where to look for Files: "here" (the default), "above-first"
	// (here, the nearest directory above, then the directories below; for
	// tools that find their build file upward themselves), "below-first"
	// (here, below, then above) or "below-else-here" (here, below, else the
	// steps run here anyway: for a tool that works without a build file).
	Find    string   `json:"find,omitempty"`
	Files   []string `json:"files,omitempty"`
	Unknown string   `json:"unknown,omitempty"` // why, when no Files are found
	Steps   []Step   `json:"steps"`
}

// Definition is the part of a language definition that plans its actions.
type Definition struct {
	Lang    string            `json:"-"`
	Family  string            `json:"family,omitempty"`
	Shared  map[string][]Step `json:"shared,omitempty"`
	Actions map[string]Action `json:"actions,omitempty"`
}

// Load reads languages/<lang>/config.json.
func Load(languages, lang string) (*Definition, error) {
	data, err := os.ReadFile(filepath.Join(languages, lang, "config.json"))
	if err != nil {
		return nil, err
	}
	var def Definition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("%s: %w", lang, err)
	}
	def.Lang = lang
	return &def, nil
}
