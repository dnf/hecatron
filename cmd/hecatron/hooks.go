package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
)

// languageExec runs a language's hook for each component, as
// libexec/lib/each-component.sh does: from the component's directory, with
// the component as $1, HECATRON_COMPONENT and HECATRON_LANG set, the
// language's environment for the component's version (project.py env),
// stdin from /dev/null; every component even after one fails. Lint then
// runs the format hook's check, unless the project turned that off or the
// lint hook checked formatting itself (HECATRON_FORMAT_CHECKED). Without a
// hook for the action, it says so as run.sh does.
func (r *invocation) languageExec(action, lang string, wanted []string) func(context.Context, io.Writer) runner.Outcome {
	return func(ctx context.Context, out io.Writer) runner.Outcome {
		hook := r.project.Resolve(lang, action+".sh")
		if hook == "" {
			return noHook(out, action, lang)
		}
		formatHook := ""
		if action == "lint" && contains(r.shell.LintFormat, lang) {
			formatHook = r.project.Resolve(lang, "format.sh")
		}
		dirs, err := r.project.ImplDirs(lang)
		if err != nil {
			fmt.Fprintln(out, "hecatron:", err)
			return runner.Outcome{ExitCode: 1}
		}
		dependency, _ := r.project.Dependency(lang)
		base := r.environment()
		var outcome runner.Outcome
		var failed []string
		for _, dir := range dirs {
			if len(wanted) > 0 && !contains(wanted, dir.Component) {
				continue
			}
			if ctx.Err() != nil {
				break // stopped: its time limit, or Ctrl-C
			}
			component := runner.Component{Name: dir.Component, Dir: dir.Dir}
			fmt.Fprintf(out, "=== %s %s (%s) ===\n", action, dir.Component, lang)
			env, ok := r.languageEnv(ctx, out, base, lang, dir.Component)
			if ok {
				if dependency != "" {
					_, found := lookPath(dependency, getenv(env, "PATH"))
					component.ToolchainMissing = !found
				}
				checked, err := os.CreateTemp("", "hecatron-format-checked")
				if err != nil {
					fmt.Fprintln(out, "hecatron:", err)
					ok = false
				} else {
					checked.Close()
					hookEnv := append(env, "HECATRON_COMPONENT="+dir.Component, "HECATRON_LANG="+lang)
					ok = runHook(ctx, out, dir.Dir, append(hookEnv, "HECATRON_FORMAT_CHECKED="+checked.Name()), hook, dir.Component)
					if info, err := os.Stat(checked.Name()); formatHook != "" && err == nil && info.Size() == 0 {
						ok = runHook(ctx, out, dir.Dir, append(hookEnv, "HECATRON_FORMAT_CHECK=1"), formatHook, dir.Component) && ok
					}
					os.Remove(checked.Name())
				}
			}
			if !ok {
				component.Failed = true
				failed = append(failed, dir.Component)
			}
			outcome.Components = append(outcome.Components, component)
		}
		switch {
		case len(outcome.Components) == 0:
			fmt.Fprintf(out, "error: no %s implementation found (implPath, or implPaths.%s, in .hecatron/config.json)\n", lang, lang)
			outcome.ExitCode = 1
		case len(failed) > 0:
			fmt.Fprintf(out, "%s: %s failed: %s\n", lang, action, strings.Join(failed, " "))
			outcome.ExitCode = 1
		default:
			fmt.Fprintf(out, "%s: %s passed for %d component(s)\n", lang, action, len(outcome.Components))
		}
		return outcome
	}
}

// noHook is run.sh's word for a language without a hook for the action.
func noHook(out io.Writer, action, lang string) runner.Outcome {
	messages := map[string]string{
		"lint":     "%s has no separate linter: its build treats warnings as errors (hecatron test %[1]s)",
		"build":    "No build step for: %s",
		"clean":    "No build artifacts for: %s",
		"audit":    "Nothing to audit for: %s (hecatron has no auditor for its dependencies)",
		"format":   "No formatter for %s (skipping)",
		"deps":     "No dependency manager for %s (skipping)",
		"outdated": "No outdated report for %s (skipping)",
	}
	if message, ok := messages[action]; ok {
		fmt.Fprintf(out, message+"\n", lang)
		return runner.Outcome{}
	}
	fmt.Fprintf(out, "error: %s has no %s hook (.hecatron/languages/%[1]s/%[2]s.sh)\n", lang, action)
	return runner.Outcome{ExitCode: 1}
}

// languageEnv is the environment for a language's hooks in a component:
// base with the language's environment for the component's version
// applied, from the Go model, else (where it declines) from project.py env.
// Its errors go to out; ok is false when it fails.
func (r *invocation) languageEnv(ctx context.Context, out io.Writer, base []string, lang, component string) ([]string, bool) {
	env, err := r.project.LanguageEnv(lang, component)
	switch {
	case err == nil:
		return env.Apply(base), true
	case !project.IsUnsupported(err):
		fmt.Fprintln(out, "hecatron:", err)
		return nil, false
	}
	return r.languageEnvFromPython(ctx, out, base, lang, component)
}

