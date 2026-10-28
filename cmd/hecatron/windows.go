package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/dnf/hecatron/internal/platform"
)

// Native Windows, phase 1 (docs/design.md §19, #78): the commands the Go
// core runs itself work without bash or Python; these still run bash (the
// languages' hooks, doctor scripts and installers) until the definitions
// are declarative (#77). On Windows they use Git for Windows' bash when it
// is installed, and otherwise say what to install.
var bashCommands = map[string]bool{
	"test":        true,
	"lint":        true,
	"build":       true,
	"clean":       true,
	"rebuild":     true,
	"retest":      true,
	"audit":       true,
	"format":      true,
	"deps":        true,
	"outdated":    true,
	"doctor":      true,
	"install":     true,
	"uninstall":   true,
	"bootstrap":   true,
	"parallel":    true,
	"get-started": true,
	"metadata":    true,
	"version":     true,
	"self-test":   true,
	"self-build":  true,
}

// exitToolchainMissing is the exit status for a toolchain that isn't
// installed (docs/design.md §19, Output); bash counts as one.
const exitToolchainMissing = 3

// requireBash says, on Windows without Git for Windows' bash, that the
// command needs it and how to get it, and returns the exit status; 0 when
// bash is there (or off Windows, where bin/hecatron's rules apply).
func (c *cli) requireBash() int {
	if runtime.GOOS != "windows" || !bashCommands[c.command] || (len(c.args) > 0 && isHelp(c.args[0])) {
		return 0
	}
	if c.command == "version" && (len(c.args) == 0 || c.args[0] == "") {
		return 0 // its usage, which the binary prints
	}
	if _, found := platform.Bash(os.Getenv("PATH")); found {
		return 0
	}
	fmt.Fprintf(os.Stderr, "hecatron: hct %s needs bash on Windows until the languages' commands are declarative (#77).\n", c.command)
	fmt.Fprintln(os.Stderr, "  Install Git for Windows (https://gitforwindows.org); hecatron runs the hooks with its bash.")
	fmt.Fprintln(os.Stderr, "  Or run hecatron inside WSL, where it works as on Linux.")
	fmt.Fprintln(os.Stderr, "  These run natively: list, status, check, init, add, remove, enable, disable, env,")
	fmt.Fprintln(os.Stderr, "  advisor, licenses, workflows, dockerfile, skill.")
	return exitToolchainMissing
}

// bashProgram is the bash that runs hooks and scripts: on Windows Git for
// Windows' (never WSL's launcher), else bash, found on PATH as before (so
// $0 and bash's messages still say "bash").
func bashProgram() string {
	if runtime.GOOS != "windows" {
		return "bash"
	}
	if bash, found := platform.Bash(os.Getenv("PATH")); found {
		return bash
	}
	return "bash"
}

// noPythonOnWindows is whether handing a command to the scripts is
// pointless: on Windows without python3, where saying why the Go model
// declined serves better than asking for Python.
func noPythonOnWindows() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	_, err := exec.LookPath("python3")
	return err != nil
}
