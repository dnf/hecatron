package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dnf/hecatron/internal/platform"
)

// A target is how bin/hecatron runs a command: a program found on PATH
// (bash, python3) with its arguments, and what it adds to the environment.
type target struct {
	argv []string
	dir  string   // the working directory, when not the current one
	env  []string // NAME=value, on top of HECATRON_HOME and HECATRON_ROOT
}

func bashScript(script string, args ...string) func(*cli) target {
	return func(c *cli) target {
		return target{argv: append([]string{"bash", c.homePath(script)}, append(args, c.args...)...)}
	}
}

func runScript(c *cli) target {
	return target{argv: append([]string{"bash", c.homePath("libexec/commands/run.sh"), c.command}, c.args...)}
}

func manage(c *cli) target {
	return target{argv: append([]string{"python3", c.homePath("libexec/commands/manage.py"), c.command}, c.args...)}
}

// delegated are bin/hecatron's commands, each as bin/hecatron runs it.
var delegated = map[string]func(*cli) target{
	"test":        runScript,
	"lint":        runScript,
	"build":       runScript,
	"clean":       runScript,
	"rebuild":     runScript,
	"retest":      runScript,
	"audit":       runScript,
	"format":      runScript,
	"deps":        runScript,
	"outdated":    runScript,
	"doctor":      bashScript("libexec/commands/doctor.sh"),
	"get-started": bashScript("libexec/onboarding/get-started.sh"),
	"metadata":    bashScript("libexec/metadata/set.sh"),
	"version":     bashScript("libexec/commands/version.sh"),
	"bootstrap":   bootstrap,
	"env":         envTarget,
	"parallel":    parallel,
	"init":        manage,
	"install":     manage,
	"uninstall":   manage,
	"list":        manage,
	"add":         manage,
	"remove":      manage,
	"enable":      manage,
	"disable":     manage,
	"dockerfile":  manage,
	"workflows":   manage,
	"check":       manage,
	"banner":      manage,
}

// bootstrap sets up a fresh machine: no arguments, the project's
// environment (shell-env's exports), then the platform's script.
func bootstrap(c *cli) target {
	script := "libexec/bootstrap/linux.sh"
	if runtime.GOOS == "darwin" {
		script = "libexec/bootstrap/macos.sh"
	}
	return c.withShellEnv(target{argv: []string{"bash", c.homePath(script)}})
}

// parallel runs a project's own job list (just recipes unless
// HECATRON_JOB_COMMAND says otherwise) with the runner, from the root.
func parallel(c *cli) target {
	t := target{argv: append([]string{"bash", c.homePath("libexec/runner/run-parallel.sh")}, c.args...), dir: c.root}
	if os.Getenv("HECATRON_JOB_COMMAND") == "" {
		t.env = append(t.env, "HECATRON_JOB_COMMAND=just")
	}
	return c.withShellEnv(t)
}

// envTarget is `hecatron env` as bin/hecatron runs it: with a language,
// project.py's env query (`env java -c age`); without, shell code exporting
// HECATRON_HOME and HECATRON_ROOT and sourcing helpers.sh, quoted by bash.
func envTarget(c *cli) target {
	if len(c.args) > 0 {
		argv := []string{"python3", c.homePath("libexec/lib/project.py"), "env", c.args[0]}
		if len(c.args) > 2 && c.args[1] == "-c" && c.args[2] != "" {
			argv = append(argv, c.args[2])
		}
		return target{argv: argv}
	}
	return target{argv: []string{"bash", "-c", `printf 'export HECATRON_HOME=%q\n' "${HECATRON_HOME}"
printf 'export HECATRON_ROOT=%q\n' "${HECATRON_ROOT}"
printf 'source %q\n' "${HECATRON_HOME}/libexec/lib/helpers.sh"`}}
}

