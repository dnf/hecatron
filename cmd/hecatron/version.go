package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf8"
)

// versionCommand is `hecatron version <version>`, version.sh in Go: each
// added language's version.sh (the project's, else the built-in one; it
// stays a script) with the new version, from the root, then a count. A
// hook that fails stops the command with its status, as set -e does.
func (c *cli) versionCommand() (int, bool) {
	if status, handled := c.versionHelp(); handled {
		return status, true
	}
	shell, err := c.project.Shell()
	if err != nil {
		return 0, false
	}
	type hook struct{ lang, path string }
	var hooks []hook
	for _, lang := range shell.Added {
		path := c.project.Resolve(lang, "version.sh")
		if strings.ContainsRune(path, '\n') || !utf8.ValidString(path) {
			return 0, false // project.py resolve prints it as $(...) reads it back
		}
		if path != "" {
			hooks = append(hooks, hook{lang, path})
		}
	}
	if status := c.requirePython(); status != 0 {
		return status, true
	}
	var exports []string
	for _, export := range shell.Exports {
		exports = append(exports, export[0]+"="+export[1])
	}
	env := c.environment(exports)
	version := c.args[0]
	for _, h := range hooks {
		fmt.Printf("==> %s\n", h.lang)
		cmd := exec.Command("bash", h.path, version)
		cmd.Dir, cmd.Env = c.root, env
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if status := bashStatus(cmd.Run()); status != 0 {
			return status, true
		}
	}
	fmt.Printf("Set version %s in %d language(s).\n", version, len(hooks))
	return 0, true
}

// bashStatus is the $? bash gives a command: its exit status, or 128 plus
// the signal that killed it.
func bashStatus(err error) int {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exit.ExitCode()
	default:
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 127
	}
}
