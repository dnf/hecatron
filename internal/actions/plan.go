package actions

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind is what an Op does.
type Kind int

const (
	Run     Kind = iota // run Argv with Env
	Remove              // remove what Paths match, never inside Keep
	Skip                // nothing to do: Message says why
	Unknown             // can't tell how: Message says what was looked for
	Note                // say Message
	Enter               // the ops that follow, up to the next Enter, are in Dir
)

// Op is one planned operation, in Dir (relative to the component's
// directory; "." for the directory itself).
type Op struct {
	Kind    Kind
	Dir     string
	Argv    []string
	Env     map[string]string
	Paths   []string
	Keep    []string
	Message string
}

// Context is where an action is planned: a component's directory inside a
// project.
type Context struct {
	Action string
	Dir    string // absolute
	Root   string // absolute; Find never looks above it
}

// Family plans an action itself, for build systems several languages share.
type Family func(def *Definition, ctx Context) ([]Op, error)

var families = map[string]Family{}

// Plan says what the default hook for ctx.Action does in ctx.Dir.
func Plan(def *Definition, ctx Context) ([]Op, error) {
	if action, ok := def.Actions[ctx.Action]; ok {
		return planAction(def, action, ctx)
	}
	if def.Family != "" {
		family, ok := families[def.Family]
		if !ok {
			return nil, fmt.Errorf("%s: no family %q", def.Lang, def.Family)
		}
		return family(def, ctx)
	}
	return nil, fmt.Errorf("%s: no %s action", def.Lang, ctx.Action)
}

func planAction(def *Definition, action Action, ctx Context) ([]Op, error) {
	if len(action.Files) > 0 {
		where, dirs := Find(ctx.Dir, ctx.Root, action.Find, action.Files)
		switch where {
		case Below:
			return inEach(dirs, ctx, func(sub Context) ([]Op, error) { return planAction(def, action, sub) })
		case Nowhere:
			if action.Find == "below-else-here" {
				break
			}
			message := action.Unknown
			if message == "" {
				message = "no " + strings.Join(action.Files, ", ") + " " + searched(action.Find)
			}
			return []Op{{Kind: Unknown, Dir: ".", Message: message}}, nil
		}
	}
	return planSteps(def, action.Steps, ctx.Dir)
}

// inEach plans an action in each directory below, as the shell hooks rerun
// themselves there (run_hook_in).
func inEach(dirs []string, ctx Context, plan func(Context) ([]Op, error)) ([]Op, error) {
	var ops []Op
	for _, dir := range dirs {
		sub := ctx
		sub.Dir = filepath.Join(ctx.Dir, dir)
		subOps, err := plan(sub)
		if err != nil {
			return nil, err
		}
		ops = append(ops, Op{Kind: Enter, Dir: dir})
		for _, op := range subOps {
			if op.Kind != Enter {
				op.Dir = filepath.Join(dir, op.Dir)
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}

func searched(find string) string {
	switch find {
	case "above-first", "below-first":
		return "here, below, or above it in the project"
	}
	return "here"
}

func planSteps(def *Definition, steps []Step, dir string) ([]Op, error) {
	var ops []Op
	for _, step := range steps {
		if step.If != nil && !step.If.holds(dir) {
			continue
		}
		switch {
		case step.Use != "":
			shared, ok := def.Shared[step.Use]
			if !ok {
				return nil, fmt.Errorf("%s: no shared steps %q", def.Lang, step.Use)
			}
			sharedOps, err := planSteps(def, shared, dir)
			if err != nil {
				return nil, err
			}
			ops = append(ops, sharedOps...)
			if stops(sharedOps) {
				return ops, nil
			}
		case len(step.Run) > 0:
			ops = append(ops, Op{Kind: Run, Dir: ".", Argv: withArgs(step, dir), Env: step.Env})
		case len(step.Remove) > 0:
			ops = append(ops, Op{Kind: Remove, Dir: ".", Paths: step.Remove, Keep: step.Keep})
		case step.Skip != "":
			return append(ops, Op{Kind: Skip, Dir: ".", Message: step.Skip}), nil
		case step.Unknown != "":
			return append(ops, Op{Kind: Unknown, Dir: ".", Message: step.Unknown}), nil
		default:
			return nil, fmt.Errorf("%s: a step with nothing to do", def.Lang)
		}
	}
	return ops, nil
}

// withArgs is a step's command with the arguments that apply in dir.
func withArgs(step Step, dir string) []string {
	argv := append([]string(nil), step.Run...)
	for _, arg := range step.Args {
		if arg.If != nil && !arg.If.holds(dir) {
			continue
		}
		if len(arg.EachProject) == 0 {
			argv = append(argv, arg.Add...)
			continue
		}
		for _, project := range ProjectsBelow(dir, arg.EachProject) {
			for _, add := range arg.Add {
				argv = append(argv, strings.ReplaceAll(add, "{dir}", project))
			}
		}
	}
	return argv
}

func stops(ops []Op) bool {
	return len(ops) > 0 && (ops[len(ops)-1].Kind == Skip || ops[len(ops)-1].Kind == Unknown)
}

func (c *Cond) holds(dir string) bool {
	switch {
	case c.File != "":
		return exists(dir, c.File)
	case c.Missing != "":
		return !exists(dir, c.Missing)
	case len(c.Contains) == 2:
		data, err := os.ReadFile(filepath.Join(dir, c.Contains[0]))
		return err == nil && strings.Contains(string(data), c.Contains[1])
	case c.Glob != "":
		return FindName(dir, c.Glob)
	case c.All != nil:
		for i := range c.All {
			if !c.All[i].holds(dir) {
				return false
			}
		}
		return true
	case c.Any != nil:
		for i := range c.Any {
			if c.Any[i].holds(dir) {
				return true
			}
		}
		return false
	case c.Not != nil:
		return !c.Not.holds(dir)
	}
	return false
}

func exists(dir, glob string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, glob))
	return len(matches) > 0
}

func existsAny(dir string, globs []string) bool {
	for _, glob := range globs {
		if exists(dir, glob) {
			return true
		}
	}
	return false
}

// Where build files were found.
type Where int

const (
	Here Where = iota
	Above
	Below
	Nowhere
)

// Find looks for files matching globs as the shell hooks' find_build does:
// here; then, by find, the nearest directory above (up to root) or the
// shallowest directories below. dirs is the directory above (absolute) or
// those below (relative).
func Find(dir, root, find string, globs []string) (Where, []string) {
	if existsAny(dir, globs) {
		return Here, nil
	}
	if find == "" || find == "here" {
		return Nowhere, nil
	}
	if find == "above-first" {
		if above, ok := DirAbove(dir, root, globs); ok {
			return Above, []string{above}
		}
	}
	if below := DirsBelow(dir, globs); len(below) > 0 {
		return Below, below
	}
	if find == "below-first" {
		if above, ok := DirAbove(dir, root, globs); ok {
			return Above, []string{above}
		}
	}
	return Nowhere, nil
}

// DirAbove is the nearest directory above dir, up to root, with a file
// matching one of the globs.
func DirAbove(dir, root string, globs []string) (string, bool) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	for d := filepath.Dir(dir); ; d = filepath.Dir(d) {
		if existsAny(d, globs) {
			return d, true
		}
		if d == root || d == filepath.Dir(d) {
			return "", false
		}
	}
}

