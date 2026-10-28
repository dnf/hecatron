package changed

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

// What `hct sync [ref]` (#92) reads after the working tree moved (a pull,
// a rebase, a checkout): where HEAD was before, the files that differ
// between there and HEAD, and what they mean for hecatron. It reads git
// and never changes it: no pull, rebase or fetch.

// SyncBase is what hct sync compares HEAD with: ref, else where HEAD was
// before it last moved (PreviousHead). A note, with no error, means there
// is nothing to compare with.
func SyncBase(root, ref string) (base Base, note string, err error) {
	if _, err := git(root, "rev-parse", "--verify", "-q", "HEAD^{commit}"); err != nil {
		if _, inRepo := git(root, "rev-parse", "--git-dir"); inRepo != nil {
			return Base{}, "", &UsageError{"sync needs a git repository"}
		}
		return Base{}, "no commit yet; nothing to compare with", nil
	}
	if ref != "" {
		if !isCommit(root, ref) {
			return Base{}, "", &UsageError{fmt.Sprintf("sync: '%s' is not a commit", ref)}
		}
		commit, err := git(root, "rev-parse", "--verify", "-q", ref+"^{commit}")
		if err != nil {
			return Base{}, "", err
		}
		return Base{Ref: ref, Source: "named", Commit: strings.TrimSpace(commit)}, "", nil
	}
	return PreviousHead(root)
}

// rebaseStep is a reflog subject a rebase writes for each step: "rebase
// (pick): ...", "rebase -i (finish): ...", "pull --rebase (start): ...".
var rebaseStep = regexp.MustCompile(`^[a-z -]*\brebase\b[a-z -]*\(([a-z ]+)\)`)

// PreviousHead is where HEAD was before it last moved: HEAD@{1}, or, when
// the last moves were a rebase's steps (one reflog entry per commit picked),
// where HEAD was before the rebase started. A note, with no error, when the
// reflog doesn't go back that far (a fresh clone, reflogs turned off).
func PreviousHead(root string) (Base, string, error) {
	out, err := git(root, "log", "-g", "-n", "1000", "--format=%H%x00%gs", "HEAD", "--")
	if err != nil {
		return Base{}, "no reflog for HEAD to find where it was; name a ref (hct sync ORIG_HEAD)", nil
	}
	type entry struct{ commit, subject string }
	var entries []entry
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		commit, subject, _ := strings.Cut(line, "\x00")
		if commit != "" {
			entries = append(entries, entry{commit, subject})
		}
	}
	previous := 1
	if len(entries) > 0 && rebaseStep.MatchString(entries[0].subject) {
		for i, e := range entries {
			step := rebaseStep.FindStringSubmatch(e.subject)
			if step == nil {
				break
			}
			if step[1] == "start" {
				previous = i + 1
				break
			}
		}
	}
	if previous >= len(entries) {
		return Base{}, "no previous HEAD in the reflog to compare with; name a ref (hct sync ORIG_HEAD)", nil
	}
	return Base{Ref: "HEAD@{" + strconv.Itoa(previous) + "}", Source: "the reflog", Commit: entries[previous].commit}, "", nil
}

// Between are the files that differ between two commits, relative to root
// (a rename as its old and new paths).
func Between(root, from, to string) ([]string, error) {
	out, err := git(root, "diff", "--name-only", "-z", "--no-renames", "--relative", from, to, "--")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}

// Commits is how many commits to has that from hasn't, and whether from is
// an ancestor of to (false after switching to another branch).
func Commits(root, from, to string) (count int, ancestor bool, err error) {
	out, err := git(root, "rev-list", "--count", from+".."+to, "--")
	if err != nil {
		return 0, false, err
	}
	if count, err = strconv.Atoi(strings.TrimSpace(out)); err != nil {
		return 0, false, err
	}
	_, notAncestor := git(root, "merge-base", "--is-ancestor", from, to)
	return count, notAncestor == nil, nil
}

// Head is HEAD's commit.
func Head(root string) (string, error) {
	out, err := git(root, "rev-parse", "--verify", "-q", "HEAD^{commit}")
	return strings.TrimSpace(out), err
}

// Catchup is what a set of changed files asks of hecatron (hct sync).
type Catchup struct {
	Deps       []Touched // lock files changed: fetch the dependencies again
	Toolchains []Touched // pin or support files, or the language's definition, changed: the version needed may differ
	Check      []string  // the project's configuration, hooks or generated files changed: hct check
}

// Touched is a language whose components files changed for.
type Touched struct {
	Name       string
	Components []string // in the project's order
	All        bool     // every component the language has
	Files      []string // the files, relative to the root
}

// Empty reports whether nothing changed that hecatron acts on.
func (c *Catchup) Empty() bool {
	return len(c.Deps) == 0 && len(c.Toolchains) == 0 && len(c.Check) == 0
}

