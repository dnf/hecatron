package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// usageText is `hecatron help`, as bin/hecatron prints it.
const usageText = `Usage: hecatron <command> [arguments]     (or: hct <command>)

Set up:
  status                     Languages, toolchain versions, last runs and changes at a glance
  sync [ref]                 After a pull or checkout: fetch changed lock files, check pins and config
  advisor [--fix] [lang...]  Recommend each ecosystem's good practices for the project
  init [dir]                 Set up a repository for hecatron
  doctor [--fix] [lang...]   Check toolchains; --fix runs installers to repair them
  install [lang...]          Install toolchains (all of the project's when none named; --plan: only show how)
  uninstall <lang...>        Uninstall toolchains
  get-started                Status table and install menu
  bootstrap                  Set up a fresh machine for the project (base packages + install)

Run (every component of each language; no languages means all, in parallel):
  test [lang...]             Tests
  lint [lang...|shell]       Linters (shell: shellcheck on every shell script)
  build [lang...]            Builds
  clean [lang...]            Remove build artifacts
  rebuild, retest [lang...]  Clean, then build or test
  audit [lang...]            Check dependencies for known vulnerabilities (--fail-on SEVERITY)
  outdated [lang...]         Report newer versions of dependencies (--json, --fail)
  licenses [lang...]         Every dependency's license, rolled up; fails on the project's policy
  deps [lang...]             Fetch dependencies, held to each component's lock file
  format [--check] [lang...] Format the code with each language's formatter (--check: change nothing)

Languages:
  list [languages] [--all]   Show the project's (or every known) language
  add <lang...>              Add languages to the project
  remove <lang...>           Remove languages from the project
  enable, disable <lang...>  Re-enable or skip languages in parallel runs, for you only

Generated files:
  dockerfile [--check]       Generate the Dockerfile from language snippets
  workflows [--check]        Generate CI: each language's workflow (--ci gitlab: one pipeline)
  check                      Verify every generated file is current
  banner --generator PATH    Print a DO NOT EDIT banner for a code generator

Project:
  parallel <name> "job"...   Run your own jobs (just recipes by default) with the live display
  env                        Print shell code for a project's own recipes: eval "$(hecatron env)"
  env LANG [-c COMPONENT]    Print a language's environment (its version's PATH and variables)
                             gives HECATRON_* variables, the language lists and helpers
                             (_name, _check, _dep_available, _lang_grid, _ada_env, ...)
  version <version>          Bump versions through each language's version.sh
  metadata                   Update package metadata through metadata.sh hooks

Hecatron itself:
  self update [--check]      Update hecatron to the latest release (install.sh's installs)
  self uninstall [--all]     Remove hecatron (and with --all the toolchains it unpacked)
  self test                  Check hecatron's own code: shellcheck and the unit tests
  self build                 Build the Go binary from this checkout, for working on hecatron
  skill install|path         Install hecatron's Agent Skill for AI coding agents

Run 'hecatron <command> help' for a command's options, and 'hecatron help agents'
for a compact reference for AI coding agents.
hecatron %s
`

// selfUsageText is `hecatron self help`, as bin/hecatron prints it.
const selfUsageText = `Usage: hecatron self <command>     (or: hct self <command>)

Manage hecatron itself:
  update [--check]     Update hecatron to the latest release (install.sh's installs)
  uninstall [--all]    Remove hecatron (and with --all the toolchains it unpacked)
  test                 Check hecatron's own code: shellcheck and the unit tests
  build                Build the Go binary from this checkout, for working on hecatron

Run 'hecatron self update help' or 'hecatron self uninstall help' for their options.
`

// self is `hecatron self <command>`: managing hecatron itself, as rustup
// self and uv self do (docs/design.md §8). self-test and self-build, the
// earlier names, still work.
func (c *cli) self() int {
	command := "help"
	if len(c.args) > 0 {
		command, c.args = c.args[0], c.args[1:]
	}
	switch command {
	case "help", "-h", "--help":
		fmt.Print(selfUsageText)
		return 0
	case "update":
		return c.selfUpdate()
	case "uninstall":
		return c.selfUninstall()
	case "test":
		return c.delegate(bashScript("libexec/commands/self-test.sh"))
	case "build":
		return c.delegate(bashScript("libexec/commands/self-build.sh"))
	}
	if status := c.requirePython(); status != 0 {
		return status
	}
	fmt.Fprintf(os.Stderr, "hecatron: self: unknown command '%s' (try 'hecatron self help')\n", command)
	return 2
}

