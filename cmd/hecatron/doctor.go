package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

// doctor is `hecatron doctor [--fix] [lang ...]`, doctor.sh's driver in Go:
// for each language (by default every added, enabled one), its problems,
// one per line: its command or lint tool not on PATH (with its version's
// environment), components whose version isn't installed, and what its
// doctor.sh prints (the project's, else the built-in one; it stays a
// script). With --fix an unhealthy language's install.sh runs and the
// language is checked again. Then the version notes, and a summary.
//
// A language name or abbreviation doctor.sh's _check would question (an
// unknown or unadded one, which may prompt "Did you mean") is left to
// doctor.sh, as is a project whose versions the Go model declines.
func (c *cli) doctor() (int, bool) {
	if len(c.args) > 0 && isHelp(c.args[0]) {
		return c.doctorHelp()
	}
	shell, err := c.project.Shell()
	if err != nil {
		return 0, false
	}
	fix := false
	var langs []string
	for _, arg := range c.args {
		if arg == "--fix" {
			fix = true
			continue
		}
		lang, ok := addedLanguage(shell, arg)
		if !ok {
			return 0, false
		}
		langs = append(langs, lang)
	}
	if len(langs) == 0 {
		for _, lang := range shell.Added {
			if !contains(shell.Disabled, lang) {
				langs = append(langs, lang)
			}
		}
	}
	if len(langs) == 0 {
		fmt.Println("No languages to check (hecatron add <lang> adds one).")
		return 0, true
	}
	var exports []string
	for _, export := range shell.Exports {
		exports = append(exports, export[0]+"="+export[1])
	}
	d := &doctorRun{cli: c, env: c.environment(exports)}
	d.project().Environ = d.env
	// Everything the Go model answers is worked out once before any script
	// runs, so a project it declines goes to doctor.sh untouched.
	for _, lang := range langs {
		if _, err := c.project.LanguageEnv(lang, ""); project.IsUnsupported(err) {
			return 0, false
		}
		if _, err := c.project.MissingVersions(lang); project.IsUnsupported(err) {
			return 0, false
		}
		if _, err := c.project.VersionNotes(lang); project.IsUnsupported(err) {
			return 0, false
		}
	}
	return d.run(langs, fix), true
}

// addedLanguage is doctor.sh's _check for a name it accepts without a
// word: an added language, by name or abbreviation (_name).
func addedLanguage(shell *project.Shell, name string) (string, bool) {
	lang := name
	for full, abbreviation := range shell.Abbreviations {
		if abbreviation == name {
			lang = full
		}
	}
	return lang, contains(shell.Known, lang) && contains(shell.Added, lang)
}

type doctorRun struct {
	*cli
	env []string // doctor.sh's: hecatron's variables and the project's exports
}

func (d *doctorRun) project() *project.Project { return d.cli.project }

func (d *doctorRun) run(langs []string, fix bool) int {
	var unhealthy []string
	for _, lang := range langs {
		found, notes := splitNotes(d.problems(lang))
		if found != "" && fix {
			installer := d.project().Resolve(lang, "install.sh")
			if installer == "" {
				return 1 // doctor.sh's `_installer="$(_resolve ...)"` under set -e
			}
			fmt.Printf("%s: repairing with %s\n", lang, installer)
			d.runInstaller(installer)
			d.project().ForgetInstalled()
			found, notes = splitNotes(d.problems(lang))
		}
		if found == "" {
			fmt.Printf("  ok    %s\n", lang)
		} else {
			fmt.Printf("  FAIL  %s\n", lang)
			for _, line := range strings.Split(found, "\n") {
				fmt.Printf("          %s\n", line)
			}
			unhealthy = append(unhealthy, lang)
		}
		if notes != "" {
			for _, line := range strings.Split(notes, "\n") {
				fmt.Printf("          %s\n", line)
			}
		}
		versionNotes, err := d.project().VersionNotes(lang)
		if err != nil {
			// doctor.sh's version-notes pipeline fails under set -e.
			if e, ok := err.(*project.Error); ok {
				fmt.Fprintln(os.Stderr, "hecatron:", e.Message)
			} else {
				fmt.Fprintln(os.Stderr, "hecatron:", err)
			}
			return 1
		}
		for _, note := range versionNotes {
			fmt.Printf("          %s: %s %s\n", note.Component, lang, note.Note)
		}
	}
	fmt.Println()
	if len(unhealthy) == 0 {
		fmt.Printf("All %d toolchains look healthy.\n", len(langs))
		return 0
	}
	fmt.Printf("%d of %d need attention: %s\n", len(unhealthy), len(langs), strings.Join(unhealthy, " "))
	if !fix {
		fmt.Printf("Repair with: hecatron doctor --fix %s   (runs each language's installer)\n", strings.Join(unhealthy, " "))
	}
	return 1
}