// Catch works out what the files changed ask for. generated are the files
// hecatron generates for the project, relative to the root.
//
//   - A lock file (a definition's cache.lockFiles) in a component's
//     directory, below it, or in a directory above it (a workspace's):
//     that language's deps, in those components.
//   - A pin or support file (pinFiles, supportFiles) in a component's
//     directory or above it, where its version is looked for: the
//     language's version, in those components. A change under
//     .hecatron/languages/<lang>/ may change its project version or pins:
//     every component.
//   - Anything under .hecatron/ but local.json (config.json, hooks,
//     definitions, templates), or a generated file: hct check. A version
//     change, when the project generates files: hct check too (generated
//     files name versions).
func Catch(p *project.Project, files, generated []string) (*Catchup, error) {
	c := &Catchup{}
	deps := touches{}
	toolchains := touches{}
	owners, err := p.Owners()
	if err != nil {
		return nil, err
	}
	for _, lang := range p.Languages {
		dirs, err := p.ImplDirs(lang)
		if err != nil {
			return nil, err
		}
		cache, err := p.Cache(lang)
		if err != nil {
			return nil, err
		}
		versionFiles, err := p.VersionFiles(lang)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if rest, ok := strings.CutPrefix(file, ".hecatron/languages/"); ok {
				if name, _, _ := strings.Cut(rest, "/"); name == lang {
					for _, d := range dirs {
						toolchains.add(lang, d.Component, file)
					}
				}
				continue
			}
			for _, d := range dirs {
				dir := project.PyRelpath(d.Dir, p.Root)
				owner, component, owned := owners.Owner(project.PyJoin(p.Root, file))
				inside := owned && owner == lang && component == d.Component
				if servesComponent(cache.LockFiles, file, dir, inside) {
					deps.add(lang, d.Component, file)
				}
				if servesComponent(versionFiles, file, dir, false) {
					toolchains.add(lang, d.Component, file)
				}
			}
		}
	}
	for _, file := range files {
		if strings.HasPrefix(file, ".hecatron/") && file != ".hecatron/local.json" || contains(generated, file) {
			c.Check = append(c.Check, file)
		}
	}
	if c.Deps, err = deps.list(p); err != nil {
		return nil, err
	}
	if c.Toolchains, err = toolchains.list(p); err != nil {
		return nil, err
	}
	if len(c.Check) == 0 && len(generated) > 0 {
		for _, t := range c.Toolchains {
			c.Check = append(c.Check, t.Files...)
		}
		c.Check = unique(c.Check)
	}
	return c, nil
}

// servesComponent reports whether a changed file is one of the patterns
// for a component in dir (relative to the root): in that directory or one
// above it, where the file is looked for; with inside (the file is in the
// component's tree, not a nested component's), also below it: a nested
// package's lock file.
func servesComponent(patterns []string, file, dir string, inside bool) bool {
	fileDir := path.Dir(file)
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(pattern, "./")
		for above := dir; ; above = path.Dir(above) {
			rest := file
			if above != "." {
				var ok bool
				if rest, ok = strings.CutPrefix(file, above+"/"); !ok {
					rest = ""
				}
			}
			if rest != "" {
				if strings.Contains(pattern, "/") {
					if ok, _ := path.Match(pattern, rest); ok {
						return true
					}
				} else if fileDir == above {
					if ok, _ := path.Match(pattern, path.Base(file)); ok {
						return true
					}
				}
			}
			if above == "." {
				break
			}
		}
		if inside && !strings.Contains(pattern, "/") {
			if ok, _ := path.Match(pattern, path.Base(file)); ok {
				return true
			}
		}
	}
	return false
}

// touches collects the components files touch, per language.
type touches map[string]*struct {
	components map[string]bool
	files      []string
}

func (t touches) add(lang, component, file string) {
	entry := t[lang]
	if entry == nil {
		entry = &struct {
			components map[string]bool
			files      []string
		}{components: map[string]bool{}}
		t[lang] = entry
	}
	entry.components[component] = true
	if !contains(entry.files, file) {
		entry.files = append(entry.files, file)
	}
}

// list is the languages touched, in the project's order.
func (t touches) list(p *project.Project) ([]Touched, error) {
	var result []Touched
	for _, lang := range p.Languages {
		entry := t[lang]
		if entry == nil {
			continue
		}
		dirs, err := p.ImplDirs(lang)
		if err != nil {
			return nil, err
		}
		touched := Touched{Name: lang, Files: entry.files}
		for _, d := range dirs {
			if entry.components[d.Component] && !contains(touched.Components, d.Component) {
				touched.Components = append(touched.Components, d.Component)
			}
		}
		touched.All = len(touched.Components) == len(dirs)
		result = append(result, touched)
	}
	return result, nil
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

func unique(list []string) []string {
	var result []string
	for _, x := range list {
		if !contains(result, x) {
			result = append(result, x)
		}
	}
	return result
}

// Short is a commit as hct sync and --changed show it.
func Short(commit string) string { return short(commit) }