func (c *cli) usage() int {
	version, _ := c.readVersion()
	fmt.Printf(usageText, strings.TrimRight(string(version), "\n"))
	return 0
}

// agentsHelp prints the reference for AI coding agents (docs/agents.md).
func (c *cli) agentsHelp() int {
	text, err := os.ReadFile(c.homePath("docs/agents.md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	_, _ = os.Stdout.Write(text)
	return 0
}

func (c *cli) version() int {
	version, err := c.readVersion()
	if err != nil {
		return 1
	}
	_, _ = os.Stdout.Write(version)
	return 0
}

// readVersion is <home>/VERSION as `cat` gives it, reporting a missing file
// as cat does.
func (c *cli) readVersion() ([]byte, error) {
	path := c.homePath("VERSION")
	data, err := os.ReadFile(path)
	if err != nil {
		reason := err.Error()
		var errno syscall.Errno
		if errors.As(err, &errno) {
			reason = capitalize(errno.Error())
		} else if errors.Is(err, fs.ErrNotExist) {
			reason = "No such file or directory"
		}
		fmt.Fprintf(os.Stderr, "cat: %s: %s\n", path, reason)
	}
	return data, err
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// isHelp is how hecatron's commands ask for their help.
func isHelp(arg string) bool { return arg == "help" || arg == "--help" }

// runActions are what each run command describes itself as, and the hook
// that decides which languages it covers.
var runActions = map[string]struct{ description, hook string }{
	"test":     {"Run each language's tests for every component", "test.sh"},
	"lint":     {"Run each language's linter for every component", "lint.sh"},
	"build":    {"Build each language for every component", "build.sh"},
	"clean":    {"Remove each language's build artifacts for every component (keeps installed dependencies)", "clean.sh"},
	"rebuild":  {"Clean, then build", "build.sh"},
	"retest":   {"Clean, then test", "test.sh"},
	"audit":    {"Check each language's dependencies for known vulnerabilities in every component", "audit.sh"},
	"deps":     {"Fetch each language's dependencies for every component, held to its lock file", "deps.sh"},
	"outdated": {"Report newer versions of each language's dependencies in every component", "outdated.sh"},
	"format":   {"Format each language's code with its formatter in every component", "format.sh"},
}

// runHelpText is `hecatron test help` and the other run commands' help, as
// run.sh prints it; ok is false where the Go model can't say it as run.sh
// would.
func (c *cli) runHelpText() (text string, ok bool) {
	shell, err := c.project.Shell()
	if err != nil {
		return "", false
	}
	tasks, err := c.project.Tasks(c.command)
	if err != nil {
		return "", false
	}
	var taskNames strings.Builder
	for _, task := range tasks {
		if strings.ContainsAny(task.Name, "\n") {
			return "", false
		}
		taskNames.WriteString(task.Name + " ")
	}
	action := runActions[c.command]
	var out strings.Builder
	fmt.Fprintf(&out, "Usage: hecatron %s [flags] [target ...]\n\n", c.command)
	fmt.Fprintf(&out, "%s. With no targets, runs them all in parallel.\n\n", action.description)
	out.WriteString(languageGrid("Languages", shell.HookLanguages(action.hook), shell.Abbreviations))
	if taskNames.Len() > 0 {
		fmt.Fprintf(&out, "\nProject tasks: %s\n", taskNames.String())
	}
	if c.command == "lint" {
		out.WriteString("\nTools:\n  shell      shellcheck on every tracked shell script\n")
	}
	out.WriteString("\nFlags:\n" +
		"  -j VALUE   Concurrency: integer, percentage (default 50%), or 'unlimited'\n" +
		"  -s MODE    Display: scroll (default), full, or zoom\n" +
		"  -c NAME    Only this component (repeatable); components: " + strings.Join(c.project.Components, " ") + "\n")
	if c.command == "audit" {
		out.WriteString("  --fail-on SEVERITY\n" +
			"             Fail on a finding at or above critical, high (default), moderate or low;\n" +
			"             a project sets its own with \"audit\": {\"failOn\": ...} (docs/audit.md)\n")
	}
	if c.command == "outdated" {
		out.WriteString("  --json     Print one JSON array of every outdated dependency (component, language,\n" +
			"             package, current, wanted, latest); the languages run one at a time\n" +
			"  --fail     Fail when anything is outdated (by default this only reports)\n")
	}
	if c.command == "format" {
		out.WriteString("  --check    Change nothing; fail, listing the files the formatter would change\n" +
			"             (hecatron lint runs this check too; docs/format.md)\n")
	}
	// The binary's own flags, listed by run.sh too (saying they need the
	// binary), so the help is the same either way.
	out.WriteString("\nWith hecatron's binary (install.sh downloads it with a release; docs/runner.md):\n" +
		"  --output FORMAT\n" +
		"             human, agent, json, ci, junit or sarif (else HECATRON_OUTPUT;\n" +
		"             agent output for AI agents)\n" +
		"  --timeout DURATION\n" +
		"             Stop a job after DURATION (90s, 30m, 1h30m; 0: no limit; else HECATRON_TIMEOUT);\n" +
		"             by default 5x its usual time from history.log, at least 10 minutes\n")
	if changedActions[c.command] {
		out.WriteString("  --changed [REF]\n" +
			"             Only the languages, components and project tasks the files changed\n" +
			"             since REF touch (default: the pull request's base, else the default branch)\n")
	}
	fmt.Fprintf(&out, "\nExamples:\n"+
		"  hecatron %[1]s               Everything, in parallel\n"+
		"  hecatron %[1]s go rs         Go and Rust (abbreviations work), every component\n"+
		"  hecatron %[1]s go -c age     Go, the age component only\n", c.command)
	return out.String(), true
}

// languageGrid is helpers.sh's _lang_grid: a count, then the languages
// (with their abbreviations) five to a row.
func languageGrid(label string, langs []string, abbreviations map[string]string) string {
	const columns, width = 5, 16
	var out strings.Builder
	fmt.Fprintf(&out, "%s (%d):\n", label, len(langs))
	for i, lang := range langs {
		if i%columns == 0 {
			out.WriteString("  ")
		}
		item := lang
		if abbreviation := abbreviations[lang]; abbreviation != "" {
			item += " (" + abbreviation + ")"
		}
		fmt.Fprintf(&out, "%-*s", width, item)
		if (i+1)%columns == 0 {
			out.WriteString("\n")
		}
	}
	if len(langs)%columns != 0 {
		out.WriteString("\n")
	}
	return out.String()
}

// doctorHelp is `hecatron doctor help`, as doctor.sh prints it once
// helpers.sh has loaded the project.
func (c *cli) doctorHelp() (int, bool) {
	if len(c.args) == 0 || !isHelp(c.args[0]) {
		return 0, false
	}
	if _, err := c.project.Shell(); err != nil {
		return 0, false
	}
	fmt.Print("Usage: hecatron doctor [--fix] [language ...]\n" +
		"\n" +
		"Check that each language's toolchain works: its build and lint tools run, and\n" +
		"known breakages are absent. With no languages, checks every added, enabled one.\n" +
		"\n" +
		"Flags:\n" +
		"  --fix      Run each unhealthy language's installer, then check it again\n")
	return 0, true
}

// versionHelp is version.sh's usage: asked for, or no version given (a
// usage error).
func (c *cli) versionHelp() (int, bool) {
	if len(c.args) > 0 && c.args[0] != "" && !isHelp(c.args[0]) {
		return 0, false
	}
	if _, err := c.project.Shell(); err != nil {
		return 0, false
	}
	fmt.Println("Usage: hecatron version <version>   (runs each language's version.sh)")
	if len(c.args) == 0 || c.args[0] == "" {
		return 2, true
	}
	return 0, true
}
