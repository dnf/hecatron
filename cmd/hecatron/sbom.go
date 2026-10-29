package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/licenses"
	"github.com/dnf/hecatron/internal/sbom"
)

const sbomUsage = `Usage: hecatron sbom [lang...] [-c COMPONENT] [--format cyclonedx|spdx] [-o FILE] [--no-dev]

A software bill of materials for the project: every component of each
language (those named, or all the project's), the packages its lock files
resolve to (OSV-Scanner's, as hct licenses reads them) with their purls,
SPDX licenses where known, and direct dependencies. CycloneDX 1.6 JSON by
default, or SPDX 2.3 JSON; to stdout, or FILE with -o. --no-dev leaves dev
and test dependencies out. SOURCE_DATE_EPOCH sets the timestamp, for a
reproducible document. Needs OSV-Scanner and the network, as hct licenses
does. See docs/guide/sbom.md.
`

// sbomCommand is `hecatron sbom` (#226). It exists only in the Go core.
func (c *cli) sbomCommand() int {
	format, output, noDev := sbom.Formats[0], "", false
	var names, components []string
	for i := 0; i < len(c.args); i++ {
		arg := c.args[i]
		value := func(what string) (string, bool) {
			if i+1 >= len(c.args) {
				fmt.Fprintf(os.Stderr, "hecatron: %s needs %s\n", arg, what)
				return "", false
			}
			i++
			return c.args[i], true
		}
		var ok bool
		switch {
		case isHelp(arg) || arg == "-h":
			fmt.Print(sbomUsage)
			return 0
		case arg == "--format":
			if format, ok = value("a format: cyclonedx or spdx"); !ok {
				return 2
			}
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "-o":
			if output, ok = value("a file"); !ok {
				return 2
			}
		case arg == "-c":
			component, ok := value("a component")
			if !ok {
				return 2
			}
			components = append(components, component)
		case arg == "--no-dev":
			noDev = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "hecatron: sbom: unknown option '%s'\n%s", arg, sbomUsage)
			return 2
		default:
			names = append(names, arg)
		}
	}
	if !contains(sbom.Formats, format) {
		fmt.Fprintf(os.Stderr, "hecatron: sbom: unknown format '%s' (cyclonedx or spdx)\n", format)
		return 2
	}
	created, err := sourceDateEpoch()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: sbom:", err)
		return 2
	}
	if status := c.findProject(); status != 0 {
		return status
	}
	p := c.project
	if p == nil {
		fmt.Fprintln(os.Stderr, "hecatron: sbom can't read this project's configuration yet; hct list and hct doctor can")
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
	report, err := licenses.GatherAll(p.Root, targets, osvScanner)
	if err != nil {
		if errors.Is(err, errNoOSVScanner) {
			fmt.Fprintf(os.Stderr, "hecatron: sbom: %v; %s, or see https://google.github.io/osv-scanner/installation/\n",
				err, osvScannerInstallHint(langs))
			return 3
		}
		fmt.Fprintln(os.Stderr, "hecatron: sbom:", err)
		return 1
	}
	for _, note := range sbomNotes(report) {
		fmt.Fprintln(os.Stderr, "hecatron: sbom:", note)
	}
	data, err := sbom.Write(sbom.Input{Project: p.Name, Commit: gitCommit(p.Root), ToolVersion: c.toolVersion(),
		Time: created, Components: sbomComponents(p.Components, targets), Packages: report.Packages, NoDev: noDev}, format)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: sbom:", err)
		return 1
	}
	if output == "" {
		_, _ = os.Stdout.Write(data)
		return 0
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: sbom:", err)
		return 1
	}
	return 0
}

// sourceDateEpoch is the document's time: SOURCE_DATE_EPOCH's, for a
// reproducible build (reproducible-builds.org), else now.
func sourceDateEpoch() (time.Time, error) {
	epoch := os.Getenv("SOURCE_DATE_EPOCH")
	if epoch == "" {
		return time.Now(), nil
	}
	seconds, err := strconv.ParseInt(epoch, 10, 64)
	if err != nil || seconds < 0 {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH must be seconds since 1970, not '%s'", epoch)
	}
	return time.Unix(seconds, 0), nil
}

// sbomNotes say what the bill of materials leaves out or can't say.
func sbomNotes(report *licenses.Report) []string {
	var notes []string
	for _, n := range report.NotCovered {
		switch {
		case n.Component != "":
			notes = append(notes, fmt.Sprintf("%s: %s: not listed: %s", n.Component, n.Language, n.Reason))
		case len(licenses.Ecosystems[n.Language]) > 0:
			notes = append(notes, fmt.Sprintf("%s: listed without licenses: %s", n.Language, n.Reason))
		default:
			notes = append(notes, fmt.Sprintf("%s: not listed: %s", n.Language, n.Reason))
		}
	}
	return append(notes, report.Warnings...)
}

// sbomComponents are the components the targets cover, in the project's
// order, each with its languages.
func sbomComponents(order []string, targets []licenses.Target) []sbom.Component {
	languages := map[string][]string{}
	for _, t := range targets {
		if !contains(languages[t.Component], t.Lang) {
			languages[t.Component] = append(languages[t.Component], t.Lang)
		}
	}
	var out []sbom.Component
	for _, name := range order {
		if langs, ok := languages[name]; ok {
			out = append(out, sbom.Component{Name: name, Languages: langs})
		}
	}
	return out
}

// gitCommit is the repository's HEAD at root, or "" outside git.
func gitCommit(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// toolVersion is hecatron's version, from <home>/VERSION.
func (c *cli) toolVersion() string {
	data, err := os.ReadFile(c.homePath("VERSION"))
	if version := strings.TrimSpace(string(data)); err == nil && version != "" {
		return version
	}
	return "unknown"
}
