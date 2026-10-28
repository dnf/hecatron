package main

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/deps"
	"github.com/dnf/hecatron/internal/reports"
	"github.com/dnf/hecatron/internal/runner"
)

// A run's reports for CI (docs/runner.md, "Reports"; docs/design.md §20):
// --output junit or sarif prints one in place of the normal output;
// HECATRON_REPORT_DIR writes them beside it (JUnit for test, lint and
// build, SARIF and Code Quality JSON for lint, SARIF for audit); and
// GITHUB_STEP_SUMMARY gets a Markdown summary of every run.

// reportSettings are what a run command writes besides its output.
type reportSettings struct {
	action  string // the command as given (rebuild's steps are its own)
	stdout  string // "junit" or "sarif" (--output), else ""
	dir     string // HECATRON_REPORT_DIR, absolute; "" for none
	summary string // GITHUB_STEP_SUMMARY
	scratch string // the hooks' test reports and audit findings; "" for none

	suitesOf *runner.Report // the report suites were made for
	suites   []reports.Suite
}

// junitActions write JUnit to HECATRON_REPORT_DIR.
var junitActions = map[string]bool{"test": true, "lint": true, "build": true}

// sarifActions write SARIF (lint's error lines, audit's findings).
var sarifActions = map[string]bool{"lint": true, "audit": true}

// reportOutput reads --output junit and sarif, which the run commands take
// besides the formats every command does; "" for those.
func reportOutput(action, output string) (string, error) {
	switch output {
	case "junit":
		return output, nil
	case "sarif":
		if !sarifActions[action] {
			return "", fmt.Errorf("--output sarif works with lint and audit, not %s", action)
		}
		return output, nil
	}
	return "", nil
}

// newReportSettings reads the environment; the scratch directory, made
// when hooks have something to leave in it, is removed by cleanup.
func newReportSettings(action, stdout string) (*reportSettings, func()) {
	s := &reportSettings{action: action, stdout: stdout, summary: os.Getenv("GITHUB_STEP_SUMMARY")}
	if dir := os.Getenv("HECATRON_REPORT_DIR"); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			s.dir = abs
		}
	}
	if s.testReports() || s.auditFindings() {
		if scratch, err := os.MkdirTemp("", "hecatron-reports-"); err == nil {
			s.scratch = scratch
		}
	}
	return s, func() {
		if s.scratch != "" {
			_ = os.RemoveAll(s.scratch)
		}
	}
}

// testReports says whether the test hooks are asked for their tools' own
// reports (HECATRON_TEST_REPORTS): for JUnit, from test or retest.
func (s *reportSettings) testReports() bool {
	return (s.action == "test" || s.action == "retest") && (s.stdout == "junit" || s.dir != "")
}

// auditFindings says whether _audit saves its findings (for SARIF and the
// job summary).
func (s *reportSettings) auditFindings() bool {
	return s.action == "audit" && (s.stdout == "sarif" || s.dir != "" || s.summary != "")
}

// testReportDir is the HECATRON_TEST_REPORTS a component's hooks get,
// made empty; "" for none.
func (r *invocation) testReportDir(lang, component string) string {
	s := r.reports
	if s == nil || s.scratch == "" || r.command != "test" || !s.testReports() {
		return ""
	}
	dir := s.componentScratch(lang, component)
	if os.RemoveAll(dir) != nil || os.MkdirAll(dir, 0o755) != nil {
		return ""
	}
	return dir
}

func (s *reportSettings) componentScratch(lang, component string) string {
	safe := func(name string) string {
		name = url.PathEscape(name)
		if strings.Trim(name, ".") == "" {
			name = strings.ReplaceAll(name, ".", "%2E")
		}
		return name
	}
	return filepath.Join(s.scratch, "tests", safe(lang), safe(component))
}

// auditFindingsEnv is HECATRON_AUDIT_FINDINGS for the hooks, if wanted.
func (r *invocation) auditFindingsEnv() []string {
	s := r.reports
	if s == nil || s.scratch == "" || !s.auditFindings() {
		return nil
	}
	return []string{"HECATRON_AUDIT_FINDINGS=" + s.scratch}
}

// suites are the report's suites, made once.
func (r *invocation) suites(report *runner.Report) []reports.Suite {
	s := r.reports
	if s.suitesOf != report {
		s.suitesOf = report
		opts := reports.SuiteOptions{Patterns: r.errorPatterns}
		if r.command == "test" && s.testReports() {
			opts.BuildReports = true
			opts.TestReports = func(lang, component string) string { return s.componentScratch(lang, component) }
		}
		s.suites = reports.Suites(report, opts)
	}
	return s.suites
}

// workspace is what reports' paths are relative to: the CI checkout
// (GITHUB_WORKSPACE, CI_PROJECT_DIR), else the project root.
func (r *invocation) workspace() string {
	for _, name := range []string{"GITHUB_WORKSPACE", "CI_PROJECT_DIR"} {
		if dir := os.Getenv(name); dir != "" {
			return dir
		}
	}
	return r.root
}

// writeStdoutReport prints --output junit or sarif; false when neither was
// asked for. A step before the last (rebuild's and retest's clean) gives
// agent output on stderr instead, so stdout holds one document.
func (r *invocation) writeStdoutReport(report *runner.Report) bool {
	s := r.reports
	if s == nil || s.stdout == "" {
		return false
	}
	if r.command == "clean" && s.action != "clean" {
		runner.WriteAgent(os.Stderr, report)
		return true
	}
	if err := r.writeReportFormat(os.Stdout, s.stdout, report); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
	}
	return true
}

