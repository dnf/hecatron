package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/licenses"
	"github.com/dnf/hecatron/internal/reports"
	"github.com/dnf/hecatron/internal/runner"
)

const licensesUsage = `Usage: hecatron licenses [lang...] [-c COMPONENT] [--output human|agent|json] [--json]

Every dependency's license, in every component of each language (those
named, or all the project's), from the lock files: OSV-Scanner reports what
they resolve to and deps.dev's licenses for it (npm, PyPI, Go, crates.io,
Maven, NuGet, RubyGems); Composer's come from composer.lock. Licenses are
rolled up per SPDX identifier; unknown ones (no metadata, or not an SPDX
expression) are listed apart, and languages hecatron can't read licenses
for are listed as not covered. With "licenses" in .hecatron/config.json
(allow, deny, ignore, includeDev), a package the policy doesn't permit
fails the command (exit 1). Needs OSV-Scanner (hecatron install rust
installs it, as do the other languages whose audit uses it) and the
network. See docs/licenses.md.
`

// licenses is `hecatron licenses` (#132). It exists only in the Go core.
func (c *cli) licenses() int {
	flag := ""
	var names, components []string
	for i := 0; i < len(c.args); i++ {
		switch arg := c.args[i]; {
		case isHelp(arg) || arg == "-h":
			fmt.Print(licensesUsage)
			return 0
		case arg == "--output":
			if i+1 >= len(c.args) {
				fmt.Fprintln(os.Stderr, "hecatron: --output needs a format: human, agent or json")
				return 2
			}
			i++
			flag = c.args[i]
		case strings.HasPrefix(arg, "--output="):
			flag = strings.TrimPrefix(arg, "--output=")
		case arg == "--json":
			flag = "json"
		case arg == "-c":
			if i+1 >= len(c.args) {
				fmt.Fprintln(os.Stderr, "hecatron: -c needs a component")
				return 2
			}
			i++
			components = append(components, c.args[i])
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "hecatron: licenses: unknown option '%s'\n%s", arg, licensesUsage)
			return 2
		default:
			names = append(names, arg)
		}
	}
	format, err := runner.SelectFormat(flag, os.Getenv, runner.IsTerminal(os.Stdout))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 2
	}
	if status := c.findProject(); status != 0 {
		return status
	}
	p := c.project
	if p == nil {
		fmt.Fprintln(os.Stderr, "hecatron: licenses can't read this project's configuration yet; hct list and hct doctor can")
		return 1
	}
	for _, component := range components {
		if !contains(p.Components, component) {
			fmt.Fprintf(os.Stderr, "hecatron: unknown component '%s' (components: %s)\n", component, strings.Join(p.Components, " "))
			return 2
		}
	}
	langs, status := c.licenseLanguages(names)
	if status != 0 {
		return status
	}
	targets, err := licenses.ProjectTargets(p, langs, components)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	report, err := licenses.Gather(p.Root, targets, osvScanner)
	if err != nil {
		if errors.Is(err, errNoOSVScanner) {
			fmt.Fprintf(os.Stderr, "hecatron: licenses: %v; %s, or see https://google.github.io/osv-scanner/installation/\n",
				err, osvScannerInstallHint(langs))
			return 3
		}
		fmt.Fprintln(os.Stderr, "hecatron: licenses:", err)
		return 1
	}
	report.Apply(p.LicensePolicy(), time.Now().Format("2006-01-02"))
	if err := licenses.Write(os.Stdout, report, string(format)); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	appendStepSummary(reports.LicensesSummary(report))
	if report.Policy != nil && len(report.Policy.Violations) > 0 {
		return 1
	}
	return 0
}

// licenseLanguages are the languages named (by name or abbreviation; each
// must be the project's), or every one the project has and hasn't disabled.
func (c *cli) licenseLanguages(names []string) ([]string, int) {
	p := c.project
	if len(names) == 0 {
		var langs []string
		for _, lang := range p.Languages {
			if !contains(p.Disabled, lang) {
				langs = append(langs, lang)
			}
		}
		return langs, 0
	}
	var langs []string
	for _, name := range names {
		lang, err := p.ResolveName(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hecatron:", err)
			return nil, 1
		}
		if lang == "" || !contains(p.Languages, lang) {
			fmt.Fprintf(os.Stderr, "hecatron: licenses: '%s' isn't one of the project's languages (%s)\n", name,
				strings.Join(p.Languages, " "))
			return nil, 2
		}
		if !contains(langs, lang) {
			langs = append(langs, lang)
		}
	}
	return langs, 0
}

var errNoOSVScanner = errors.New("OSV-Scanner isn't installed")

// osvScannerInstallHint names the hecatron install that brings OSV-Scanner:
// every language whose licenses it reads installs it with its toolchain, so
// any of the project's will do.
func osvScannerInstallHint(langs []string) string {
	var readable []string
	for _, lang := range langs {
		if licenses.Covers(lang) {
			readable = append(readable, lang)
		}
	}
	if len(readable) == 0 {
		return "hecatron install of a language it reads installs it"
	}
	hint := "hecatron install " + readable[0] + " installs it with the toolchain"
	if len(readable) > 1 {
		hint += " (as does installing " + strings.Join(readable[1:], " or ") + ")"
	}
	return hint
}

// osvScanner runs OSV-Scanner's license report on a directory: every
// package (--all-packages) with deps.dev's licenses. It exits 1 when it
// finds vulnerabilities too, and 128 when the directory has nothing it
// reads.
func osvScanner(dir string) ([]byte, error) {
	program, err := exec.LookPath("osv-scanner")
	if err != nil {
		return nil, errNoOSVScanner
	}
	cmd := exec.Command(program, "scan", "source", "--recursive", "--all-packages", "--licenses", "--format", "json", dir)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit) && exit.ExitCode() == 1:
	case errors.As(err, &exit) && exit.ExitCode() == 128:
		return nil, nil
	default:
		return nil, fmt.Errorf("osv-scanner failed on %s (%v): %s", dir, err, lastLines(stderr.String(), 10))
	}
	return stdout.Bytes(), nil
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
