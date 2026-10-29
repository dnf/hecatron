package changed

import (
	"fmt"
	"path"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

// What a changed file selects (docs/guide/runner.md, "Only what changed"):
//
//   - .hecatron/languages/<lang>/...: that language, every component (its
//     hooks or definition changed);
//   - anything else under .hecatron/ (config.json above all): everything
//     (the layout, languages, tasks or settings may have changed);
//   - a file in a language's implementation directory: that language and
//     component (the deepest directory, as hct status maps changes);
//   - a build or version file (a definition's detect, sharedBuildFiles,
//     pinFiles or supportFiles name, or one of commonBuildFiles) in a
//     directory above implementation directories: every language and
//     component below it (a root CMakeLists.txt, Gradle settings, a
//     workspace's lock file or .tool-versions can change how any of them
//     builds or which toolchain it uses);
//   - a project task: when a file matches its taskPaths, or, with none
//     declared, when any file changed (it may read anything);
//   - lint's shell check: when a shell script changed.
//
// Other files (docs, CI workflows no definition reads) select nothing.

// commonBuildFiles are build, workspace and lock files that no definition
// names but that are shared by the components below them.
var commonBuildFiles = []string{
	"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts", "gradle.properties",
	"gradle/", "gradlew", "gradlew.bat",
	"go.work", "go.work.sum", "Cargo.lock",
	"package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "pnpm-lock.yaml", "pnpm-workspace.yaml", "bun.lock", "bun.lockb",
	"uv.lock", "poetry.lock", "Pipfile", "Pipfile.lock",
	"Gemfile.lock", "composer.lock", "mix.lock",
	"Directory.Build.props", "Directory.Build.targets", "Directory.Packages.props", "global.json", "nuget.config", "NuGet.Config", "*.sln", "*.slnx",
	".editorconfig",
}

// Selection is what a set of changed files selects.
type Selection struct {
	Base       Base
	Note       string   // set when everything runs for want of a base
	Files      []string // relative to the root
	Everything string   // set when everything runs: the file that made it so and why
	Languages  []Language
	tasks      map[string]string // task name: why it was selected
	shell      string
}

// Language is a language selected, with the components selected and why.
type Language struct {
	Name       string
	Components []string // in the project's order
	Why        string   // the first file that selected it, and how
	files      int      // how many files selected it
	of         int      // how many components it has
}

// All is whether everything runs.
func (s *Selection) All() bool { return s.Note != "" || s.Everything != "" }

// Language is a selected language's components; ok is false when it isn't
// selected.
func (s *Selection) Language(name string) (components []string, ok bool) {
	if s.All() {
		return nil, true
	}
	for _, l := range s.Languages {
		if l.Name == name {
			return l.Components, true
		}
	}
	return nil, false
}

// Task is whether a project task runs, and why.
func (s *Selection) Task(name string) (string, bool) {
	if s.All() {
		return "", true
	}
	why, ok := s.tasks[name]
	return why, ok
}

// Shell is whether lint's shell check runs.
func (s *Selection) Shell() bool { return s.All() || s.shell != "" }

// Empty reports whether nothing runs.
func (s *Selection) Empty() bool {
	return !s.All() && len(s.Languages) == 0 && len(s.tasks) == 0 && s.shell == ""
}

// Select works out what the files changed since base select. tasks are
// the project tasks to consider.
func Select(p *project.Project, base Base, files []string, tasks []string) (*Selection, error) {
	s := &Selection{Base: base, Files: files, tasks: map[string]string{}}
	owners, err := p.Owners()
	if err != nil {
		return nil, err
	}
	dirs := map[string][]project.ComponentDir{}
	buildFiles := map[string][]string{}
	for _, lang := range p.Languages {
		if dirs[lang], err = p.ImplDirs(lang); err != nil {
			return nil, err
		}
		if buildFiles[lang], err = p.BuildFiles(lang); err != nil {
			return nil, err
		}
	}
	picked := map[string]map[string]bool{}
	why := map[string]string{}
	counted := map[string]map[string]bool{}
	pick := func(lang, component, file, reason string) {
		if picked[lang] == nil {
			picked[lang], counted[lang] = map[string]bool{}, map[string]bool{}
			why[lang] = file + reason
		}
		picked[lang][component] = true
		counted[lang][file] = true
	}
	for _, file := range files {
		if rest, ok := strings.CutPrefix(file, ".hecatron/languages/"); ok {
			lang, _, _ := strings.Cut(rest, "/")
			for _, d := range dirs[lang] {
				pick(lang, d.Component, file, ": its hooks or definition")
			}
			continue
		}
		if strings.HasPrefix(file, ".hecatron/") {
			if s.Everything == "" && file != ".hecatron/local.json" {
				s.Everything = file + ", the project's configuration"
			}
			continue
		}
		if lang, component, ok := owners.Owner(project.PyJoin(p.Root, file)); ok {
			pick(lang, component, file, "")
		}
		below := func(lang, dir, reason string) {
			for _, d := range dirs[lang] {
				if under(project.PyRelpath(d.Dir, p.Root), dir) {
					pick(lang, d.Component, file, reason)
				}
			}
		}
		for _, lang := range p.Languages {
			for _, pattern := range buildFiles[lang] {
				switch {
				case !isBuildFile(file, []string{pattern}):
				case strings.Contains(pattern, "/"): // relative to the root: a version source for every component
					below(lang, ".", ": where its version is read from")
				default:
					below(lang, path.Dir(file), ": a build or version file above it")
				}
			}
		}
		if isBuildFile(file, commonBuildFiles) {
			for _, lang := range p.Languages {
				below(lang, buildFileDir(file), ": a build file above it")
			}
		}
		if s.shell == "" && (strings.HasSuffix(file, ".sh") || strings.HasSuffix(file, ".bash")) {
			s.shell = file
		}
	}
	for _, lang := range p.Languages {
		if picked[lang] == nil {
			continue
		}
		l := Language{Name: lang, Why: why[lang], files: len(counted[lang]), of: len(dirs[lang])}
		for _, d := range dirs[lang] {
			if picked[lang][d.Component] {
				l.Components = append(l.Components, d.Component)
			}
		}
		s.Languages = append(s.Languages, l)
	}
	for _, task := range tasks {
		patterns, declared, err := p.TaskPaths(task)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if !declared {
				s.tasks[task] = "no taskPaths: any change"
				break
			}
			if isBuildFile(file, patterns) {
				s.tasks[task] = file
				break
			}
		}
	}
	return s, nil
}

