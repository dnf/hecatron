package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

const execUsage = `Usage: hecatron exec LANG [-c COMPONENT] -- CMD [ARGS...]

Run CMD with LANG's environment for a component: its toolchain version's
PATH and variables, what 'hecatron env LANG -c COMPONENT' prints. CMD runs
in the current directory. Without -c, the component whose LANG directory
holds the current directory, else the only component in LANG. Exits with
CMD's status (127 when it isn't found).

  hct exec zig -c parser -- zig build
  hct exec python -- python -m pytest
`

// execNotInstalled is the exit status when the component's toolchain
// version isn't installed, as for a run where only toolchains were missing.
const execNotInstalled = 3

// execCommand is `hecatron exec` (#225). It exists only in the Go core.
func (c *cli) execCommand() int {
	lang, component, argv, status := parseExecArgs(c.args)
	if status >= 0 {
		return status
	}
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
	return replaceProcess(program, argv, environ)
}

// parseExecArgs reads LANG [-c COMPONENT] -- CMD [ARGS...]; status is -1
// to go on, else the status to exit with.
func parseExecArgs(args []string) (lang, component string, argv []string, status int) {
	usageError := func(message string) (string, string, []string, int) {
		fmt.Fprintf(os.Stderr, "hecatron: exec: %s\n%s", message, execUsage)
		return "", "", nil, 2
	}
	i := 0
	for ; i < len(args) && args[i] != "--"; i++ {
		switch arg := args[i]; {
		case isHelp(arg) || arg == "-h":
			fmt.Print(execUsage)
			return "", "", nil, 0
		case arg == "-c":
			if i+1 >= len(args) {
				return usageError("-c needs a component")
			}
			i++
			component = args[i]
		case strings.HasPrefix(arg, "-"):
			return usageError("unknown option '" + arg + "'")
		case lang == "":
			lang = arg
		default:
			return usageError("put the command after -- (" + arg + ")")
		}
	}
	switch {
	case lang == "":
		return usageError("name a language")
	case i+1 >= len(args):
		return usageError("give the command to run after --")
	}
	return lang, component, args[i+1:], -1
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
