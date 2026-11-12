package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
)

const execUsage = `Usage: hecatron exec LANG [-c COMPONENT] [--output agent] -- CMD [ARGS...]

Run CMD with LANG's environment for a component: its toolchain version's
PATH and variables, what 'hecatron env LANG -c COMPONENT' prints. CMD runs
in the current directory, under the project's wrapper (config.json
"wrapper") when it has one. Without -c, the component whose LANG directory
holds the current directory, else the only component in LANG. Exits with
CMD's status (127 when it isn't found).

  --output agent
             Keep CMD's output in <logs>/exec/<CMD>.log and print one line
             when it passes; when it fails, the log, its error lines (LANG's
             error patterns) and the command that reruns it, as a run
             command's agent output does (HECATRON_RERUN, when a script
             running hct exec for its users sets it, is that command)

  hct exec zig -c parser -- zig build
  hct exec python -- python -m pytest
  hct exec rust --output agent -- cargo test -p parser
`

// execNotInstalled is the exit status when the component's toolchain
// version isn't installed, as for a run where only toolchains were missing.
const execNotInstalled = 3

// execArgs are exec's arguments.
type execArgs struct {
	lang, component string
	output          runner.Format // "" (CMD's output as it comes) or runner.Agent
	argv            []string
}

// execCommand is `hecatron exec` (#225). It exists only in the Go core.
func (c *cli) execCommand() int {
	a, status := parseExecArgs(c.args)
	if status >= 0 {
		return status
	}
	lang, component, argv := a.lang, a.component, a.argv
	if status := c.findProject(); status != 0 {
		return status
	}
	p := c.project
	if p == nil {
		fmt.Fprintln(os.Stderr, "hecatron: exec can't read this project's configuration yet; eval \"$(hct env LANG -c COMPONENT)\" can")
		return 1
	}
	resolved, err := p.ResolveName(lang)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	if resolved == "" {
		fmt.Fprintf(os.Stderr, "hecatron: unknown language: %s (try 'hecatron list --all')\n", lang)
		return 2
	}
	lang = resolved
	if component == "" {
		if component, status = execComponent(p, lang); status != 0 {
			return status
		}
	} else if !contains(p.Components, component) {
		fmt.Fprintf(os.Stderr, "hecatron: unknown component '%s' (components: %s)\n", component, strings.Join(p.Components, " "))
		return 2
	}
	if status := checkInstalled(p, lang, component); status != 0 {
		return status
	}
	p.Environ = c.environment(nil)
	env, err := p.LanguageEnv(lang, component)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	environ := env.Apply(c.environment(nil))
	program, status := findProgram(argv[0], environ)
	if status != 0 {
		return status
	}
	if a.output == runner.Agent {
		return c.execForAgent(a, environ)
	}
	if len(p.Wrapper) > 0 {
		wrapped := p.Wrap(argv)
		if program, status = findProgram(wrapped[0], environ); status != 0 {
			return status
		}
		argv = wrapped
	}
	return replaceProcess(program, argv, environ)
}

// execForAgent runs the command as a job of its own, its output in
// <logs>/exec/<CMD>.log, and reports it as a run command's agent output
// does: a line when it passes; when it fails, the log, its error lines
// and the command that reruns it. Ctrl-C stops it, its process group
// with it.
func (c *cli) execForAgent(a execArgs, environ []string) int {
	logs := filepath.Join(c.project.Logs, "exec")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	label := execLabel(a.argv[0])
	result := runner.Result{Label: label, Lang: a.lang, Log: filepath.Join(logs, label+".log")}
	log, err := os.Create(result.Log)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cwd, _ := workingDir()
	job := runner.Job{Label: label, Kind: runner.Runnable, Lang: a.lang,
		Exec: func(ctx context.Context, out io.Writer) runner.Outcome {
			argv := c.project.Wrap(a.argv)
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
			cmd.Dir, cmd.Env = cwd, environ
			return runner.Outcome{ExitCode: runCommand(cmd, out)}
		}}
	began := time.Now()
	outcome, status := runner.Execute(ctx, job, log)
	if err := log.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
	}
	if ctx.Err() != nil {
		return 130
	}
	result.Duration, result.ExitCode, result.Status = time.Since(began), outcome.ExitCode, status
	if result.Unsuccessful() {
		if text, err := os.ReadFile(result.Log); err == nil {
			r := &invocation{cli: c, patterns: map[string]*runner.Patterns{}}
			result.Excerpt = runner.Extract(text, r.errorPatterns(a.lang))
		}
	}
	runner.WriteAgent(os.Stdout, &runner.Report{Action: "exec " + a.lang, Results: []runner.Result{result},
		Wall: result.Duration, LogDir: logs, Root: c.root, Cwd: cwd, RerunCommand: execRerun(a)})
	if result.Unsuccessful() && outcome.ExitCode != 0 {
		return outcome.ExitCode
	}
	return runner.ExitStatus([]runner.Result{result})
}

// execLabel names an exec's log after its command: cargo for
// /usr/bin/cargo, gradlew for ./gradlew.
func execLabel(command string) string {
	label := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == ' ' {
			return '_'
		}
		return r
	}, filepath.Base(command))
	if label == "" || label == "." || label == ".." {
		return "command"
	}
	return label
}

