package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dnf/hecatron/internal/deps"
	"github.com/dnf/hecatron/internal/project"
)

// The hooks' report readers (internal/deps), in place of libexec/lib's
// audit.py and outdated.py, with the same arguments, output and exit
// status. The run commands give the hooks this binary as HECATRON_BINARY
// (run.go), and languages/_audit.sh and _outdated.sh call it when set:
//
//	hecatron _audit report FORMAT FILE LANG COMPONENT
//	hecatron _audit eol LANG VERSION < releases
//	hecatron _outdated report FORMAT FILE LANG COMPONENT [DIRECT]
//	hecatron _outdated collect DIR

const auditReportUsage = `usage: hecatron _audit report FORMAT FILE LANG COMPONENT
       hecatron _audit eol LANG VERSION < releases
`

const outdatedReportUsage = `usage: hecatron _outdated report FORMAT FILE LANG COMPONENT [DIRECT]
       hecatron _outdated collect DIR
`

func (c *cli) auditTool() int {
	args := c.args
	switch {
	case len(args) == 5 && args[0] == "report":
		format, path, lang, component := args[1], args[2], args[3], args[4]
		if !deps.IsAuditFormat(format) {
			fmt.Fprintf(os.Stderr, "hecatron _audit: unknown format %s\n", project.PyStringRepr(format))
			return 2
		}
		text, err := readReport(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hecatron _audit:", err)
			return 1
		}
		return deps.RunAuditReport(os.Stdout, os.Stderr, format, text, lang, component, reportConfig(),
			os.Getenv("HECATRON_AUDIT_FAIL_ON"), time.Now().Format("2006-01-02"))
	case len(args) == 3 && args[0] == "eol":
		releases, _ := io.ReadAll(os.Stdin)
		if warning := deps.EOLWarning(args[1], args[2], project.PyText(releases),
			deps.ReleaseLineParts(c.home.Dir, args[1])); warning != "" {
			fmt.Println(warning)
		}
		return 0
	}
	fmt.Fprint(os.Stderr, auditReportUsage)
	return 2
}

func (c *cli) outdatedTool() int {
	args := c.args
	switch {
	case (len(args) == 5 || len(args) == 6) && args[0] == "report":
		format, path, lang, component := args[1], args[2], args[3], args[4]
		direct := ""
		if len(args) == 6 {
			direct = args[5]
		}
		if !deps.IsOutdatedFormat(format) {
			fmt.Fprintf(os.Stderr, "hecatron _outdated: unknown format %s\n", project.PyStringRepr(format))
			return 2
		}
		text, err := readReport(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hecatron _outdated:", err)
			return 1
		}
		return deps.RunOutdatedReport(os.Stdout, os.Stderr, format, text, lang, component, direct,
			os.Getenv("HECATRON_OUTDATED_JSON_DIR"), os.Getenv("HECATRON_OUTDATED_FAIL") == "1")
	case len(args) == 2 && args[0] == "collect":
		text, err := deps.Collect(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "hecatron _outdated:", err)
			return 1
		}
		fmt.Println(text)
		return 0
	}
	fmt.Fprint(os.Stderr, outdatedReportUsage)
	return 2
}

// readReport is a report file as Python reads it: UTF-8 with invalid bytes
// replaced, universal newlines.
func readReport(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return project.PyText(data), nil
}

// reportConfig is the project's config.json, from HECATRON_ROOT or the
// working directory up.
func reportConfig() any {
	start := os.Getenv("HECATRON_ROOT")
	if start == "" {
		start = "."
	}
	return deps.ProjectConfig(start)
}
