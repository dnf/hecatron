package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
)

// The run commands (test, lint, build, clean, rebuild, retest, audit,
// format, deps, outdated), as run.sh runs them: named targets one at a
// time, streaming their output; none, every language and project task in
// parallel with internal/runner. Each language job runs the language's hook
// for each component as each-component.sh does (hooks.go). What run.sh
// would answer with an error of its own (an unknown or unadded language)
// and outdated --json are delegated to it.

// runArgs are a run command's arguments, as run.sh reads them.
type runArgs struct {
	jobs, display string
	components    []string
	targets       []string
	output        string
	env           []string // what the flags export to the hooks
	outdatedJSON  bool
	rest          []string // the arguments without --output, for run.sh
}

func parseRunArgs(action string, args []string) (runArgs, error) {
	var a runArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case arg == "-j":
			a.jobs = value()
		case arg == "-s":
			a.display = value()
		case arg == "-c":
			if i+1 < len(args) {
				a.components = append(a.components, value())
			}
		case strings.HasPrefix(arg, "--component="):
			a.components = append(a.components, strings.TrimPrefix(arg, "--component="))
		case arg == "--output":
			if i+1 >= len(args) {
				return a, fmt.Errorf("--output needs a format: human, agent, json or ci")
			}
			a.output = value()
		case strings.HasPrefix(arg, "--output="):
			a.output = strings.TrimPrefix(arg, "--output=")
			if a.output == "" {
				return a, fmt.Errorf("--output needs a format: human, agent, json or ci")
			}
		case arg == "--fail-on" && action == "audit":
			a.env = append(a.env, "HECATRON_AUDIT_FAIL_ON="+value())
		case strings.HasPrefix(arg, "--fail-on="):
			a.env = append(a.env, "HECATRON_AUDIT_FAIL_ON="+strings.TrimPrefix(arg, "--fail-on="))
		case arg == "--check" && action == "format":
			a.env = append(a.env, "HECATRON_FORMAT_CHECK=1")
		case arg == "--json" && action == "outdated":
			a.outdatedJSON = true
		case arg == "--fail" && action == "outdated":
			a.env = append(a.env, "HECATRON_OUTDATED_FAIL=1")
		default:
			a.targets = append(a.targets, arg)
		}
	}
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--output":
			i++
		case strings.HasPrefix(args[i], "--output="):
		default:
			a.rest = append(a.rest, args[i])
		}
	}
	return a, nil
}