// Directories below that are never searched: vendored code, test data and
// build output.
var notSearched = map[string]bool{
	"node_modules": true, "vendor": true, "third_party": true, "third-party": true, "external": true,
	"testdata": true, "test-data": true, "test_data": true, "fixtures": true,
	"build": true, "target": true, "out": true, "dist": true, "bin": true, "obj": true,
}

// FindName says whether a file named like the glob is in dir or below it,
// at any depth, outside the directories never searched and hidden ones
// (version control, virtual environments).
func FindName(dir, glob string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != dir && (strings.HasPrefix(entry.Name(), ".") || notSearched[entry.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		if ok, _ := filepath.Match(glob, entry.Name()); ok {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// ProjectsBelow is the directories below dir, at most three levels down,
// with a file matching one of the globs, leaving out any inside another:
// the projects inside the one in dir, relative to it and sorted.
func ProjectsBelow(dir string, globs []string) []string {
	found := map[string]bool{}
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if entry.IsDir() {
			if rel != "." && (strings.HasPrefix(entry.Name(), ".") || notSearched[entry.Name()] || depth > 3) {
				return filepath.SkipDir
			}
			return nil
		}
		if depth < 2 {
			return nil
		}
		for _, glob := range globs {
			if ok, _ := filepath.Match(glob, entry.Name()); ok {
				found[filepath.ToSlash(filepath.Dir(rel))] = true
			}
		}
		return nil
	})
	dirs := make([]string, 0, len(found))
	for sub := range found {
		dirs = append(dirs, sub)
	}
	sort.Strings(dirs)
	var outermost []string
	for _, sub := range dirs {
		inside := false
		for _, kept := range outermost {
			if strings.HasPrefix(sub, kept+"/") {
				inside = true
				break
			}
		}
		if !inside {
			outermost = append(outermost, sub)
		}
	}
	return outermost
}

// DirsBelow is the shallowest directories below dir, at most three levels
// down, with a file matching one of the globs, relative to dir and sorted.
func DirsBelow(dir string, globs []string) []string {
	found := map[string]int{}
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if entry.IsDir() {
			if rel != "." && (strings.HasPrefix(entry.Name(), ".") || notSearched[entry.Name()] || depth > 3) {
				return filepath.SkipDir
			}
			return nil
		}
		if depth < 2 {
			return nil
		}
		for _, glob := range globs {
			if ok, _ := filepath.Match(glob, entry.Name()); ok {
				found[filepath.Dir(rel)] = depth
			}
		}
		return nil
	})
	least := 0
	for _, depth := range found {
		if least == 0 || depth < least {
			least = depth
		}
	}
	var dirs []string
	for sub, depth := range found {
		if depth == least {
			dirs = append(dirs, sub)
		}
	}
	sort.Strings(dirs)
	return dirs
}
