package reports

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dnf/hecatron/internal/runner"
)

// SARIF 2.1.0 (OASIS) for lint's error lines and audit's findings, which
// GitHub code scanning shows on a pull request and GitLab and other SARIF
// readers ingest; GitLab Code Quality JSON for lint's.

const sarifSchema = "https://json.schemastore.org/sarif-2.1.0.json"

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool              sarifTool              `json:"tool"`
	AutomationDetails sarifAutomation        `json:"automationDetails"`
	OriginalURIBaseID map[string]sarifURIRef `json:"originalUriBaseIds,omitempty"`
	Results           []sarifResult          `json:"results"`
}

type sarifURIRef struct {
	URI string `json:"uri"`
}

type sarifAutomation struct {
	ID string `json:"id"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version,omitempty"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string          `json:"id"`
	Name             string          `json:"name,omitempty"`
	ShortDescription sarifMessage    `json:"shortDescription"`
	FullDescription  *sarifMessage   `json:"fullDescription,omitempty"`
	HelpURI          string          `json:"helpUri,omitempty"`
	Default          *sarifRuleLevel `json:"defaultConfiguration,omitempty"`
	Properties       *sarifRuleProps `json:"properties,omitempty"`
}

type sarifRuleLevel struct {
	Level string `json:"level"`
}

type sarifRuleProps struct {
	Tags             []string `json:"tags,omitempty"`
	SecuritySeverity string   `json:"security-severity,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID       string             `json:"ruleId"`
	RuleIndex    int                `json:"ruleIndex"`
	Level        string             `json:"level"`
	Message      sarifMessage       `json:"message"`
	Locations    []sarifLocation    `json:"locations,omitempty"`
	Suppressions []sarifSuppression `json:"suppressions,omitempty"`
	Properties   map[string]string  `json:"properties,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           *sarifRegion  `json:"region,omitempty"`
}

type sarifArtifact struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
}

type sarifSuppression struct {
	Kind          string `json:"kind"`
	Justification string `json:"justification,omitempty"`
}

const (
	informationURI = "https://github.com/dnf/hecatron"
	srcRoot        = "%SRCROOT%"
)

func location(path string, line, column int) []sarifLocation {
	l := sarifLocation{PhysicalLocation: sarifPhysical{ArtifactLocation: sarifArtifact{URI: path, URIBaseID: srcRoot}}}
	if line > 0 {
		l.PhysicalLocation.Region = &sarifRegion{StartLine: line, StartColumn: column}
	}
	return []sarifLocation{l}
}

func writeSARIF(w io.Writer, run sarifRun) error {
	if run.Results == nil {
		run.Results = []sarifResult{}
	}
	if run.Tool.Driver.Rules == nil {
		run.Tool.Driver.Rules = []sarifRule{}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(sarifLog{Schema: sarifSchema, Version: "2.1.0", Runs: []sarifRun{run}})
}

// LintFinding is one error line a linter reported at a file and line.
type LintFinding struct {
	Label   string // the job: a language, a task, shell
	Path    string // relative to the workspace, forward slashes
	Line    int
	Column  int
	Message string
}

// LintFindings are the located error lines in the logs of a lint run's
// failed jobs, every one (not just an excerpt's ten), paths relative to
// the workspace.
func LintFindings(r *runner.Report, patterns func(lang string) *runner.Patterns, workspace string) []LintFinding {
	var findings []LintFinding
	for _, result := range r.Results {
		if !result.Unsuccessful() {
			continue
		}
		log, err := os.ReadFile(result.Log)
		if err != nil {
			continue
		}
		var p *runner.Patterns
		if patterns != nil && result.Lang != "" {
			p = patterns(result.Lang)
		}
		for _, line := range runner.Locate(log, p) {
			message := line.Message
			if message == "" {
				message = line.Text
			}
			findings = append(findings, LintFinding{Label: result.Label, Path: r.WorkspacePath(result, line, workspace),
				Line: line.Line, Column: line.Column, Message: message})
		}
	}
	return findings
}

// WriteLintSARIF writes lint's findings as SARIF: a rule per job
// (lint/go, lint/shell...), each finding an error at its file and line.
func WriteLintSARIF(w io.Writer, version string, findings []LintFinding) error {
	run := sarifRun{Tool: sarifTool{Driver: sarifDriver{Name: "hecatron", Version: version, InformationURI: informationURI}},
		AutomationDetails: sarifAutomation{ID: "hct-lint/"}}
	index := map[string]int{}
	var labels []string
	for _, f := range findings {
		if _, ok := index[f.Label]; !ok {
			index[f.Label] = 0
			labels = append(labels, f.Label)
		}
	}
	sort.Strings(labels)
	for i, label := range labels {
		index[label] = i
		run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarifRule{ID: "lint/" + label,
			ShortDescription: sarifMessage{Text: "An error hct lint " + label + " reported"},
			Default:          &sarifRuleLevel{Level: "error"}})
	}
	for _, f := range findings {
		run.Results = append(run.Results, sarifResult{RuleID: "lint/" + f.Label, RuleIndex: index[f.Label], Level: "error",
			Message: sarifMessage{Text: f.Message}, Locations: location(f.Path, f.Line, f.Column)})
	}
	return writeSARIF(w, run)
}

type codeQualityIssue struct {
	Description string              `json:"description"`
	CheckName   string              `json:"check_name"`
	Fingerprint string              `json:"fingerprint"`
	Severity    string              `json:"severity"`
	Location    codeQualityLocation `json:"location"`
}

type codeQualityLocation struct {
	Path  string           `json:"path"`
	Lines codeQualityLines `json:"lines"`
}

type codeQualityLines struct {
	Begin int `json:"begin"`
}

// WriteCodeQuality writes lint's findings as GitLab Code Quality JSON
// (a CodeClimate subset): each a major issue at its file and line, its
// fingerprint a hash of job, file, line and message.
func WriteCodeQuality(w io.Writer, findings []LintFinding) error {
	issues := []codeQualityIssue{}
	for _, f := range findings {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%s", f.Label, f.Path, f.Line, f.Message)))
		issues = append(issues, codeQualityIssue{Description: f.Message, CheckName: "hct lint " + f.Label,
			Fingerprint: hex.EncodeToString(sum[:16]), Severity: "major",
			Location: codeQualityLocation{Path: f.Path, Lines: codeQualityLines{Begin: f.Line}}})
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(issues)
}

// AuditFinding is one advisory affecting one package in one component,
// as `hecatron _audit report` records it for the reports.
type AuditFinding struct {
	ID        string   `json:"id"`
	Aliases   []string `json:"aliases,omitempty"`
	Package   string   `json:"package"`
	Version   string   `json:"version"`
	Severity  string   `json:"severity"` // critical, high, moderate, low; "" unrated
	Title     string   `json:"title"`
	URL       string   `json:"url,omitempty"`
	Lang      string   `json:"lang"`
	Component string   `json:"component"`
	Format    string   `json:"format"`         // the auditor's report format (npm, osv...)
	File      string   `json:"file,omitempty"` // the lock file or manifest; absolute until placed
	Line      int      `json:"line,omitempty"` // the first line naming the package
	Accepted  bool     `json:"accepted,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Until     string   `json:"until,omitempty"`
	// A possible secret's (Format SecretsFormat, ID its rule): the commit
	// it was found in (scanning the history) and gitleaks' fingerprint.
	// Nothing of the secret itself.
	Commit      string `json:"commit,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// SecretsFormat is the Format of audit's secret scan's findings.
const SecretsFormat = "gitleaks"

// IsSecret says whether a finding is a possible secret.
func (f AuditFinding) IsSecret() bool { return f.Format == SecretsFormat }

// severityRank orders severities, unrated as high (as the threshold does).
func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "", "high":
		return 1
	case "moderate":
		return 2
	}
	return 3
}

// sarifLevel is a severity as a SARIF level.
func sarifLevel(severity string) string {
	switch severityRank(severity) {
	case 0, 1:
		return "error"
	case 2:
		return "warning"
	}
	return "note"
}

// securitySeverity is the score GitHub code scanning ranks security
// alerts by (critical from 9.0, high from 7.0, medium from 4.0).
func securitySeverity(severity string) string {
	return map[int]string{0: "9.5", 1: "8.0", 2: "5.5", 3: "2.0"}[severityRank(severity)]
}

// WriteAuditSARIF writes audit's findings as SARIF: a rule per advisory
// (its ID, title and link, tagged security with a security-severity), a
// result per package affected at its lock file or manifest, an accepted
// risk as a suppressed result with its reason.
func WriteAuditSARIF(w io.Writer, version string, findings []AuditFinding) error {
	run := sarifRun{Tool: sarifTool{Driver: sarifDriver{Name: "hecatron", Version: version, InformationURI: informationURI}},
		AutomationDetails: sarifAutomation{ID: "hct-audit/"}}
	index := map[string]int{}
	for _, f := range sortedFindings(findings) {
		i, ok := index[f.ID]
		if !ok {
			i = len(run.Tool.Driver.Rules)
			index[f.ID] = i
			title := f.Title
			if title == "" {
				title = f.ID
			}
			kind := "dependency"
			if f.IsSecret() {
				kind = "secret"
			}
			rule := sarifRule{ID: f.ID, ShortDescription: sarifMessage{Text: title}, HelpURI: f.URL,
				Default:    &sarifRuleLevel{Level: sarifLevel(f.Severity)},
				Properties: &sarifRuleProps{Tags: []string{"security", kind}, SecuritySeverity: securitySeverity(f.Severity)}}
			if len(f.Aliases) > 0 {
				rule.FullDescription = &sarifMessage{Text: title + " (also " + strings.Join(f.Aliases, ", ") + ")"}
			}
			run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, rule)
		}
		severity := f.Severity
		if severity == "" {
			severity = "unrated"
		}
		result := sarifResult{RuleID: f.ID, RuleIndex: i, Level: sarifLevel(f.Severity),
			Message:    sarifMessage{Text: fmt.Sprintf("%s %s is affected by %s (%s): %s", f.Package, f.Version, f.ID, severity, f.Title)},
			Properties: map[string]string{"package": f.Package, "version": f.Version, "language": f.Lang, "component": f.Component}}
		if f.IsSecret() {
			result.Message.Text = fmt.Sprintf("Possible secret (%s); fingerprint %s", f.ID, f.Fingerprint)
			result.Properties = map[string]string{"fingerprint": f.Fingerprint}
			if f.Commit != "" {
				result.Properties["commit"] = f.Commit
			}
		}
		if f.File != "" {
			result.Locations = location(f.File, f.Line, 0)
		}
		if f.Accepted {
			justification := f.Reason
			if f.Until != "" {
				justification = strings.TrimSpace(justification + " (accepted until " + f.Until + ")")
			}
			result.Suppressions = []sarifSuppression{{Kind: "external", Justification: justification}}
		}
		run.Results = append(run.Results, result)
	}
	return writeSARIF(w, run)
}

// sortedFindings are findings most severe first, then by package, ID,
// language and component.
func sortedFindings(findings []AuditFinding) []AuditFinding {
	sorted := append([]AuditFinding(nil), findings...)
	sort.SliceStable(sorted, func(a, b int) bool {
		x, y := sorted[a], sorted[b]
		if rx, ry := severityRank(x.Severity), severityRank(y.Severity); rx != ry {
			return rx < ry
		}
		for _, pair := range [][2]string{{x.Package, y.Package}, {x.ID, y.ID}, {x.Lang, y.Lang}, {x.Component, y.Component}} {
			if pair[0] != pair[1] {
				return pair[0] < pair[1]
			}
		}
		return false
	})
	return sorted
}

// LineOf is the first line of a file naming a package, else 0.
func LineOf(path, pkg string) int {
	if pkg == "" {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for n := 1; scanner.Scan(); n++ {
		if strings.Contains(scanner.Text(), pkg) {
			return n
		}
	}
	return 0
}

// auditFiles are, per audit format, the files its findings are placed at:
// the lock file the auditor read, else the manifest (globs allowed).
var auditFiles = map[string][]string{
	"npm":          {"package-lock.json", "npm-shrinkwrap.json", "package.json"},
	"pnpm":         {"pnpm-lock.yaml", "package.json"},
	"yarn-classic": {"yarn.lock", "package.json"},
	"yarn-berry":   {"yarn.lock", "package.json"},
	"bun":          {"bun.lock", "bun.lockb", "package.json"},
	"pip-audit":    {"uv.lock", "requirements*.txt", "pyproject.toml", "setup.py"},
	"govulncheck":  {"go.mod"},
	"bundle-audit": {"Gemfile.lock", "Gemfile"},
	"composer":     {"composer.lock", "composer.json"},
	"dotnet":       {"packages.lock.json", "*.sln", "*.slnx", "*.csproj", "*.fsproj", "*.vbproj"},
	"osv":          {"Cargo.lock", "pom.xml", "gradle.lockfile", "build.gradle.kts", "build.gradle", "pubspec.lock", "mix.lock", "Package.resolved", "renv.lock", "cabal.project.freeze", "stack.yaml.lock", "conan.lock"},
}

// AuditFile is the file in dir an audit format's findings are placed at,
// "" when there's none.
func AuditFile(format, dir string) string {
	for _, pattern := range auditFiles[format] {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		sort.Strings(matches)
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && !info.IsDir() {
				return match
			}
		}
	}
	return ""
}
