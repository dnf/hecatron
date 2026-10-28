package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

// env is `hecatron env`. On Windows it prints PowerShell assignments
// instead of POSIX shell. With a language, that language's environment for
// a component (`env java -c age`), as project.py's env query prints it.
// Without, shell code a project's own recipes eval to get HECATRON_HOME,
// HECATRON_ROOT and the helpers: paths bash's printf %q would leave alone
// are printed here; any other is quoted by bash itself (envTarget), since
// %q's escaping is bash's own.
func (c *cli) env() (int, bool) {
	if len(c.args) > 0 {
		return c.languageEnv()
	}
	if runtime.GOOS == "windows" {
		// No bash to source the helpers into: the two variables, for
		// PowerShell (`hct env | Invoke-Expression`).
		fmt.Printf("$env:HECATRON_HOME = %s\n$env:HECATRON_ROOT = %s\n",
			project.PowerShellQuote(c.home.Dir), project.PowerShellQuote(c.root))
		return 0, true
	}
	helpers := c.homePath("libexec/lib/helpers.sh")
	if !quoteFree(c.home.Dir) || !quoteFree(c.root) || !quoteFree(helpers) {
		return 0, false
	}
	fmt.Printf("export HECATRON_HOME=%s\nexport HECATRON_ROOT=%s\nsource %s\n", c.home.Dir, c.root, helpers)
	return 0, true
}

// languageEnv is `hecatron env LANG [-c COMPONENT]`. Like bin/hecatron, it
// takes the language as written and a component only after -c; the rest of
// the arguments are ignored.
func (c *cli) languageEnv() (int, bool) {
	lang, component := c.args[0], ""
	if len(c.args) > 2 && c.args[1] == "-c" {
		component = c.args[2]
	}
	c.project.Environ = c.environment(nil)
	env, err := c.project.LanguageEnv(lang, component)
	if project.IsUnsupported(err) {
		return 0, false
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1, true
	}
	if runtime.GOOS == "windows" {
		fmt.Print(env.PowerShell())
		return 0, true
	}
	fmt.Print(env.Exports())
	return 0, true
}

// quoteFree is text printf %q prints unchanged.
func quoteFree(s string) bool {
	return s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./-") == "" &&
		!strings.HasPrefix(s, "-")
}