// runCommand is a run command in Go; handled is false for what it leaves
// to run.sh.
func (c *cli) runCommand() (int, bool) {
	a, err := parseRunArgs(c.command, c.args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 2, true
	}
	shell, err := c.project.Shell()
	if err != nil {
		return 0, false
	}
	for _, component := range a.components {
		if !contains(c.project.Components, component) {
			fmt.Fprintf(os.Stderr, "hecatron: unknown component '%s' (components: %s)\n", component,
				strings.Join(c.project.Components, " "))
			return 2, true
		}
	}
	if len(a.targets) > 0 && isHelp(a.targets[0]) {
		text, ok := c.runHelpText()
		if !ok {
			return 0, false
		}
		fmt.Print(text)
		return 0, true
	}
	if a.outdatedJSON {
		c.args = a.rest
		return 0, false
	}
	format, err := runner.SelectFormat(a.output, os.Getenv, runner.IsTerminal(os.Stdout))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 2, true
	}
	r := &invocation{cli: c, shell: shell, args: a, format: format, patterns: map[string]*runner.Patterns{}}
	if !r.resolveTargets() {
		return 0, false
	}
	if status := c.requirePython(); status != 0 {
		return status, true
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r.ctx = ctx
	status := r.run()
	if ctx.Err() != nil {
		return 130, true
	}
	return status, true
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

// invocation is one run command's invocation.
type invocation struct {
	*cli
	ctx      context.Context
	shell    *project.Shell
	args     runArgs
	format   runner.Format
	named    []runner.Job // the targets, resolved
	patterns map[string]*runner.Patterns
}

// resolveTargets turns the targets into jobs as run.sh's _run_target would
// pick them: a project task, lint's shell, a language by name or
// abbreviation. False when a target isn't one (run.sh says why).
func (r *invocation) resolveTargets() bool {
	tasks, err := r.project.Tasks(r.command)
	if err != nil {
		return false
	}
	names := map[string]string{}
	for lang, abbreviation := range r.shell.Abbreviations {
		names[abbreviation] = lang
	}
	for _, target := range r.args.targets {
		label := target
		if lang, ok := names[target]; ok {
			label = lang
		}
		job := runner.Job{Label: label}
		if command, ok := taskCommand(tasks, target); ok {
			job.Task, job.Exec = true, r.taskExec(command)
		} else if r.command == "lint" && target == "shell" {
			job.Exec = r.shellcheckExec()
		} else if contains(r.shell.Added, label) {
			job.Lang, job.Exec = label, r.languageExec(r.command, label, r.args.components)
		} else {
			return false
		}
		r.named = append(r.named, job)
	}
	return true
}

func taskCommand(tasks []project.Task, name string) (string, bool) {
	for _, task := range tasks {
		if task.Name == name {
			return task.Command, true
		}
	}
	return "", false
}

func (r *invocation) run() int {
	switch r.command {
	case "rebuild", "retest":
		then := map[string]string{"rebuild": "build", "retest": "test"}[r.command]
		clean := r.sub("clean")
		if status := clean.run(); status != 0 {
			return status
		}
		return r.sub(then).run()
	}
	if len(r.named) > 0 {
		return r.runNamed()
	}
	return r.runAll()
}

// sub is the same invocation for another action (rebuild's clean, then
// build).
func (r *invocation) sub(action string) *invocation {
	c := *r.cli
	c.command = action
	s := &invocation{cli: &c, ctx: r.ctx, shell: r.shell, args: r.args, format: r.format, patterns: r.patterns}
	s.resolveTargets()
	return s
}

// environment is what the hooks and tasks run with: bin/hecatron's and
// helpers.sh's exports, run.sh's, and the flags'.
func (r *invocation) environment() []string {
	extra := []string{"HECATRON_JOB_COMMAND=" + r.homePath("bin/hecatron")}
	for _, export := range r.shell.Exports {
		extra = append(extra, export[0]+"="+export[1])
	}
	return r.cli.environment(append(extra, r.args.env...))
}

func (r *invocation) logDir() string { return filepath.Join(r.project.Logs, r.command) }

// runNamed runs the targets one at a time. Human (and CI) output streams
// each one's output as run.sh does; agent and JSON output keep it in the
// logs and report at the end.
func (r *invocation) runNamed() int {
	if err := os.MkdirAll(r.logDir(), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	stream := r.format == runner.Human || r.format == runner.CI
	start := time.Now()
	var results []runner.Result
	for _, job := range r.named {
		result := runner.Result{Label: job.Label, Lang: job.Lang, Task: job.Task,
			Log: filepath.Join(r.logDir(), job.Label+".log")}
		log, err := os.Create(result.Log)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hecatron:", err)
			return 1
		}
		var out io.Writer = log
		if stream {
			out = io.MultiWriter(os.Stdout, log)
		}
		began := time.Now()
		outcome := job.Exec(r.ctx, out)
		log.Close()
		result.Duration, result.ExitCode, result.Components = time.Since(began), outcome.ExitCode, outcome.Components
		result.Status = runner.Passed
		if outcome.ExitCode != 0 {
			result.Status = runner.Failed
		}
		results = append(results, result)
		if r.ctx.Err() != nil {
			break
		}
	}
	report := r.report(results, time.Since(start))
	switch r.format {
	case runner.Agent:
		runner.WriteAgent(os.Stdout, report)
	case runner.JSON:
		runner.WriteJSON(os.Stdout, report)
	case runner.CI:
		runner.WriteAnnotations(os.Stdout, report, os.Getenv("GITHUB_WORKSPACE"))
	}
	return runner.ExitStatus(results)
}

// report adds each failure's excerpt.
func (r *invocation) report(results []runner.Result, wall time.Duration) *runner.Report {
	for i := range results {
		if results[i].Status != runner.Failed {
			continue
		}
		if log, err := os.ReadFile(results[i].Log); err == nil {
			results[i].Excerpt = runner.Extract(log, r.errorPatterns(results[i].Lang))
		}
	}
	cwd, _ := workingDir()
	return &runner.Report{Action: r.command, Targets: r.args.targets, Components: r.args.components,
		Results: results, Wall: wall, LogDir: r.logDir(), Root: r.root, Cwd: cwd}
}

// errorPatterns are a language's, from its definition; nil for none.
func (r *invocation) errorPatterns(lang string) *runner.Patterns {
	if lang == "" {
		return nil
	}
	if p, ok := r.patterns[lang]; ok {
		return p
	}
	list, _ := r.project.DefinitionValue(lang, "errorPatterns")
	versions, _ := r.project.DefinitionValue(lang, "errorToolVersions")
	p, _ := runner.ParsePatterns(list, versions)
	if len(p.List) == 0 {
		p = nil
	}
	r.patterns[lang] = p
	return p
}

// jobs is the parallel job list, as run.sh builds it: each language the
// action covers (disabled, its toolchain missing, or run), then the
// project's tasks and lint's shell check.
func (r *invocation) jobs() []runner.Job {
	var jobs []runner.Job
	for _, lang := range r.shell.HookLanguages(runActions[r.command].hook) {
		job := runner.Job{Label: lang, Lang: lang}
		switch {
		case contains(r.shell.Disabled, lang):
			job.Kind = runner.Disabled
		case r.dependencyAvailable(lang):
			job.Exec = r.languageExec(r.command, lang, r.args.components)
		default:
			job.Kind = runner.NotInstalled
		}
		jobs = append(jobs, job)
	}
	var extra []runner.Job
	if len(r.args.components) == 0 {
		tasks, _ := r.project.Tasks(r.command)
		for _, task := range tasks {
			fields := strings.Fields(task.Name)
			label := ""
			if len(fields) > 0 {
				label = fields[len(fields)-1]
			}
			extra = append(extra, runner.Job{Label: strings.ReplaceAll(label, "/", "_"), Task: true, Exec: r.taskExec(task.Command)})
		}
	}
	if tools, _ := r.project.Tools(); r.command == "lint" && hasKey(tools, "shell") {
		job := runner.Job{Label: "shell", Kind: runner.NotInstalled}
		if _, found := lookPath("shellcheck", os.Getenv("PATH")); found {
			job.Kind, job.Exec = runner.Runnable, r.shellcheckExec()
		}
		extra = append(extra, job)
	}
	if len(extra) > 0 {
		jobs = append(append(jobs, runner.Job{Label: "---", Kind: runner.Separator}), extra...)
	}
	return jobs
}

func hasKey(m map[string]string, key string) bool {
	_, ok := m[key]
	return ok
}

// runAll runs every job in parallel.
func (r *invocation) runAll() int {
	jobs := r.jobs()
	if len(jobs) == 0 {
		if r.format == runner.JSON {
			runner.WriteJSON(os.Stdout, r.report(nil, 0))
		} else {
			fmt.Printf("Nothing to %s.\n", r.command)
		}
		return 0
	}
	slots, err := runner.ParseSlots(r.args.jobs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	switch r.args.display {
	case "", "scroll", "full", "zoom":
	default:
		fmt.Fprintf(os.Stderr, "error: -s accepts full, scroll, or zoom (got '%s')\n", r.args.display)
		return 2
	}
	if err := os.MkdirAll(r.logDir(), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	unlock, err := runner.Lock(r.logDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hecatron %s is already running (lock: %s)\n", r.command, err)
		return 1
	}
	defer unlock()

	human := r.format == runner.Human || r.format == runner.CI
	if runner.Runnables(jobs) == 0 {
		results, _ := runner.Run(r.ctx, jobs, runner.Options{LogDir: r.logDir()})
		report := r.report(results, 0)
		if human {
			runner.WriteNothingRan(os.Stdout, report)
		} else {
			r.writeReport(report)
		}
		return 0
	}
	var live *runner.Live
	options := runner.Options{Slots: slots, LogDir: r.logDir()}
	if human && runner.IsTerminal(os.Stdout) {
		live = runner.NewLive(os.Stdout, jobs)
		options.Observer = live
		live.Start()
	}
	results, wall := runner.Run(r.ctx, jobs, options)
	if live != nil {
		live.Stop()
	}
	report := r.report(results, wall)
	if human {
		if live == nil {
			runner.WriteChart(os.Stdout, report, runner.SlotMode(slots, jobs), chartWidth())
		}
		runner.WriteTally(os.Stdout, report)
	}
	r.writeReport(report)
	return runner.ExitStatus(results)
}

// writeReport writes what follows human output, or replaces it.
func (r *invocation) writeReport(report *runner.Report) {
	switch r.format {
	case runner.Agent:
		runner.WriteAgent(os.Stdout, report)
	case runner.JSON:
		runner.WriteJSON(os.Stdout, report)
	case runner.CI:
		runner.WriteAnnotations(os.Stdout, report, os.Getenv("GITHUB_WORKSPACE"))
	}
}

// chartWidth is the terminal's width as run-parallel.sh finds it off a
// terminal: stdin's terminal, else COLUMNS, else 80.
func chartWidth() int {
	if columns, _, ok := runner.TerminalSize(os.Stdin); ok && columns > 0 {
		return columns
	}
	var columns int
	if _, err := fmt.Sscan(os.Getenv("COLUMNS"), &columns); err == nil && columns > 0 {
		return columns
	}
	return 80
}