func (r *invocation) languageEnvFromPython(ctx context.Context, out io.Writer, base []string, lang, component string) ([]string, bool) {
	cmd := exec.CommandContext(ctx, "python3", r.homePath("libexec/lib/project.py"), "env", lang, component)
	cmd.Dir, cmd.Env, cmd.Stderr = r.root, base, out
	exports, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	env, ok := applyExports(base, string(exports))
	if !ok {
		// Not the exports env_exports writes: let bash evaluate them.
		env = append(append([]string(nil), base...), "HECATRON_EVAL_ENV="+string(exports))
	}
	return env, true
}

// applyExports applies project.py's env_exports output (export NAME=WORD,
// and export PATH=WORD:"${PATH}") to env. ok is false for anything else.
func applyExports(env []string, exports string) ([]string, bool) {
	env = append([]string(nil), env...)
	for _, line := range strings.Split(strings.TrimSuffix(exports, "\n"), "\n") {
		if line == "" {
			continue
		}
		assignment, ok := strings.CutPrefix(line, "export ")
		name, word, found := strings.Cut(assignment, "=")
		if !ok || !found || name == "" || strings.Trim(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_") != "" {
			return nil, false
		}
		value, ok := shellWord(word, getenv(env, "PATH"))
		if !ok {
			return nil, false
		}
		env = setEnv(env, name, value)
	}
	return env, true
}

// shellWord is the value of a word shlex.quote writes ('...' and bare
// safe characters), and "${PATH}".
func shellWord(word, path string) (string, bool) {
	var value strings.Builder
	for word != "" {
		switch {
		case strings.HasPrefix(word, "'"):
			end := strings.IndexByte(word[1:], '\'')
			if end < 0 {
				return "", false
			}
			value.WriteString(word[1 : end+1])
			word = word[end+2:]
		case strings.HasPrefix(word, `"${PATH}"`):
			value.WriteString(path)
			word = word[len(`"${PATH}"`):]
		case strings.HasPrefix(word, `"'"`):
			value.WriteByte('\'')
			word = word[3:]
		default:
			c := word[0]
			if strings.IndexByte("\"\\$`!*?[]{}();&|<> \t~#", c) >= 0 {
				return "", false
			}
			value.WriteByte(c)
			word = word[1:]
		}
	}
	return value.String(), true
}

func getenv(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(env[i], name+"="); ok {
			return value
		}
	}
	return ""
}

// runHook runs a hook with bash, as each-component.sh does; ok is its
// success.
func runHook(ctx context.Context, out io.Writer, dir string, env []string, hook, component string) bool {
	argv := []string{hook, component}
	if exports := getenv(env, "HECATRON_EVAL_ENV"); exports != "" {
		argv = append([]string{"-c", `eval "${HECATRON_EVAL_ENV}" && unset HECATRON_EVAL_ENV && exec bash "$@"`, "bash"}, argv...)
	}
	cmd := exec.CommandContext(ctx, "bash", argv...)
	cmd.Dir, cmd.Env = dir, env
	return runCommand(cmd, out) == 0
}

// runCommand runs a command with stdin from /dev/null and its output in
// out, and returns its exit status (1 when it couldn't start or was
// stopped by a signal).
func runCommand(cmd *exec.Cmd, out io.Writer) int {
	cmd.Stdout, cmd.Stderr = out, out
	stopped := runner.Supervise(cmd)
	err := cmd.Run()
	stopped()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit) && exit.ExitCode() > 0:
		return exit.ExitCode()
	case !errors.As(err, &exit):
		fmt.Fprintln(out, "hecatron:", err)
	}
	return 1
}

// taskExec runs a project task's command with bash from the root, as
// run.sh does.
func (r *invocation) taskExec(command string) func(context.Context, io.Writer) runner.Outcome {
	return func(ctx context.Context, out io.Writer) runner.Outcome {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		cmd.Dir, cmd.Env = r.root, r.environment()
		return runner.Outcome{ExitCode: runCommand(cmd, out)}
	}
}

// shellcheckExec is lint's shell tool: libexec/lint/shellcheck.sh.
func (r *invocation) shellcheckExec() func(context.Context, io.Writer) runner.Outcome {
	return func(ctx context.Context, out io.Writer) runner.Outcome {
		cmd := exec.CommandContext(ctx, "bash", r.homePath("libexec/lint/shellcheck.sh"))
		cmd.Dir, cmd.Env = r.root, r.environment()
		return runner.Outcome{ExitCode: runCommand(cmd, out)}
	}
}

// dependencyAvailable is helpers.sh's _dep_available: the language's (or
// tool's) command is on PATH, counting the language's own "path"
// directories; true when it names none.
func (r *invocation) dependencyAvailable(lang string) bool {
	dependency, err := r.project.Dependency(lang)
	if err != nil || dependency == "" {
		return err == nil
	}
	dirs, _ := r.project.StaticPath(lang)
	path := os.Getenv("PATH")
	for _, dir := range dirs {
		path = dir + string(os.PathListSeparator) + path
	}
	_, found := lookPath(dependency, path)
	return found
}

// lookPath finds an executable on a PATH value, as `command -v` does.
func lookPath(name, path string) (string, bool) {
	if strings.ContainsRune(name, '/') {
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