// isBuildFile reports whether file matches one of the patterns; a pattern
// naming a directory (gradle/) matches the files in it.
func isBuildFile(file string, patterns []string) bool {
	for _, pattern := range patterns {
		if matches(pattern, file) {
			return true
		}
		if dir, ok := strings.CutSuffix(pattern, "/"); ok && !strings.Contains(dir, "/") && strings.Contains("/"+file, "/"+dir+"/") {
			return true
		}
	}
	return false
}

// buildFileDir is the directory a common build file serves: its own, or
// for a file in a gradle/ directory, the one holding gradle/.
func buildFileDir(file string) string {
	switch i := strings.Index("/"+file, "/gradle/"); {
	case i == 0:
		return "."
	case i > 0:
		return file[:i-1]
	}
	return path.Dir(file)
}

// under reports whether a root-relative directory is dir or below it.
func under(rel, dir string) bool {
	return dir == "." || rel == dir || strings.HasPrefix(rel, dir+"/")
}

// Line is the selection in one line: what was compared, what runs and why.
func (s *Selection) Line() string {
	if s.Note != "" {
		return s.Note
	}
	head := fmt.Sprintf("Changed since %s (%s, merge base %s): %d file%s", s.Base.Ref, s.Base.Source, short(s.Base.Commit), len(s.Files), plural(len(s.Files)))
	if s.Everything != "" {
		return head + "; everything runs: " + s.Everything
	}
	// Languages selected for the same reason share it: a root build file
	// can select dozens.
	var parts []string
	var reasons []string
	names := map[string][]string{}
	others := map[string]int{}
	for _, l := range s.Languages {
		name := l.Name
		if len(l.Components) < l.of {
			name += " [" + strings.Join(l.Components, ", ") + "]"
		}
		if names[l.Why] == nil {
			reasons = append(reasons, l.Why)
		}
		names[l.Why] = append(names[l.Why], name)
		others[l.Why] = max(others[l.Why], l.files-1)
	}
	for _, why := range reasons {
		part := strings.Join(names[why], ", ") + " (" + why
		if others[why] > 0 {
			part += fmt.Sprintf(" +%d more", others[why])
		}
		parts = append(parts, part+")")
	}
	for _, task := range sortedKeys(s.tasks) {
		parts = append(parts, "task "+task+" ("+s.tasks[task]+")")
	}
	if s.shell != "" {
		parts = append(parts, "shell ("+s.shell+")")
	}
	if len(parts) == 0 {
		if len(s.Files) == 0 {
			return head + "; nothing to run"
		}
		return head + "; nothing to run: none is in a language's directory or a build file (" + s.Files[0] + more(len(s.Files)) + ")"
	}
	return head + "; running " + strings.Join(parts, ", ")
}

func short(commit string) string {
	if len(commit) > 10 {
		return commit[:10]
	}
	return commit
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func more(n int) string {
	if n > 1 {
		return fmt.Sprintf(" +%d", n-1)
	}
	return ""
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
