package advisor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Condition tests files, with the vocabulary of a definition's step
// conditions (docs/design.md §19): exactly one of
//
//	{"file": GLOB}               a file or directory matches, in the directory
//	{"missing": GLOB}            nothing matches
//	{"tracked": GLOB}            a match is committed to git (outside git: exists)
//	{"contains": [GLOB, REGEX]}  a matching file's text matches the RE2 regex
//	{"all": [...]}, {"any": [...]}, {"not": {...}}
//
// GLOB is relative to the rule's directory (the component's, or the
// root's), in filepath.Match syntax; "up": true also looks in each parent
// directory up to the project root, as tools that search upward for their
// configuration do.
type Condition struct {
	File     string       `json:"file,omitempty"`
	Missing  string       `json:"missing,omitempty"`
	Tracked  string       `json:"tracked,omitempty"`
	Contains []string     `json:"contains,omitempty"`
	Up       bool         `json:"up,omitempty"`
	All      []*Condition `json:"all,omitempty"`
	Any      []*Condition `json:"any,omitempty"`
	Not      *Condition   `json:"not,omitempty"`
}

func (c *Condition) validate() error {
	if c == nil {
		return nil
	}
	set := 0
	for _, present := range []bool{c.File != "", c.Missing != "", c.Tracked != "", c.Contains != nil, c.All != nil, c.Any != nil, c.Not != nil} {
		if present {
			set++
		}
	}
	if set != 1 {
		return fmt.Errorf("a condition has exactly one of file, missing, tracked, contains, all, any, not")
	}
	if c.Up && c.All == nil && c.Any == nil && c.Not == nil {
		// fine: up applies to the file tests
	} else if c.Up {
		return fmt.Errorf(`"up" applies to file, missing, tracked and contains`)
	}
	for _, glob := range []string{c.File, c.Missing, c.Tracked} {
		if err := checkGlob(glob); err != nil {
			return err
		}
	}
	if c.Contains != nil {
		if len(c.Contains) != 2 {
			return fmt.Errorf("contains is [GLOB, REGEX]")
		}
		if err := checkGlob(c.Contains[0]); err != nil {
			return err
		}
		if _, err := regexp.Compile(c.Contains[1]); err != nil {
			return fmt.Errorf("contains: %v", err)
		}
	}
	for _, sub := range append(append([]*Condition{}, c.All...), c.Any...) {
		if err := sub.validate(); err != nil {
			return err
		}
	}
	return c.Not.validate()
}

func checkGlob(glob string) error {
	if glob == "" {
		return nil
	}
	if _, err := filepath.Match(glob, ""); err != nil || filepath.IsAbs(glob) || strings.Contains(glob, "..") {
		return fmt.Errorf("bad glob %q", glob)
	}
	return nil
}

// files is what conditions read, cached for one run: directory listings
// through globs, file contents, compiled regexes, and the files git tracks.
type files struct {
	root    string
	tracked map[string]bool // trackedSet, relative to root; nil outside git
	mu      sync.Mutex
	text    map[string]string
	regex   map[string]*regexp.Regexp
}

func newFiles(root string, tracked map[string]bool) *files {
	return &files{root: root, tracked: tracked, text: map[string]string{}, regex: map[string]*regexp.Regexp{}}
}

// eval is whether the condition holds in dir.
func (f *files) eval(c *Condition, dir string) bool {
	switch {
	case c == nil:
		return true
	case c.All != nil:
		for _, sub := range c.All {
			if !f.eval(sub, dir) {
				return false
			}
		}
		return true
	case c.Any != nil:
		for _, sub := range c.Any {
			if f.eval(sub, dir) {
				return true
			}
		}
		return false
	case c.Not != nil:
		return !f.eval(c.Not, dir)
	case c.Missing != "":
		return !f.anyDir(dir, c.Up, func(d string) bool { return len(f.glob(d, c.Missing)) > 0 })
	case c.File != "":
		return f.anyDir(dir, c.Up, func(d string) bool { return len(f.glob(d, c.File)) > 0 })
	case c.Tracked != "":
		return f.anyDir(dir, c.Up, func(d string) bool {
			for _, match := range f.glob(d, c.Tracked) {
				if f.isTracked(match) {
					return true
				}
			}
			return false
		})
	case c.Contains != nil:
		re := f.compile(c.Contains[1])
		return f.anyDir(dir, c.Up, func(d string) bool {
			for _, match := range f.glob(d, c.Contains[0]) {
				if re.MatchString(f.read(match)) {
					return true
				}
			}
			return false
		})
	}
	return false
}

// anyDir is whether test holds in dir, or with up in any directory from
// dir to the root.
func (f *files) anyDir(dir string, up bool, test func(string) bool) bool {
	for {
		if test(dir) {
			return true
		}
		if !up || dir == f.root || !strings.HasPrefix(dir, f.root+string(filepath.Separator)) {
			return false
		}
		dir = filepath.Dir(dir)
	}
}

func (f *files) glob(dir, pattern string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, filepath.FromSlash(pattern)))
	return matches
}

func (f *files) isTracked(path string) bool {
	if f.tracked == nil {
		return true // outside git, a file that exists is as committed as it gets
	}
	rel, err := filepath.Rel(f.root, path)
	if err != nil {
		return false
	}
	return f.tracked[filepath.ToSlash(rel)]
}

// trackedSet is git's tracked files under root and every directory holding
// one (a directory is tracked when a file in it is).
func trackedSet(paths []string) map[string]bool {
	set := map[string]bool{}
	for _, path := range paths {
		for path != "." && path != "" && !set[path] {
			set[path] = true
			path = pathDir(path)
		}
	}
	return set
}

func pathDir(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return ""
}

// read is a file's text, at most 1 MiB of it; "" for a directory or a
// file that can't be read.
func (f *files) read(path string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if text, ok := f.text[path]; ok {
		return text
	}
	text := ""
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		if file, err := os.Open(path); err == nil {
			buffer := make([]byte, 1<<20)
			n, _ := file.Read(buffer)
			for n < len(buffer) {
				m, err := file.Read(buffer[n:])
				n += m
				if err != nil || m == 0 {
					break
				}
			}
			file.Close()
			text = string(buffer[:n])
		}
	}
	f.text[path] = text
	return text
}

func (f *files) compile(expr string) *regexp.Regexp {
	f.mu.Lock()
	defer f.mu.Unlock()
	re, ok := f.regex[expr]
	if !ok {
		re = regexp.MustCompile(expr) // validated when loaded
		f.regex[expr] = re
	}
	return re
}