func (r *invocation) writeReportFormat(w io.Writer, format string, report *runner.Report) error {
	switch format {
	case "junit":
		return reports.WriteJUnit(w, r.command, report.Wall, r.suites(report))
	case "sarif":
		if r.command == "audit" {
			return reports.WriteAuditSARIF(w, r.home.Version(), r.auditFindings())
		}
		return reports.WriteLintSARIF(w, r.home.Version(), reports.LintFindings(report, r.errorPatterns, r.workspace()))
	case "codequality":
		return reports.WriteCodeQuality(w, reports.LintFindings(report, r.errorPatterns, r.workspace()))
	}
	return nil
}

// auditFindings are what the audit hooks saved, placed in the workspace.
func (r *invocation) auditFindings() []reports.AuditFinding {
	if r.reports.scratch == "" {
		return nil
	}
	dirs := map[string]string{}
	for _, lang := range r.shell.Added {
		list, _ := r.project.ImplDirs(lang)
		for _, d := range list {
			dirs[lang+"\x00"+d.Component] = d.Dir
		}
	}
	return reports.PlaceAudit(reports.ReadAuditRecords(r.reports.scratch), r.workspace(), func(f reports.AuditFinding) string {
		if dir, ok := dirs[f.Lang+"\x00"+f.Component]; ok {
			return reports.AuditFile(f.Format, dir)
		}
		return ""
	})
}

// finishReports writes HECATRON_REPORT_DIR's files and the job summary.
// Neither changes the exit status: a failure is a warning.
func (r *invocation) finishReports(report *runner.Report) {
	s := r.reports
	if s == nil || (r.command == "clean" && s.action != "clean") {
		return
	}
	if s.dir != "" {
		var formats [][2]string // format, file suffix
		if junitActions[r.command] {
			formats = append(formats, [2]string{"junit", ".junit.xml"})
		}
		if sarifActions[r.command] {
			formats = append(formats, [2]string{"sarif", ".sarif"})
		}
		if r.command == "lint" {
			formats = append(formats, [2]string{"codequality", ".codequality.json"})
		}
		for _, f := range formats {
			if err := r.writeReportFile(f[0], filepath.Join(s.dir, r.reportName()+f[1]), report); err != nil {
				fmt.Fprintln(os.Stderr, "hecatron: HECATRON_REPORT_DIR:", err)
			}
		}
	}
	if s.summary != "" {
		markdown := reports.RunSummary(report, r.suites(report))
		if r.command == "audit" {
			markdown += reports.AuditSummary(r.auditFindings())
		}
		appendStepSummary(markdown)
	}
}

func (r *invocation) writeReportFile(format, path string, report *runner.Report) error {
	var b bytes.Buffer
	if err := r.writeReportFormat(&b, format, report); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// reportName is a report file's name without its suffix: the action, and
// the targets when named (test-go), so the runs one CI job makes don't
// overwrite each other's.
func (r *invocation) reportName() string {
	parts := []string{r.command}
	for _, job := range r.named {
		parts = append(parts, url.PathEscape(job.Label))
	}
	return strings.Join(parts, "-")
}

// appendStepSummary appends Markdown to the GitHub Actions job summary
// when GITHUB_STEP_SUMMARY is set (hct licenses, hct advisor).
func appendStepSummary(markdown string) {
	if err := reports.AppendSummary(os.Getenv("GITHUB_STEP_SUMMARY"), markdown); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: GITHUB_STEP_SUMMARY:", err)
	}
}

// goTestFilter is `hecatron _gotest FILE`, which the Go test hook pipes go
// test -json through when asked for its report: the events saved to FILE,
// go test's usual output on stdout.
func (c *cli) goTestFilter() int {
	if len(c.args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: hecatron _gotest FILE < go-test-json")
		return 2
	}
	f, err := os.Create(c.args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron _gotest:", err)
		_, _ = io.Copy(os.Stdout, os.Stdin)
		return 1
	}
	err = reports.GoTestFilter(os.Stdin, os.Stdout, f)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron _gotest:", err)
		return 1
	}
	return 0
}

// recordAudit saves a report's findings for the run command's reports
// (HECATRON_AUDIT_FINDINGS), each at the lock file or manifest the auditor
// read: OSV-Scanner names it per package, the others read the one in the
// directory the hook ran them in.
func recordAudit(dir, format, text, lang, component string) {
	classified, err := deps.Classify(format, text, reportConfig(), time.Now().Format("2006-01-02"))
	if err != nil {
		return // the report's error is _audit's own output
	}
	cwd, _ := os.Getwd()
	file := reports.AuditFile(format, cwd)
	var sources map[string]string
	if format == "osv" {
		sources = reports.OSVSources(text)
	}
	findings := []reports.AuditFinding{}
	for _, c := range classified {
		f := c.Finding
		finding := reports.AuditFinding{ID: deps.Text(f.ID), Package: deps.Text(f.Package), Version: deps.Text(f.Version),
			Severity: f.Severity, Title: deps.Text(f.Title), URL: deps.Text(f.URL), Lang: lang, Component: component,
			Format: format, File: file, Accepted: c.Accepted, Reason: c.Reason, Until: c.Until}
		for _, alias := range f.Aliases {
			finding.Aliases = append(finding.Aliases, deps.Text(alias))
		}
		if source, ok := sources[finding.Package]; ok {
			if !filepath.IsAbs(source) {
				source = filepath.Join(cwd, source)
			}
			finding.File = source
		}
		findings = append(findings, finding)
	}
	_ = reports.WriteAuditRecord(dir, findings)
}
