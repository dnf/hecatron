package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dnf/hecatron/internal/changed"
	"github.com/dnf/hecatron/internal/generate"
	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
)

const syncUsage = `Usage: hecatron sync [ref] [--no-deps] [--output human|agent|json]

After the working tree moved (a pull, a rebase, a checkout): what changed
since ref (default: where HEAD was before, from the reflog) means for
hecatron, in a few lines. Changed lock files: runs hct deps for those
languages and components (--no-deps: only says so). Changed pin or support
files: the toolchain version each component now needs and whether it's
installed; it never installs. Changed configuration, hooks or generated
files: runs hct check. Never pulls, rebases or fetches. Exit status 1 when
hct deps or hct check fails. The output format is chosen as for hct test.
`

// syncReport is what hct sync found and did; the JSON output.
type syncReport struct {
	Note       string          `json:"note,omitempty"` // nothing to compare with, and why
	Base       *syncBase       `json:"base,omitempty"`
	Head       string          `json:"head,omitempty"`
	Files      []string        `json:"files"`
	Deps       *syncDeps       `json:"deps,omitempty"`
	Toolchains []syncToolchain `json:"toolchains"`
	Check      *syncCheck      `json:"check,omitempty"`
	Nothing    bool            `json:"nothing"` // nothing changed that hecatron acts on
	Status     int             `json:"exitCode"`
}

type syncBase struct {
	Ref      string `json:"ref"`
	Source   string `json:"source"` // "named" or "the reflog"
	Commit   string `json:"commit"`
	Commits  int    `json:"commits"`  // on HEAD since the base
	Ancestor bool   `json:"ancestor"` // the base is an ancestor of HEAD
}

type syncLanguage struct {
	Name       string   `json:"name"`
	Components []string `json:"components"`
	Files      []string `json:"files"`
	Status     string   `json:"status,omitempty"` // deps: passed, failed, timed out, not installed, disabled, no hook, not run
	all        bool
}

type syncDeps struct {
	Languages []syncLanguage  `json:"languages"`
	Ran       bool            `json:"ran"` // false with --no-deps
	Command   string          `json:"command"`
	Run       json.RawMessage `json:"run,omitempty"` // hct deps --output json's report
	report    *runner.Report
}

type syncToolchain struct {
	syncLanguage
	Versions []syncVersion `json:"versions"`
	Error    string        `json:"error,omitempty"`
	Install  *syncInstall  `json:"install,omitempty"`
}

// syncInstall is the command that installs a missing toolchain, which
// needs the user's approval.
type syncInstall struct {
	Command string `json:"command"`
	AskUser bool   `json:"askUser"`
}

type syncVersion struct {
	Components []string `json:"components"`
	Spec       string   `json:"spec"` // "" when nothing names one
	Exact      bool     `json:"exact"`
	Source     string   `json:"source"`
	Installed  *bool    `json:"installed"`                  // nil: can't tell (no installed.sh)
	Version    string   `json:"installedVersion,omitempty"` // the installed version that matches
}

type syncCheck struct {
	Files  []string `json:"files"`
	Passed bool     `json:"passed"`
	Output string   `json:"output"`
}

// sync is `hecatron sync [ref]` (#92). It exists only in the Go core.
func (c *cli) sync() int {
	ref, flag, noDeps := "", "", false
	for i := 0; i < len(c.args); i++ {
		switch arg := c.args[i]; {
		case arg == "help" || arg == "-h" || arg == "--help":
			fmt.Print(syncUsage)
			return 0
		case arg == "--no-deps":
			noDeps = true
		case arg == "--output":
			if i+1 >= len(c.args) {
				fmt.Fprintln(os.Stderr, "hecatron: --output needs a format: human, agent or json")
				return 2
			}
			i++
			flag = c.args[i]
		case strings.HasPrefix(arg, "--output="):
			flag = strings.TrimPrefix(arg, "--output=")
		case !strings.HasPrefix(arg, "-") && ref == "":
			ref = arg
		default:
			fmt.Fprintf(os.Stderr, "hecatron: sync: unexpected argument '%s'\n%s", arg, syncUsage)
			return 2
		}
	}
	format, err := runner.SelectFormat(flag, os.Getenv, runner.IsTerminal(os.Stdout))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 2
	}
	if status := c.findProject(); status != 0 {
		return status
	}
	if c.project == nil {
		fmt.Fprintln(os.Stderr, "hecatron: sync can't read this project's configuration yet; hct deps and hct check can")
		return 1
	}
	s := &syncSession{cli: c, format: format, noDeps: noDeps, out: os.Stdout}
	return s.run(ref)
}

