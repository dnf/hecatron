// Package status is `hct status` (#121): what an agent or a person needs
// first in a repository, in one command. Hecatron's version, the project
// (its config saved or only detected), and for each language its
// directories, the toolchain version it resolves to and why, whether that
// toolchain's command is on each component's PATH, the last run of each
// action, and the uncommitted changes in it; then the commands worth
// running next. It runs one `git status` and no doctor scripts.
package status

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
)

// Actions are the run commands whose last run status reports, in order.
var Actions = []string{"test", "lint", "build", "format", "deps", "audit", "outdated", "docs", "bench", "clean"}

// Status is everything status reports.
type Status struct {
	Version   string     `json:"version"`
	Runner    string     `json:"runner"` // "binary": status exists only in the Go core
	Root      string     `json:"root"`
	Config    string     `json:"config"` // "saved" or "detected"
	Languages []Language `json:"languages"`
	// OtherChanges are changed files in no language's directory.
	OtherChanges []string `json:"otherChanges"`
	Git          bool     `json:"git"` // false outside a git repository: no changes known
	Next         []Next   `json:"next"`
	// Update is a newer hecatron release the last check found, from its
	// cache only (no network); nil when none is known.
	Update *Update `json:"update"`
}

// Update is a newer release of hecatron than this one.
type Update struct {
	Latest  string    `json:"latest"`
	Checked time.Time `json:"checked"` // when the check that found it ran
	Command string    `json:"command"` // what installs it
}

// SetUpdate records a newer release, and suggests installing it (asking
// the user first: a new version can change generated files).
func (s *Status) SetUpdate(latest string, checked time.Time) {
	if latest == "" {
		return
	}
	s.Update = &Update{Latest: latest, Checked: checked.UTC(), Command: "hct self update"}
	s.Next = append(s.Next, Next{"hct self update", "hecatron " + latest + " is available", true})
}

// Language is one of the project's languages.
type Language struct {
	Name       string      `json:"name"`
	Disabled   bool        `json:"disabled"`
	Components []Component `json:"components"`
	Versions   []Version   `json:"versions"`
	// VersionError is why the version couldn't be resolved, as hecatron
	// says it ("not a version: ..."), else "".
	VersionError string `json:"versionError,omitempty"`
	// Dependency is the command the toolchain gives (go, cargo, uv); ""
	// when the definition names none.
	Dependency string `json:"dependency"`
	// Toolchain is "installed" (on every component's PATH), "missing" (on
	// none), "partly", or "none" (nothing to check).
	Toolchain string         `json:"toolchain"`
	Runs      map[string]Run `json:"runs"`
	Changes   []string       `json:"changes"` // changed files, relative to the root
}

// Component is a component implemented in the language.
type Component struct {
	Name      string `json:"name"`
	Dir       string `json:"dir"` // relative to the root, "." for the root
	Toolchain bool   `json:"toolchainFound"`
	// VersionMissing is the version the component resolves to when none
	// installed matches it, as hct doctor reports it ("=24.22.1"), else "".
	VersionMissing string `json:"versionMissing,omitempty"`
	// RangeRun is the installed version the component runs instead of the
	// default, which none of is installed, because its rangeFiles allow it
	// (package.json's engines.node); nil otherwise.
	RangeRun *RangeRun `json:"rangeRun,omitempty"`
}

// RangeRun is a version a component runs because its range allows it.
type RangeRun struct {
	Version string `json:"version"` // what runs: "26.13.0"
	Allowed string `json:"allowed"` // the range: ">=22"
	File    string `json:"file"`    // where it's written: "clients/web/package.json"
}

// Version is a toolchain version the components resolve to.
type Version struct {
	Spec    string   `json:"spec"` // "" when nothing names one
	Exact   bool     `json:"exact"`
	Sources []string `json:"sources"` // as the version queries give them: "pin:go/go.mod", "default"...
}

// Run is an action's last run for the language.
type Run struct {
	Status   string    `json:"status"` // "passed", "failed", "skipped" (nothing ran: no tests, no tool) or "unknown"
	Finished time.Time `json:"finished"`
	Duration *float64  `json:"durationSeconds"` // when history.log recorded that run
	Log      string    `json:"log"`
	ago      time.Duration
}

// Next is a command worth running next.
type Next struct {
	Command string `json:"command"`
	Why     string `json:"why"`
	AskUser bool   `json:"askUser"` // it installs something: ask the user first
}