// withShellEnv adds what bin/hecatron evals from project.py's shell-env
// before bootstrap and parallel: its exported variables. From the Go model
// when it reads the project, else from project.py itself, through bash.
func (c *cli) withShellEnv(t target) target {
	if c.project != nil {
		if shell, err := c.project.Shell(); err == nil {
			for _, export := range shell.Exports {
				t.env = append(t.env, export[0]+"="+export[1])
			}
			return t
		}
	}
	const evalShellEnv = `set -euo pipefail
_env="$(python3 "${HECATRON_HOME}/libexec/lib/project.py" shell-env)"
eval "${_env}"
exec "$@"`
	t.argv = append([]string{"bash", "-c", evalShellEnv, "hecatron"}, t.argv...)
	return t
}

func (c *cli) homePath(relative string) string {
	return filepath.Join(c.home.Dir, filepath.FromSlash(relative))
}

// environment is the process's environment with HECATRON_HOME, and
// HECATRON_ROOT once the project is found, as bin/hecatron exports them.
func (c *cli) environment(extra []string) []string {
	env := setEnv(os.Environ(), "HECATRON_HOME", c.home.Dir)
	if c.root != "" {
		env = setEnv(env, "HECATRON_ROOT", c.root)
	}
	for _, entry := range extra {
		name, value, _ := strings.Cut(entry, "=")
		env = setEnv(env, name, value)
	}
	return env
}

func setEnv(env []string, name, value string) []string { return platform.Setenv(env, name, value) }

// delegate runs a target in place of this process, after the Python check
// bin/hecatron makes before every command but help and --version.
func (c *cli) delegate(target func(*cli) target) int {
	if status := c.requirePython(); status != 0 {
		return status
	}
	t := target(c)
	program, err := exec.LookPath(t.argv[0])
	if t.argv[0] == "bash" {
		program = bashProgram()
		program, err = exec.LookPath(program)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hecatron: %s not found on PATH (needed for '%s' until it is ported to Go)\n", t.argv[0], c.command)
		return 127
	}
	if t.dir != "" {
		if err := os.Chdir(t.dir); err != nil {
			fmt.Fprintln(os.Stderr, "hecatron:", err)
			return 1
		}
	}
	return replaceProcess(program, t.argv, c.environment(t.env))
}

// requirePython says how to get Python 3.9 when it's missing, as
// bin/hecatron does: every command it runs needs it.
func (c *cli) requirePython() int {
	if c.pythonChecked {
		return 0
	}
	python, err := exec.LookPath("python3")
	if err == nil && exec.Command(python, "-c", "import sys; sys.exit(sys.version_info < (3, 9))").Run() == nil {
		c.pythonChecked = true
		return 0
	}
	if err == nil {
		version, _ := exec.Command(python, "-V").CombinedOutput()
		fmt.Fprintf(os.Stderr, "hecatron: needs Python 3.9 or later; python3 is %s.\n",
			strings.TrimRight(strings.TrimPrefix(string(version), "Python "), "\n"))
	} else {
		fmt.Fprintln(os.Stderr, "hecatron: needs Python 3.9 or later, and there is no python3 on PATH.")
	}
	fmt.Fprintln(os.Stderr, "  macOS:          xcode-select --install   (or: brew install python)")
	fmt.Fprintln(os.Stderr, "  Debian, Ubuntu: sudo apt-get install python3")
	fmt.Fprintln(os.Stderr, "  Fedora:         sudo dnf install python3")
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "  Windows:        Python from the Microsoft Store, which provides python3")
	}
	return 1
}

// rootFromPython finds the project with project.py, for a project the Go
// model declines: project.py prints the root, or (with report) why there
// is none.
func (c *cli) rootFromPython(report bool) bool {
	if c.requirePython() != 0 {
		return false
	}
	cmd := exec.Command("python3", c.homePath("libexec/lib/project.py"), "root")
	cmd.Env = c.environment(nil)
	if report {
		cmd.Stderr = os.Stderr
	}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	c.root = strings.TrimRight(string(out), "\n")
	return true
}
