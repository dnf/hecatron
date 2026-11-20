package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/runner"
	"github.com/dnf/hecatron/internal/status"
)

const goUsage = `Usage: hecatron go [lang...] [--output human|agent|json]

Getting a project's tests running, in one command: what each language
resolves to, then hct install (its output in <logs>/go/install.log, one
line here), then hct test. An agent's first command in a repository, in
place of hct status, hct install and hct test one at a time. It installs
toolchains, so ask the user first, as for hct install. With no
languages, every one of the project's. The output format is chosen as
for hct test; with json, the lines before the test run go to stderr.
`

// goCommand is `hecatron go`. It exists only in the Go core.
func (c *cli) goCommand() int {
	var langs []string
	flag := ""
	for i := 0; i < len(c.args); i++ {
		switch arg := c.args[i]; {
		case arg == "help" || arg == "-h" || arg == "--help":
			fmt.Print(goUsage)
			return 0
		case arg == "--output" && i+1 < len(c.args):
			i++
			flag = c.args[i]
		case strings.HasPrefix(arg, "--output="):
			flag = strings.TrimPrefix(arg, "--output=")
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "hecatron: go: unexpected option '%s'\n%s", arg, goUsage)
			return 2
		default:
			langs = append(langs, arg)
		}
	}
	format, err := runner.SelectFormat(flag, os.Getenv, runner.IsTerminal(os.Stdout))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 2
	}
	if code := c.findProject(); code != 0 {
		return code
	}
	if c.project == nil {
		fmt.Fprintln(os.Stderr, "hecatron: go can't read this project's configuration yet; hct install and hct test can")
		return 1
	}
	s, err := status.Gather(c.project, c.environment(nil), time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	chosen, ok := chooseLanguages(s, langs)
	if !ok {
		return 2
	}
	if len(chosen) == 0 {
		fmt.Fprintln(os.Stderr, "hecatron: go: the project has no languages (hct add <lang>)")
		return 1
	}
	// Agent and human output read as one run; JSON stays one document.
	info := io.Writer(os.Stdout)
	if format == runner.JSON {
		info = os.Stderr
	}
	_, _ = fmt.Fprintln(info, goHeader(chosen))

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	installed := c.goInstall(info, self, langs)
	test := exec.Command(self, append(append([]string{"test"}, langs...), "--output", string(format))...)
	test.Stdin, test.Stdout, test.Stderr = os.Stdin, os.Stdout, os.Stderr
	tested := exitCode(test.Run())
	if tested == 0 && format != runner.JSON {
		_, _ = fmt.Fprintf(info, "Test again: %s\n", strings.Join(append([]string{"hct test"}, langs...), " "))
	}
	if tested != 0 {
		return tested
	}
	if !installed {
		return 1
	}
	return 0
}

// chooseLanguages are the named languages, or every enabled one, as status
// gathered them; false (having said why) for a name the project doesn't
// have.
func chooseLanguages(s *status.Status, names []string) ([]status.Language, bool) {
	if len(names) == 0 {
		var all []status.Language
		for _, l := range s.Languages {
			if !l.Disabled {
				all = append(all, l)
			}
		}
		return all, true
	}
	var chosen []status.Language
	for _, name := range names {
		found := false
		for _, l := range s.Languages {
			if l.Name == name {
				chosen, found = append(chosen, l), true
			}
		}
		if !found {
			var known []string
			for _, l := range s.Languages {
				known = append(known, l.Name)
			}
			fmt.Fprintf(os.Stderr, "hecatron: go: '%s' isn't one of the project's languages (%s)\n", name, strings.Join(known, " "))
			return nil, false
		}
	}
	return chosen, true
}

// goHeader is the languages and the versions they resolve to, in one line:
// "hct go: go 1.24 (missing), python 3.14 (missing), perl (installed)".
func goHeader(langs []status.Language) string {
	var parts []string
	for _, l := range langs {
		var specs []string
		for _, v := range l.Versions {
			if v.Spec != "" {
				specs = append(specs, v.Spec)
			}
		}
		part := l.Name
		if len(specs) > 0 {
			part += " " + strings.Join(specs, "/")
		}
		if l.Toolchain != "none" {
			part += " (" + l.Toolchain + ")"
		}
		parts = append(parts, part)
	}
	return "hct go: " + strings.Join(parts, ", ")
}

// goInstall runs hct install for langs (every language when none) with
// its output in <logs>/go/install.log, and says in one line how it went,
// with the log's error lines when it failed. Whether it succeeded.
func (c *cli) goInstall(w io.Writer, self string, langs []string) bool {
	path := filepath.Join(c.project.Logs, "go", "install.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return false
	}
	log, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return false
	}
	install := exec.Command(self, append([]string{"install"}, langs...)...)
	install.Stdin, install.Stdout, install.Stderr = os.Stdin, log, log
	start := time.Now()
	code := exitCode(install.Run())
	_ = log.Close()
	shown := path
	if cwd, err := workingDir(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil && len(rel) < len(path) {
			shown = rel
		}
	}
	took := runner.FormatDuration(time.Since(start))
	if code == 0 {
		_, _ = fmt.Fprintf(w, "install: done, %s, log %s\n", took, shown)
		return true
	}
	_, _ = fmt.Fprintf(w, "FAIL install exit %d, %s, log %s\n", code, took, shown)
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range runner.Extract(data, nil).Lines {
			_, _ = fmt.Fprintf(w, "  %s\n", line.Text)
		}
	}
	return false
}

// exitCode is a child's exit status from its Run error: 0, its own, or 1
// when it didn't start or was signalled.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() >= 0 {
		return exit.ExitCode()
	}
	return 1
}