// execRerun is the command line that runs it again: HECATRON_RERUN, set by
// a script that runs hct exec for its own users and is how they run it
// again; else the hct exec command line, quoted for a POSIX shell.
func execRerun(a execArgs) string {
	if rerun := os.Getenv("HECATRON_RERUN"); rerun != "" {
		return rerun
	}
	words := []string{"hct", "exec", a.lang}
	if a.component != "" {
		words = append(words, "-c", a.component)
	}
	words = append(words, "--output", "agent", "--")
	for _, word := range a.argv {
		words = append(words, shellQuote(word))
	}
	return strings.Join(words, " ")
}

// shellQuote is word as a POSIX shell reads it back: as it is when it
// holds nothing special, else in single quotes.
func shellQuote(word string) string {
	if word != "" && strings.Trim(word, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./=:,+@%") == "" {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

// parseExecArgs reads LANG [-c COMPONENT] [--output FORMAT] -- CMD
// [ARGS...]; status is -1 to go on, else the status to exit with.
func parseExecArgs(args []string) (a execArgs, status int) {
	usageError := func(message string) (execArgs, int) {
		fmt.Fprintf(os.Stderr, "hecatron: exec: %s\n%s", message, execUsage)
		return execArgs{}, 2
	}
	i := 0
	for ; i < len(args) && args[i] != "--"; i++ {
		switch arg := args[i]; {
		case isHelp(arg) || arg == "-h":
			fmt.Print(execUsage)
			return execArgs{}, 0
		case arg == "-c":
			if i+1 >= len(args) {
				return usageError("-c needs a component")
			}
			i++
			a.component = args[i]
		case arg == "--output" || strings.HasPrefix(arg, "--output="):
			value, given := strings.CutPrefix(arg, "--output=")
			if !given {
				if i+1 >= len(args) {
					return usageError("--output needs a format")
				}
				i++
				value = args[i]
			}
			switch runner.Format(value) {
			case runner.Agent:
				a.output = runner.Agent
			case runner.Human:
				a.output = ""
			default:
				return usageError("--output takes agent or human, not '" + value + "'")
			}
		case strings.HasPrefix(arg, "-"):
			return usageError("unknown option '" + arg + "'")
		case a.lang == "":
			a.lang = arg
		default:
			return usageError("put the command after -- (" + arg + ")")
		}
	}
	switch {
	case a.lang == "":
		return usageError("name a language")
	case i+1 >= len(args):
		return usageError("give the command to run after --")
	}
	a.argv = args[i+1:]
	return a, -1
}

// execComponent is the component whose directory in lang holds the
// working directory (the innermost, when they nest), else the only one
// in lang (or the project's only component).
func execComponent(p *project.Project, lang string) (string, int) {
	dirs, err := p.ImplDirs(lang)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return "", 1
	}
	if cwd, err := workingDir(); err == nil {
		cwd = realPath(cwd)
		best, bestLength := "", -1
		for _, dir := range dirs {
			real := realPath(dir.Dir)
			if within(cwd, real) && len(real) > bestLength {
				best, bestLength = dir.Component, len(real)
			}
		}
		if best != "" {
			return best, 0
		}
	}
	var candidates []string
	for _, dir := range dirs {
		candidates = append(candidates, dir.Component)
	}
	if len(candidates) == 0 {
		candidates = p.Components
	}
	if len(candidates) == 1 {
		return candidates[0], 0
	}
	fmt.Fprintf(os.Stderr, "hecatron: exec: which component? Run it in one's %s directory or pass -c (components in %s: %s)\n",
		lang, lang, strings.Join(candidates, " "))
	return "", 2
}

func realPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// within is whether path is dir or below it.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// checkInstalled says when the component's resolved version of lang isn't
// installed, as the doctor does, and how to install it.
func checkInstalled(p *project.Project, lang, component string) int {
	missing, err := p.MissingVersions(lang)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	index := slices.IndexFunc(missing, func(m project.MissingVersion) bool { return m.Component == component })
	if index < 0 {
		return 0
	}
	m := missing[index]
	fmt.Fprintf(os.Stderr, "hecatron: %s: %s %s not installed (from %s); hct install %s installs it\n",
		m.Component, lang, m.Spec, m.Source, lang)
	return execNotInstalled
}

// findProgram finds name on environ's PATH as a shell would: 127 when
// it isn't there, 126 when it can't be run.
func findProgram(name string, environ []string) (string, int) {
	for _, entry := range environ {
		if path, ok := strings.CutPrefix(entry, "PATH="); ok {
			_ = os.Setenv("PATH", path) // exec.LookPath reads this process's PATH; CMD replaces it next
		}
	}
	program, err := exec.LookPath(name)
	if errors.Is(err, exec.ErrDot) {
		err = nil // a "." on PATH is the user's choice, as in a shell
	}
	switch {
	case err == nil:
		return program, 0
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist):
		fmt.Fprintf(os.Stderr, "hecatron: exec: %s: command not found\n", name)
		return "", 127
	default:
		fmt.Fprintf(os.Stderr, "hecatron: exec: %s: %v\n", name, err)
		return "", 126
	}
}