// syncSession is one hct sync.
type syncSession struct {
	*cli
	format runner.Format
	noDeps bool
	out    io.Writer
	report syncReport
}

func (s *syncSession) json() bool { return s.format == runner.JSON }

// say writes a line of human or agent output.
func (s *syncSession) say(format string, args ...any) {
	if !s.json() {
		_, _ = fmt.Fprintf(s.out, format+"\n", args...)
	}
}

func (s *syncSession) run(ref string) int {
	s.report = syncReport{Files: []string{}, Toolchains: []syncToolchain{}}
	status := s.gather(ref)
	s.report.Status = status
	if s.json() && status != 2 {
		encoder := json.NewEncoder(s.out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(s.report); err != nil {
			fmt.Fprintln(os.Stderr, "hecatron:", err)
			return 1
		}
	}
	return status
}

func (s *syncSession) gather(ref string) int {
	root := s.root
	base, note, err := changed.SyncBase(root, ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		if _, usage := err.(*changed.UsageError); usage {
			return 2
		}
		return 1
	}
	if note != "" {
		s.report.Note = note
		s.say("hct sync: %s", note)
		return 0
	}
	head, err := changed.Head(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	commits, ancestor, err := changed.Commits(root, base.Commit, head)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	s.report.Base = &syncBase{Ref: base.Ref, Source: base.Source, Commit: base.Commit, Commits: commits, Ancestor: ancestor}
	s.report.Head = head
	files, err := changed.Between(root, base.Commit, head)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	s.report.Files = append(s.report.Files, files...)
	if len(files) == 0 {
		s.report.Nothing = true
		s.say("hct sync: nothing changed since %s (%s)", changed.Short(base.Commit), base.Ref)
		return 0
	}
	since := fmt.Sprintf("%d commit%s", commits, plural(commits))
	if !ancestor {
		since = "not an ancestor of HEAD"
	}
	head1 := fmt.Sprintf("hct sync: since %s (%s, %s): %d file%s changed", changed.Short(base.Commit), base.Ref, since, len(files), plural(len(files)))
	p := s.project
	p.Environ = s.environment(nil)
	catchup, err := changed.Catch(p, files, s.generatedFiles())
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	if catchup.Empty() {
		s.report.Nothing = true
		s.say("%s; nothing for hecatron to do", head1)
		return 0
	}
	s.say("%s", head1)
	status := 0
	if len(catchup.Deps) > 0 {
		status = max(status, s.deps(catchup.Deps))
	}
	for _, t := range catchup.Toolchains {
		s.toolchain(t)
	}
	if len(catchup.Check) > 0 {
		status = max(status, s.check(catchup.Check))
	}
	if d := s.report.Deps; d != nil && d.report != nil && runner.ExitStatus(d.report.Results) != 0 && !s.json() {
		runner.WriteAgent(s.out, d.report)
	}
	return status
}

// generatedFiles are the files hecatron generates for the project,
// relative to the root.
func (s *syncSession) generatedFiles() []string {
	p := s.project
	var files []string
	if dockerfile, err := p.Generated("dockerfile"); err == nil && dockerfile != "" {
		files = append(files, project.PyRelpath(project.PyJoin(p.Root, dockerfile), p.Root))
	}
	if github, err := s.generatesGitHub(); err == nil && github {
		for _, lang := range p.Languages {
			if path, err := generate.WorkflowPath(p, lang); err == nil && path != "" {
				files = append(files, project.PyRelpath(path, p.Root))
			}
		}
	}
	if contains(s.ciSettings().Systems, project.CIGitLab) {
		files = append(files, generate.GitLabPath)
	}
	updates, _ := p.UpdateSettings()
	for _, path := range []string{updates.Dependabot, updates.Renovate} {
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}

// touched is a language as a line names it: with its components when not
// all, and the first file.
func touched(name string, components []string, all bool, files []string) string {
	if !all {
		name += " [" + strings.Join(components, ", ") + "]"
	}
	return name + " (" + files[0] + more(len(files)) + ")"
}

func more(n int) string {
	if n > 1 {
		return fmt.Sprintf(" +%d", n-1)
	}
	return ""
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// components are the components hct deps would need -c for: none when
// every language touched has all its components.
func (d *syncDeps) components() []string {
	var components []string
	all := true
	for _, l := range d.Languages {
		all = all && l.all
		for _, component := range l.Components {
			if !contains(components, component) {
				components = append(components, component)
			}
		}
	}
	if all {
		return nil
	}
	return components
}

// command is the hct deps command for the languages touched.
func (d *syncDeps) command() string {
	words := []string{"hct", "deps"}
	for _, l := range d.Languages {
		words = append(words, l.Name)
	}
	for _, component := range d.components() {
		words = append(words, "-c", component)
	}
	return strings.Join(words, " ")
}

// deps runs hct deps for the languages and components whose lock files
// changed, as the runner runs it (time limits, logs, history), unless
// --no-deps.
func (s *syncSession) deps(touchedDeps []changed.Touched) int {
	d := &syncDeps{}
	for _, t := range touchedDeps {
		d.Languages = append(d.Languages, syncLanguage{Name: t.Name, Components: t.Components, Files: t.Files, all: t.All})
	}
	d.Command = d.command()
	s.report.Deps = d
	if s.noDeps {
		var names []string
		for i := range d.Languages {
			l := &d.Languages[i]
			l.Status = "not run"
			names = append(names, touched(l.Name, l.Components, l.all, l.Files))
		}
		s.say("  deps: %s changed: run %s", strings.Join(names, ", "), d.Command)
		return 0
	}
	if status := s.requirePython(); status != 0 {
		return status
	}
	report, status := s.runDeps(touchedDeps)
	if report == nil {
		return status
	}
	d.Ran, d.report = true, report
	var buffer bytes.Buffer
	if err := runner.WriteJSON(&buffer, report); err == nil {
		d.Run = json.RawMessage(bytes.TrimSpace(buffer.Bytes()))
	}
	results := map[string]runner.Status{}
	for _, result := range report.Results {
		if result.Lang != "" {
			results[result.Lang] = result.Status
		}
	}
	var order []string
	byStatus := map[string][]string{}
	for i := range d.Languages {
		l := &d.Languages[i]
		l.Status = "no hook"
		if result, ok := results[l.Name]; ok {
			l.Status = string(result)
		}
		if byStatus[l.Status] == nil {
			order = append(order, l.Status)
		}
		byStatus[l.Status] = append(byStatus[l.Status], touched(l.Name, l.Components, l.all, l.Files))
	}
	var parts []string
	for _, st := range order {
		parts = append(parts, strings.Join(byStatus[st], ", ")+": "+depsOutcome[st])
	}
	s.say("  deps: %s", strings.Join(parts, "; "))
	return status
}

// depsOutcome is how a deps job's status reads in sync's line.
var depsOutcome = map[string]string{
	string(runner.Passed):           "fetched",
	string(runner.Failed):           "failed",
	string(runner.TimedOut):         "timed out",
	string(runner.StatusNotInstall): "toolchain not installed",
	string(runner.StatusDisabled):   "disabled (hct enable)",
	"no hook":                       "no deps hook",
}

// runDeps runs the deps jobs in parallel, as hct deps does; a nil report
// with the exit status when it couldn't.
func (s *syncSession) runDeps(touchedDeps []changed.Touched) (*runner.Report, int) {
	shell, err := s.project.Shell()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: sync can't run hct deps for this project; run it yourself:", err)
		return nil, 1
	}
	timeout, err := runner.SelectTimeout("", false, os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return nil, 2
	}
	c := *s.cli
	c.command = "deps"
	selection := &changed.Selection{}
	for _, t := range touchedDeps {
		selection.Languages = append(selection.Languages, changed.Language{Name: t.Name, Components: t.Components})
	}
	r := &invocation{cli: &c, shell: shell, format: s.format, timeout: timeout, patterns: map[string]*runner.Patterns{}, selection: selection}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r.ctx = ctx
	r.project.Environ = r.environment()
	jobs := r.jobs()
	if len(jobs) == 0 {
		return &runner.Report{Action: "deps", LogDir: r.logDir(), Root: r.root}, 0
	}
	if err := os.MkdirAll(r.logDir(), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return nil, 1
	}
	unlock, err := runner.Lock(r.logDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hecatron deps is already running (lock: %s)\n", err)
		return nil, 1
	}
	defer unlock()
	r.limit(jobs)
	options := runner.Options{LogDir: r.logDir()}
	var live *runner.Live
	if s.format == runner.Human && runner.IsTerminal(os.Stdout) && runner.Runnables(jobs) > 0 {
		live = runner.NewLive(os.Stdout, jobs)
		options.Observer = live
		live.Start()
	}
	results, wall := runner.Run(ctx, jobs, options)
	if live != nil {
		live.Stop()
	}
	if ctx.Err() != nil {
		return nil, 130
	}
	report := r.report(results, wall)
	report.Changed = nil // sync says what changed itself
	report.Components = s.report.Deps.components()
	return report, runner.ExitStatus(results)
}

// toolchain says the version each touched component now needs and whether
// it's installed. It never installs: hct install changes the machine, so
// it's suggested, for the user to approve.
func (s *syncSession) toolchain(t changed.Touched) {
	p := s.project
	report := syncToolchain{syncLanguage: syncLanguage{Name: t.Name, Components: t.Components, Files: t.Files, all: t.All}, Versions: []syncVersion{}}
	defer func() { s.report.Toolchains = append(s.report.Toolchains, report) }()
	head := touched(t.Name, t.Components, t.All, t.Files)
	checkable := p.Resolve(t.Name, "installed.sh") != ""
	for _, component := range t.Components {
		resolved, err := p.ResolveVersion(t.Name, component)
		var installed project.Installed
		found := false
		if err == nil && checkable {
			installed, found, err = p.InstalledVersion(t.Name, component)
		}
		if err != nil {
			report.Error = versionError(err)
			s.say("  toolchain: %s: the version can't be resolved: %s", head, report.Error)
			return
		}
		v := syncVersion{Spec: strings.TrimLeft(resolved.Spec, "="), Exact: resolved.Exact(), Source: resolved.Source}
		if checkable {
			v.Installed = &found
			v.Version = installed.Version
		}
		merged := false
		for i := range report.Versions {
			w := &report.Versions[i]
			if w.Spec == v.Spec && w.Source == v.Source && sameInstalled(w.Installed, v.Installed) {
				w.Components = append(w.Components, component)
				merged = true
				break
			}
		}
		if !merged {
			v.Components = []string{component}
			report.Versions = append(report.Versions, v)
		}
	}
	var parts []string
	missing := false
	for _, v := range report.Versions {
		part := "needs " + v.Spec
		if v.Spec == "" {
			part = "names no version"
		}
		if v.Source != "" && !contains(t.Files, strings.TrimPrefix(v.Source, "pin:")) {
			part += " (" + v.Source + ")"
		}
		switch {
		case v.Installed == nil:
			part += ", installed: can't tell (hct doctor " + t.Name + ")"
		case *v.Installed:
			part += ", installed"
			if v.Version != v.Spec {
				part += " (" + v.Version + ")"
			}
		case v.Spec == "":
		default:
			part += ", not installed"
			missing = true
		}
		if len(report.Versions) > 1 {
			part = "[" + strings.Join(v.Components, ", ") + "] " + part
		}
		parts = append(parts, part)
	}
	line := fmt.Sprintf("  toolchain: %s: %s", head, strings.Join(parts, "; "))
	if missing {
		report.Install = &syncInstall{Command: "hct install " + t.Name, AskUser: true}
		line += "; hct install " + t.Name + " installs it (ask the user first)"
	}
	s.say("%s", line)
}

func sameInstalled(a, b *bool) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

func versionError(err error) string {
	if e, ok := err.(*project.Error); ok {
		return e.Message
	}
	if project.IsUnsupported(err) {
		return "resolved by project.py only (hct doctor says it)"
	}
	return err.Error()
}

// check runs hct check (this binary, so a check it leaves to manage.py is
// delegated as usual) and reports it.
func (s *syncSession) check(files []string) int {
	result := &syncCheck{Files: files}
	s.report.Check = result
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	cmd := exec.Command(exe, "check")
	cmd.Dir, cmd.Env = s.root, s.environment([]string{"HECATRON_OUTPUT="})
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err = cmd.Run()
	result.Output = output.String()
	result.Passed = err == nil
	why := files[0] + more(len(files)) + " changed"
	if result.Passed {
		s.say("  check: %s; hct check passed", why)
		return 0
	}
	if _, exited := err.(*exec.ExitError); !exited {
		result.Output += err.Error() + "\n"
	}
	s.say("  check: %s; hct check failed:", why)
	for _, line := range strings.Split(strings.TrimRight(result.Output, "\n"), "\n") {
		s.say("    %s", line)
	}
	return 1
}