// Gather collects the project's status. env is the environment hooks start
// from (PATH, HOME); now is the time runs are reported against.
func Gather(p *project.Project, env []string, now time.Time) (*Status, error) {
	s := &Status{Version: p.Home.Version(), Runner: "binary", Root: p.Root, Config: "saved",
		Languages: []Language{}, OtherChanges: []string{}}
	if p.Detected {
		s.Config = "detected"
	}
	p.Environ = env
	changes, inGit := changedFiles(p.Root)
	s.Git = inGit
	owners, err := p.Owners()
	if err != nil {
		return nil, err
	}
	byLang := map[string][]string{}
	for _, file := range changes {
		if lang, _, ok := owners.Owner(project.PyJoin(p.Root, file)); ok {
			byLang[lang] = append(byLang[lang], file)
		} else {
			s.OtherChanges = append(s.OtherChanges, file)
		}
	}
	for _, lang := range p.Languages {
		l, err := gatherLanguage(p, lang, env, now)
		if err != nil {
			return nil, err
		}
		l.Changes = append([]string{}, byLang[lang]...)
		s.Languages = append(s.Languages, l)
	}
	s.Next = suggest(s)
	return s, nil
}

func gatherLanguage(p *project.Project, lang string, env []string, now time.Time) (Language, error) {
	l := Language{Name: lang, Disabled: contains(p.Disabled, lang), Runs: map[string]Run{},
		Components: []Component{}, Versions: []Version{}}
	dirs, err := p.ImplDirs(lang)
	if err != nil {
		return l, err
	}
	if l.Dependency, err = p.Dependency(lang); err != nil {
		return l, err
	}
	versions, err := p.Versions(lang)
	switch e := err.(type) {
	case nil:
		for _, v := range versions {
			l.Versions = append(l.Versions, Version{strings.TrimLeft(v.Spec, "="), strings.HasPrefix(v.Spec, "="), append([]string{}, v.Sources...)})
		}
	case *project.Error:
		l.VersionError = e.Message
	default:
		if !project.IsUnsupported(err) {
			return l, err
		}
		l.VersionError = "resolved by project.py only (hct doctor " + lang + " says it)"
	}
	missing := map[string]string{}
	if list, err := p.MissingVersions(lang); err == nil {
		for _, m := range list {
			missing[m.Component] = m.Spec
		}
	}
	found := 0
	for _, d := range dirs {
		c := Component{Name: d.Component, Dir: project.PyRelpath(d.Dir, p.Root), VersionMissing: missing[d.Component]}
		if version, allowed, where, ok, err := p.RangeRun(lang, d.Component); err == nil && ok {
			c.RangeRun = &RangeRun{version, allowed, where}
		}
		if l.Dependency != "" {
			c.Toolchain = toolchainFound(p, lang, d.Component, l.Dependency, env)
			if c.Toolchain {
				found++
			}
		}
		l.Components = append(l.Components, c)
	}
	switch {
	case l.Dependency == "" || len(dirs) == 0:
		l.Toolchain = "none"
	case found == len(dirs):
		l.Toolchain = "installed"
	case found == 0:
		l.Toolchain = "missing"
	default:
		l.Toolchain = "partly"
	}
	for _, action := range Actions {
		if run, ok := lastRun(p, action, lang, now); ok {
			l.Runs[action] = run
		}
	}
	return l, nil
}

// toolchainFound is whether the language's command is on the PATH the
// component's hooks run with (its version's environment).
func toolchainFound(p *project.Project, lang, component, dependency string, env []string) bool {
	path := getenv(env, "PATH")
	if languageEnv, err := p.LanguageEnv(lang, component); err == nil {
		path = getenv(languageEnv.Apply(env), "PATH")
	} else if static, err := p.StaticPath(lang); err == nil && len(static) > 0 {
		path = strings.Join(static, ":") + ":" + path
	}
	return lookPath(dependency, path)
}

func getenv(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(env[i], name+"="); ok {
			return value
		}
	}
	return ""
}