// problems are a language's problems, one per line, as doctor.sh's
// _problems prints them into a command substitution: trailing newlines
// and NUL bytes dropped. What the scripts write to stderr goes to stderr.
func (d *doctorRun) problems(lang string) string {
	var out bytes.Buffer
	env := d.env
	if languageEnv, err := d.project().LanguageEnv(lang, ""); err == nil {
		env = languageEnv.Apply(d.env)
	} else {
		reportModelError(err) // project.py env's message; the environment stays as it was
	}
	path := getenv(env, "PATH")
	dependency, _ := d.project().Dependency(lang)
	if dependency != "" && !commandFound(dependency, path) {
		fmt.Fprintf(&out, "%s not found\n", dependency)
		return commandSubstitution(out.Bytes())
	}
	if lint, _ := d.project().LintDependency(lang); lint != "" && !commandFound(lint, path) {
		fmt.Fprintf(&out, "lint tool %s not found\n", lint)
	}
	missing, err := d.project().MissingVersions(lang)
	if err != nil {
		reportModelError(err)
	}
	for _, m := range missing {
		fmt.Fprintf(&out, "%s: %s %s not installed (from %s)\n", m.Component, lang, m.Spec, m.Source)
	}
	if script := d.project().Resolve(lang, "doctor.sh"); script != "" {
		cmd := exec.Command("bash", script)
		cmd.Dir, cmd.Env = d.root, env
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, &out, os.Stderr
		_ = cmd.Run() // a doctor.sh exits non-zero when it finds problems; it printed them
	}
	return commandSubstitution(out.Bytes())
}

// runInstaller runs a language's install.sh as doctor.sh does: from the
// working directory, with doctor.sh's environment, its output shown, its
// failure ignored (the language is checked again).
func (d *doctorRun) runInstaller(installer string) {
	cmd := exec.Command("bash", installer)
	cmd.Env = d.env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
}

func reportModelError(err error) {
	var e *project.Error
	if errors.As(err, &e) {
		fmt.Fprintln(os.Stderr, "hecatron:", e.Message)
	}
}

// splitNotes separates what a doctor.sh says that isn't a problem (its lines
// starting "note: ", without the prefix) from its problems, as doctor.sh's
// _notes_of and _problems_of do.
func splitNotes(output string) (problems, notes string) {
	var problemLines, noteLines []string
	for _, line := range strings.Split(output, "\n") {
		if note, ok := strings.CutPrefix(line, "note: "); ok {
			noteLines = append(noteLines, note)
		} else {
			problemLines = append(problemLines, line)
		}
	}
	return strings.TrimRight(strings.Join(problemLines, "\n"), "\n"), strings.TrimRight(strings.Join(noteLines, "\n"), "\n")
}

// commandSubstitution is what bash's $(...) keeps of output.
func commandSubstitution(out []byte) string {
	return strings.TrimRight(strings.ReplaceAll(string(out), "\x00", ""), "\n")
}

// bashBuiltins are what `command -v` finds without a PATH: bash's builtins
// and keywords.
var bashBuiltins = strings.Fields(`. : [ alias bg bind break builtin caller cd command compgen complete compopt
continue declare dirs disown echo enable eval exec exit export false fc fg getopts hash help history jobs kill
let local logout mapfile popd printf pushd pwd read readarray readonly return set shift shopt source suspend test
times trap true type typeset ulimit umask unalias unset wait ! [[ ]] case do done elif else esac fi for function if
in select then time until while { } coproc`)

// commandFound is bash's `command -v name` with PATH path.
func commandFound(name, path string) bool {
	if contains(bashBuiltins, name) {
		return true
	}
	_, found := lookPath(name, path)
	return found
}
