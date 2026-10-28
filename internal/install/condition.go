package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Condition is a test an install makes: the hooks' tier-1 vocabulary
// (file, missing, all, any, not; §19) and what installers test today
// (glob, command, run, platform). One test per condition; Because is the
// reason the doctor gives when a tool it makes needed is missing.
//
// File, missing and glob hold when any component's directory satisfies
// them, so a tool one component needs is installed for the language.
type Condition struct {
	File     string      `json:"file,omitempty"`     // a file matching the glob is in the component's directory
	Missing  string      `json:"missing,omitempty"`  // nothing matching the glob is
	Glob     string      `json:"glob,omitempty"`     // a file named like this is anywhere below it (find -name)
	Command  string      `json:"command,omitempty"`  // on the language's PATH
	Run      []string    `json:"run,omitempty"`      // exits 0 (output discarded, stdin closed)
	Platform string      `json:"platform,omitempty"` // "linux", "macos", "linux-aarch64"...
	Path     string      `json:"path,omitempty"`     // this file exists (absolute, ~ or {home})
	All      []Condition `json:"all,omitempty"`
	Any      []Condition `json:"any,omitempty"`
	Not      *Condition  `json:"not,omitempty"`
	Because  string      `json:"because,omitempty"`
}

// Context is what conditions are tested against.
type Context struct {
	Dirs     []string // each component's directory for the language
	Root     string   // the project root: where runs run
	Env      []string // the language's environment (its PATH)
	Platform Platform
	System   System
	Vars     map[string]string // {home}, {bin}: the toolchain's, in run and path
}

func (c *Condition) tests() int {
	n := 0
	for _, set := range []bool{c.File != "", c.Missing != "", c.Glob != "", c.Command != "", len(c.Run) > 0,
		c.Platform != "", c.Path != "", c.All != nil, c.Any != nil, c.Not != nil} {
		if set {
			n++
		}
	}
	return n
}

func (c *Condition) validate() error {
	if c.tests() != 1 {
		return errors.New("a condition needs exactly one test")
	}
	for i := range c.All {
		if err := c.All[i].validate(); err != nil {
			return err
		}
	}
	for i := range c.Any {
		if err := c.Any[i].validate(); err != nil {
			return err
		}
	}
	if c.Not != nil {
		return c.Not.validate()
	}
	return nil
}

// Holds tests the condition; a nil condition holds.
func (c *Condition) Holds(ctx *Context) bool {
	if c == nil {
		return true
	}
	switch {
	case c.File != "":
		return anyDir(ctx.Dirs, func(dir string) bool { return globExists(dir, c.File) })
	case c.Missing != "":
		return anyDir(ctx.Dirs, func(dir string) bool { return !globExists(dir, c.Missing) })
	case c.Glob != "":
		return anyDir(ctx.Dirs, func(dir string) bool { return findName(dir, c.Glob) })
	case c.Command != "":
		_, found := LookPath(c.Command, getenv(ctx.Env, "PATH"))
		return found
	case len(c.Run) > 0:
		return ctx.System.Succeeds(fill(c.Run, ctx.Vars), ctx.Root, ctx.Env)
	case c.Path != "":
		_, err := os.Stat(expandHome(fillOne(c.Path, ctx.Vars), ctx.Env))
		return err == nil
	case c.Platform != "":
		key := ctx.Platform.OS + "-" + ctx.Platform.Arch
		return c.Platform == ctx.Platform.OS || c.Platform == key
	case c.All != nil:
		for i := range c.All {
			if !c.All[i].Holds(ctx) {
				return false
			}
		}
		return true
	case c.Any != nil:
		for i := range c.Any {
			if c.Any[i].Holds(ctx) {
				return true
			}
		}
		return false
	case c.Not != nil:
		return !c.Not.Holds(ctx)
	}
	return false
}

// String describes the condition for a plan.
func (c *Condition) String() string {
	if c == nil {
		return "always"
	}
	if c.Because != "" {
		return c.Because
	}
	switch {
	case c.File != "":
		return "a component has " + c.File
	case c.Missing != "":
		return "a component has no " + c.Missing
	case c.Glob != "":
		return "a component has " + c.Glob + " files"
	case c.Command != "":
		return c.Command + " is on PATH"
	case len(c.Run) > 0:
		return "`" + strings.Join(c.Run, " ") + "` succeeds"
	case c.Platform != "":
		return "on " + c.Platform
	case c.Path != "":
		return c.Path + " exists"
	case c.Not != nil:
		return "not (" + c.Not.String() + ")"
	}
	var parts []string
	join := " and "
	list := c.All
	if c.Any != nil {
		list, join = c.Any, " or "
	}
	for i := range list {
		parts = append(parts, list[i].String())
	}
	return strings.Join(parts, join)
}

func anyDir(dirs []string, test func(string) bool) bool {
	for _, dir := range dirs {
		if test(dir) {
			return true
		}
	}
	return false
}

func globExists(dir, pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	return len(matches) > 0
}

// findName is `find dir -name pattern`: anything below dir, dir included,
// whose name matches.
func findName(dir, pattern string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: skipped, as find reports and goes on
		}
		if matched, _ := filepath.Match(pattern, entry.Name()); matched {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// LookPath finds an executable named name in path's directories.
func LookPath(name, path string) (string, bool) {
	if strings.ContainsRune(name, os.PathSeparator) {
		return name, isExecutable(name)
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(dir, name)
		if isExecutable(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// expandHome expands a leading ~/ to HOME.
func expandHome(path string, env []string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(getenv(env, "HOME"), rest)
	}
	return path
}

func getenv(env []string, name string) string {
	value := ""
	for _, entry := range env {
		if strings.HasPrefix(entry, name+"=") {
			value = entry[len(name)+1:]
		}
	}
	return value
}
