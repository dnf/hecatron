package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/advisor"
	"github.com/dnf/hecatron/internal/runner"
)

const advisorUsage = `Usage: hecatron advisor [--output human|agent|json] [--fail-on recommended|suggestion] [--fix] [lang...]

Recommend what each language's ecosystem considers good practice for the
project: lock files committed, toolchains pinned and supported, formatter
and linter configured, CI covering each language, and more, each with why
and a link to the ecosystem's own documentation. Reads files only: no
network, nothing installed or run. "recommended" is what the ecosystem asks
for; "suggestion" is good practice a project may have reasons to skip.

  --fix              Make the safe, mechanical changes (a toolchain pin file
                     at the version in use), listing each; never edits code
  --fail-on LEVEL    Exit 1 if there is a finding at LEVEL or above
  --output FORMAT    As for hct test: --output, else HECATRON_OUTPUT, else
                     agent output for an AI agent, else human

Silence a rule in .hecatron/config.json (hct check validates it):
  "advisor": {"ignore": [{"id": "rust/toolchain-pinned", "reason": "..."}]}
An id without a slash ("toolchain-pinned") silences it for every language;
"path" limits an entry to one directory.
`

// advisor is `hecatron advisor` (#105). It exists only in the Go core.
func (c *cli) advisor() int {
	flag, failOn, fix := "", "", false
	var names []string
	usage := func(format string, args ...any) int {
		fmt.Fprintf(os.Stderr, "hecatron: advisor: "+format+"\n%s", append(args, advisorUsage)...)
		return 2
	}
	for i := 0; i < len(c.args); i++ {
		switch arg := c.args[i]; {
		case arg == "help" || arg == "-h" || arg == "--help":
			fmt.Print(advisorUsage)
			return 0
		case arg == "--fix":
			fix = true
		case (arg == "--output" || arg == "--fail-on") && i+1 >= len(c.args):
			return usage("%s needs a value", arg)
		case arg == "--output":
			i++
			flag = c.args[i]
		case strings.HasPrefix(arg, "--output="):
			flag = strings.TrimPrefix(arg, "--output=")
		case arg == "--fail-on":
			i++
			failOn = c.args[i]
		case strings.HasPrefix(arg, "--fail-on="):
			failOn = strings.TrimPrefix(arg, "--fail-on=")
		case strings.HasPrefix(arg, "-"):
			return usage("unexpected argument '%s'", arg)
		default:
			names = append(names, arg)
		}
	}
	if failOn != "" && failOn != advisor.Recommended && failOn != advisor.Suggestion {
		return usage("--fail-on must be recommended or suggestion (got '%s')", failOn)
	}
	format, err := runner.SelectFormat(flag, os.Getenv, runner.IsTerminal(os.Stdout))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 2
	}
	if status := c.findProject(); status != 0 {
		return status
	}
	if c.project == nil {
		fmt.Fprintln(os.Stderr, "hecatron: advisor can't read this project's configuration yet; hct check says why")
		return 1
	}
	var languages []string
	for _, name := range names {
		lang, err := c.project.ResolveName(name)
		if err != nil || lang == "" || !contains(c.project.Languages, lang) {
			fmt.Fprintf(os.Stderr, "hecatron: advisor: '%s' isn't one of the project's languages (%s)\n", name, strings.Join(c.project.Languages, ", "))
			return 2
		}
		if !contains(languages, lang) {
			languages = append(languages, lang)
		}
	}
	report, err := advisor.Advise(c.project, advisor.Options{Languages: languages, Now: time.Now(), Fix: fix})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	if err := advisor.Write(os.Stdout, report, string(format)); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	failing := report.Count(advisor.Recommended)
	if failOn == advisor.Suggestion {
		failing += report.Count(advisor.Suggestion)
	}
	if failOn != "" && failing > 0 {
		return 1
	}
	return 0
}