func lookPath(name, path string) bool {
	if strings.ContainsRune(name, '/') {
		return isExecutable(name)
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		if isExecutable(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// LastRun is an action's last run for a language, as status reports it.
func LastRun(p *project.Project, action, lang string, now time.Time) (Run, bool) {
	return lastRun(p, action, lang, now)
}

// lastRun is an action's last run for a language: when its log was last
// written, passed or failed from the summary line the log ends with (both
// runners write it, for named runs too), and the duration when history.log's
// last run is the one that wrote the log.
func lastRun(p *project.Project, action, lang string, now time.Time) (Run, bool) {
	dir := filepath.Join(p.Logs, action)
	log := filepath.Join(dir, lang+".log")
	info, err := os.Stat(log)
	if err != nil || !info.Mode().IsRegular() {
		return Run{}, false
	}
	run := Run{Status: logStatus(log, lang, action), Finished: info.ModTime().UTC(), Log: log, ago: now.Sub(info.ModTime())}
	if history, err := os.Stat(filepath.Join(dir, "history.log")); err == nil && !info.ModTime().After(history.ModTime()) {
		if duration, code, ok := runner.ReadHistory(filepath.Join(dir, "history.log")).Last(lang); ok {
			seconds := duration.Seconds()
			run.Duration = &seconds
			if run.Status == "unknown" {
				run.Status = historyStatus(code)
			}
		}
	}
	return run, true
}

// historyStatus is a status history.log records, as status reports it.
func historyStatus(code int) string {
	switch code {
	case 0:
		return "passed"
	case 77: // runner's skippedStatus
		return "skipped"
	}
	return "failed"
}

// logStatus reads the line a language job's log ends with:
// "<lang>: <action> passed for N component(s)", "<lang>: <action> failed:
// ...", or, when every component (or the job) had nothing to do,
// "<lang>: <action> skipped for N component(s): ..." or "<lang>: <action>
// skipped: ...". A job some of whose components skipped ends with its
// passed line.
func logStatus(path, lang, action string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	prefix := lang + ": " + action + " "
	status := "unknown"
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), prefix)
		switch {
		case !ok:
		case strings.HasPrefix(line, "passed for "):
			status = "passed"
		case strings.HasPrefix(line, "failed: "):
			status = "failed"
		case strings.HasPrefix(line, "skipped: "), skippedFor.MatchString(line):
			status = "skipped"
		}
	}
	return status
}

// skippedFor is the rest of each-component.sh's line naming the
// components that had nothing to do.
var skippedFor = regexp.MustCompile(`^skipped for \d+ component\(s\): `)

// changedFiles are the files git status lists as changed or untracked
// under root, relative to it, from one `git status`, less those in another
// repository or worktree below root (project.OutsideNestedRepos); false
// outside a git repository.
func changedFiles(root string) ([]string, bool) {
	top := gitTop(root)
	if top == "" {
		return nil, false
	}
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	prefix, _ := filepath.Rel(top, root)
	var files []string
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		code, path := entry[:2], entry[3:]
		if code[0] == 'R' || code[0] == 'C' {
			i++ // the original path follows a rename or copy
		}
		if prefix != "." {
			var ok bool
			if path, ok = strings.CutPrefix(path, prefix+"/"); !ok {
				continue
			}
		}
		files = append(files, path)
	}
	sort.Strings(files)
	return project.OutsideNestedRepos(root, files), true
}

// gitTop is the top of the working tree holding dir, found as git does
// (a .git directory or file), without running git.
func gitTop(dir string) string {
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// suggest picks the commands worth running next: installing a missing
// toolchain (which needs the user's approval), rerunning what failed, and
// testing what changed.
func suggest(s *Status) []Next {
	var next []Next
	for _, l := range s.Languages {
		if l.Toolchain == "missing" || l.Toolchain == "partly" {
			next = append(next, Next{"hct install " + l.Name, l.Dependency + " not found; installs a toolchain", true})
		} else if versionsMissing(l) != "" {
			next = append(next, Next{"hct install " + l.Name, "installs " + versionsMissing(l) + ", which hct doctor finds missing", true})
		}
	}
	for _, l := range s.Languages {
		tested := false
		for _, action := range []string{"test", "lint", "build"} {
			if run, ok := l.Runs[action]; ok && run.Status == "failed" {
				why := "failed last run"
				if len(l.Changes) > 0 {
					why += "; has changes"
				}
				next = append(next, Next{"hct " + action + " " + l.Name, why, false})
				tested = tested || action == "test"
			}
		}
		if !tested && len(l.Changes) > 0 {
			next = append(next, Next{"hct test " + l.Name, "has changes", false})
		}
	}
	if len(next) == 0 && len(s.Languages) > 0 {
		next = append(next, Next{"hct test", "every language's tests", false})
	}
	if len(s.Languages) == 0 {
		next = append(next, Next{"hct add <lang>", "no languages: add one", false})
	}
	return next
}

// versionsMissing names the components whose version isn't installed, as
// hct doctor finds them ("24.22.1 (web)"), "" for none.
func versionsMissing(l Language) string {
	var parts []string
	for _, c := range l.Components {
		if c.VersionMissing != "" {
			parts = append(parts, strings.TrimLeft(c.VersionMissing, "=")+" ("+c.Name+")")
		}
	}
	return strings.Join(parts, ", ")
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}
