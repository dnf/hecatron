package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/dnf/hecatron/internal/project"
)

// install and uninstall are manage.py's cmd_install and cmd_uninstall in
// Go: each language's install.sh or uninstall.sh (they stay scripts) from
// the project root, with hook_env's variables and the language's version
// environment. Installing every language defers fetching dependencies until
// every toolchain is in (HECATRON_DEFER_DEPS, #107).

// installStep is one language's script and what it runs with.
type installStep struct {
	lang, name, script string // script: its path, "" when there is none
}

// install is `hecatron install [lang...]`: the named languages (added
// ones), stopping at the first that fails, or every added one, carrying on
// past failures and fetching the dependencies the installers deferred.
func (c *cli) install() (int, bool) {
	args, ok := parseManageArgs(c.args, nil, nil, -1)
	if !ok {
		return 0, false
	}
	if status := c.requirePython(); status != 0 {
		return status, true
	}
	installAll := len(args.positionals) == 0
	langs := c.project.Languages
	if !installAll {
		resolved, failure, ok := c.resolveAll(args.positionals, true)
		if !ok {
			return 0, false
		}
		if failure != nil {
			return failure.report(), true
		}
		langs = resolved
	}
	steps, ok := c.installSteps(langs, "install.sh")
	if !ok {
		return 0, false
	}
	deferred, err := os.CreateTemp("", "hecatron-deps-*.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1, true
	}
	defer func() { _ = os.Remove(deferred.Name()) }()
	_ = deferred.Close()
	var extra []string
	if installAll {
		extra = []string{"HECATRON_DEFER_DEPS=" + deferred.Name()}
	}
	var failed []string
	for i, step := range steps {
		if c.runDefinitionScript(step, i > 0, extra) != 0 {
			if !installAll {
				return 1, true
			}
			failed = append(failed, step.lang)
		}
	}
	listed, _ := os.ReadFile(deferred.Name())
	var depsFailed []string
	for _, lang := range unique(project.PyFields(string(listed))) {
		if !contains(failed, lang) && c.fetchDeps(lang) != 0 {
			depsFailed = append(depsFailed, lang)
		}
	}
	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "\nFailed to install: %s\n", strings.Join(failed, " "))
	}
	if len(depsFailed) > 0 {
		lead := "\n"
		if len(failed) > 0 {
			lead = ""
		}
		fmt.Fprintf(os.Stderr, "%sFailed to fetch dependencies: %s (retry with: hct deps %[2]s)\n", lead, strings.Join(depsFailed, " "))
	}
	if len(failed) > 0 || len(depsFailed) > 0 {
		return 1, true
	}
	return 0, true
}

// uninstall is `hecatron uninstall LANG...`: each named (added) language's
// uninstall.sh, stopping with the status of the first that fails.
func (c *cli) uninstall() (int, bool) {
	args, ok := parseManageArgs(c.args, nil, nil, -1)
	if !ok {
		return 0, false
	}
	if status := c.requirePython(); status != 0 {
		return status, true
	}
	if len(args.positionals) == 0 {
		return (&manageFailure{"usage: hecatron uninstall <lang>...", 2}).report(), true
	}
	langs, failure, ok := c.resolveAll(args.positionals, true)
	if !ok {
		return 0, false
	}
	if failure != nil {
		return failure.report(), true
	}
	steps, ok := c.installSteps(langs, "uninstall.sh")
	if !ok {
		return 0, false
	}
	for i, step := range steps {
		if status := c.runDefinitionScript(step, i > 0, nil); status != 0 {
			return status, true
		}
	}
	return 0, true
}

// installSteps are the languages' scripts. ok is false when the Go model
// declines a language's environment: worked out here, before any script
// runs, so the command can still be handed to manage.py untouched.
func (c *cli) installSteps(langs []string, script string) ([]installStep, bool) {
	c.project.Environ = c.environment(nil)
	steps := make([]installStep, 0, len(langs))
	for _, lang := range langs {
		if _, err := c.project.LanguageEnv(lang, ""); err != nil {
			return nil, false
		}
		steps = append(steps, installStep{lang, script, c.project.Resolve(lang, script)})
	}
	return steps, true
}

// runDefinitionScript is manage.py's run_definition_script: the script
// with bash from the root, its status (a signal's as Python's exit gives
// it). After an earlier installer ran, what installed.sh says is asked
// again, as manage.py asks it only when it gets to the language.
func (c *cli) runDefinitionScript(step installStep, afterAnother bool, extra []string) int {
	if step.script == "" {
		fmt.Fprintf(os.Stderr, "%s: no %s\n", step.lang, step.name)
		return 1
	}
	p := c.project
	env := c.environment(append([]string{
		"HECATRON_ROOT=" + p.Root,
		"HECATRON_PROJECT=" + p.Name,
		"HECATRON_COMPONENTS=" + strings.Join(p.Components, " "),
		"HECATRON_IMPL_PATH=" + p.ImplPath,
	}, extra...))
	if afterAnother {
		p.ForgetInstalled()
	}
	if languageEnv, err := p.LanguageEnv(step.lang, ""); err == nil {
		env = languageEnv.Apply(env)
	} else {
		reportModelError(err)
	}
	cmd := exec.Command("bash", step.script)
	cmd.Dir, cmd.Env = p.Root, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return pythonReturnCode(cmd.Run())
}

// fetchDeps is `hecatron deps LANG` from the root, as manage.py runs it
// through bin/hecatron (which runs this binary where there is one).
func (c *cli) fetchDeps(lang string) int {
	self, err := os.Executable()
	argv := []string{self, "deps", lang}
	if err != nil {
		argv = []string{"bash", c.homePath("bin/hecatron"), "deps", lang}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = c.root, c.environment(nil)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return pythonReturnCode(cmd.Run())
}

// pythonReturnCode is the exit status a subprocess.run returncode gives
// sys.exit: a signal's negative number taken modulo 256.
func pythonReturnCode(err error) int {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return (256 - int(status.Signal())) % 256
		}
		return exit.ExitCode()
	default:
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
}

func unique(items []string) []string {
	var result []string
	for _, item := range items {
		if !contains(result, item) {
			result = append(result, item)
		}
	}
	return result
}
