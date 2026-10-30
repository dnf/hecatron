package main

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// reportsProject is runProject with hooks that leave what real tools do:
// alpha's test hook a JUnit report in HECATRON_TEST_REPORTS (as pytest
// does) and, in component b, Surefire's under target/; its audit hook an
// npm audit report for _audit.
func reportsProject(t *testing.T) string {
	t.Helper()
	npm, err := os.ReadFile(filepath.Join(repoHome(t), "tests", "audit", "reports", "npm.json"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for name, content := range runProject {
		files[name] = content
	}
	files[".hecatron/languages/alpha/test.sh"] = stubHook + `
if [ -n "${HECATRON_TEST_REPORTS:-}" ]; then
    echo '<testsuite><testcase classname="hook" name="fromHook" time="0.5"/></testsuite>' >"${HECATRON_TEST_REPORTS}/hook.xml"
fi
if [ "$1" = b ]; then
    mkdir -p target/surefire-reports
    echo '<testsuite><testcase classname="calc.CalcTest" name="fromSurefire" time="0.1"><failure message="boom" type="AssertionError">trace</failure></testcase></testsuite>' >target/surefire-reports/TEST-calc.CalcTest.xml
fi
`
	files[".hecatron/languages/alpha/audit.sh"] = stubHook + `"${HECATRON_BINARY}" _audit report npm npm-audit.json "${HECATRON_LANG}" "$1"` + "\n"
	files["a/alpha/npm-audit.json"] = string(npm)
	files["a/alpha/package-lock.json"] = "{\n  \"packages\": {\n    \"node_modules/lodash\": {\"version\": \"4.17.15\"}\n  }\n}\n"
	root := fixture(t, files)
	for name := range files {
		if strings.HasSuffix(name, ".sh") {
			if err := os.Chmod(filepath.Join(root, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

type junitDoc struct {
	Tests    int `xml:"tests,attr"`
	Failures int `xml:"failures,attr"`
	Suites   []struct {
		Name  string `xml:"name,attr"`
		Cases []struct {
			Name string `xml:"name,attr"`
		} `xml:"testcase"`
	} `xml:"testsuite"`
}

func (d junitDoc) cases(suite string) []string {
	var names []string
	for _, s := range d.Suites {
		if s.Name == suite {
			for _, c := range s.Cases {
				names = append(names, c.Name)
			}
		}
	}
	return names
}

func TestReportsBesideTheOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub hooks are bash")
	}
	root := reportsProject(t)
	summary := filepath.Join(t.TempDir(), "summary.md")
	env := []string{"HECATRON_HOME=" + repoHome(t), "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=agent", "CLAUDECODE=",
		"GITHUB_ACTIONS=", "GITHUB_WORKSPACE=", "CI_PROJECT_DIR=", "GITHUB_STEP_SUMMARY=" + summary, "HECATRON_REPORT_DIR=reports"}
	got := runIn(t, root, env, binary, "test")
	if got.status != 1 || !strings.HasPrefix(got.stdout, "hct test: 3 passed, 2 failed") {
		t.Fatalf("the normal output: %+v", got)
	}
	data, err := os.ReadFile(filepath.Join(root, "reports", "test.junit.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc junitDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for suite, want := range map[string]string{"alpha/a": "fromHook", "alpha/b": "fromHook fromSurefire", "beta/a": "a",
		"beta/b": "b", "missing": "missing", "off": "off", "flaky": "flaky"} {
		if names := strings.Join(doc.cases(suite), " "); names != want {
			t.Errorf("suite %s: %q, want %q\n%s", suite, names, want, data)
		}
	}
	// beta in a and flaky fail; Surefire's case in alpha/b fails although
	// the hook passed.
	if doc.Failures != 3 {
		t.Errorf("%d failures:\n%s", doc.Failures, data)
	}
	markdown, err := os.ReadFile(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"### hct test: 3 passed, 2 failed, 3 skipped", "| delta | a | skipped: no tests in a |", "| beta | a | **failed** |", "| alpha | b | passed |",
		"#### FAIL beta (a/beta/): exit 1", "Log: `.logs/test/beta.log`", "beta.src:3: broken", "Rerun: `hct test beta flaky`"} {
		if !strings.Contains(string(markdown), part) {
			t.Errorf("no %q in the summary:\n%s", part, markdown)
		}
	}

	// A second command appends; a named run's files are its own.
	if got := runIn(t, root, env, binary, "lint", "beta"); got.status != 0 {
		t.Fatalf("lint: %+v", got)
	}
	markdown, _ = os.ReadFile(summary)
	if !strings.Contains(string(markdown), "### hct lint: 1 passed") || !strings.Contains(string(markdown), "### hct test:") {
		t.Errorf("not appended:\n%s", markdown)
	}
	for _, name := range []string{"lint-beta.junit.xml", "lint-beta.sarif", "lint-beta.codequality.json"} {
		if _, err := os.Stat(filepath.Join(root, "reports", name)); err != nil {
			t.Error(err)
		}
	}

	got = runIn(t, root, env, binary, "audit", "alpha")
	sarif, err := os.ReadFile(filepath.Join(root, "reports", "audit-alpha.sarif"))
	if err != nil {
		t.Fatalf("%v\n%+v", err, got)
	}
	var log struct {
		Runs []struct {
			Results []struct {
				RuleID    string `json:"ruleId"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(sarif, &log); err != nil || len(log.Runs) != 1 || len(log.Runs[0].Results) == 0 {
		t.Fatalf("%v:\n%s", err, sarif)
	}
	for _, result := range log.Runs[0].Results {
		loc := result.Locations[0].PhysicalLocation
		if !strings.HasPrefix(result.RuleID, "GHSA-") || loc.ArtifactLocation.URI != "a/alpha/package-lock.json" || loc.Region.StartLine != 3 {
			t.Errorf("%+v", result)
		}
	}
	markdown, _ = os.ReadFile(summary)
	if !strings.Contains(string(markdown), "#### Audit findings: ") || !strings.Contains(string(markdown), "| lodash |") {
		t.Errorf("no audit findings in the summary:\n%s", markdown)
	}
}

func TestReportsOnStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub hooks are bash")
	}
	root := reportsProject(t)
	env := []string{"HECATRON_HOME=" + repoHome(t), "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=", "CLAUDECODE=1",
		"GITHUB_ACTIONS=", "GITHUB_STEP_SUMMARY=", "HECATRON_REPORT_DIR="}
	got := runIn(t, root, env, binary, "test", "beta", "--output", "junit")
	var doc junitDoc
	if err := xml.Unmarshal([]byte(got.stdout), &doc); err != nil || got.status != 1 || doc.Failures != 1 || doc.Tests != 2 {
		t.Errorf("%v: %+v", err, got)
	}
	if _, err := os.Stat(filepath.Join(root, "reports")); err == nil {
		t.Error("files written without HECATRON_REPORT_DIR")
	}
	got = runIn(t, root, env, binary, "retest", "alpha", "--output=junit")
	if err := xml.Unmarshal([]byte(got.stdout), &doc); err != nil || got.status != 0 || !strings.HasPrefix(got.stderr, "hct clean: 1 passed") {
		t.Errorf("retest: %v: %+v", err, got)
	}
	got = runIn(t, root, env, binary, "lint", "--output", "sarif")
	var log map[string]any
	if err := json.Unmarshal([]byte(got.stdout), &log); err != nil || log["version"] != "2.1.0" {
		t.Errorf("lint: %v: %+v", err, got)
	}
	got = runIn(t, root, env, binary, "test", "--output", "sarif")
	if got.status != 2 || got.stderr != "hecatron: --output sarif works with lint and audit, not test\n" {
		t.Errorf("test --output sarif: %+v", got)
	}
}

// hct advisor appends its findings to the job summary too.
func TestAdvisorJobSummary(t *testing.T) {
	root := fixture(t, cliProjects["components"])
	summary := filepath.Join(t.TempDir(), "summary.md")
	env := []string{"HECATRON_HOME=" + repoHome(t), "HECATRON_BUILTIN_LANGUAGES=", "PATH=" + t.TempDir(), "GITHUB_STEP_SUMMARY=" + summary}
	if got := runIn(t, root, env, binary, "advisor", "--output", "agent"); got.status != 0 {
		t.Fatalf("%+v", got)
	}
	markdown, err := os.ReadFile(summary)
	if err != nil || !strings.HasPrefix(string(markdown), "### hct advisor: ") || !strings.Contains(string(markdown), "| ci/configured |") {
		t.Errorf("%v:\n%s", err, markdown)
	}
}

// hct audit --output json lists every finding as a record of its own, with
// the installed and fixed versions; a component with nothing to audit is
// skipped, with why, in every format, and never counted as passed.
func TestAuditJSONAndSkipped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub hooks are bash")
	}
	root := reportsProject(t)
	env := []string{"HECATRON_HOME=" + repoHome(t), "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=", "CLAUDECODE=",
		"GITHUB_ACTIONS=", "GITHUB_STEP_SUMMARY=", "HECATRON_REPORT_DIR=", "XDG_DATA_HOME=" + t.TempDir(),
		"GITHUB_WORKSPACE=", "CI_PROJECT_DIR="}
	got := runIn(t, root, env, binary, "audit", "alpha", "gamma", "--output", "json")
	var doc struct {
		Summary struct {
			Passed, Failed, Skipped int
		} `json:"summary"`
		Jobs []struct {
			Label, Status, Reason string
		} `json:"jobs"`
		Vulnerabilities []struct {
			Component, Language, Package, Version, ID, Severity, Tool, File string
			Fixed, Aliases                                                  []string
		} `json:"vulnerabilities"`
		Secrets []any `json:"secrets"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		t.Fatalf("%v: %+v", err, got)
	}
	npm, _ := os.ReadFile(filepath.Join(repoHome(t), "tests", "audit", "reports", "npm.json"))
	if n := strings.Count(string(npm), `"url": "https://github.com/advisories/`); len(doc.Vulnerabilities) != n || n == 0 {
		t.Errorf("%d vulnerabilities, want every one of the report's %d:\n%s", len(doc.Vulnerabilities), n, got.stdout)
	}
	fixed := map[string][]string{}
	for _, v := range doc.Vulnerabilities {
		if v.Component != "a" || v.Language != "alpha" || v.Package != "lodash" || v.Version != "4.17.15" ||
			!strings.HasPrefix(v.ID, "GHSA-") || v.Tool != "npm" || v.File != "a/alpha/package-lock.json" ||
			v.Fixed == nil || v.Severity == "" {
			t.Errorf("%+v", v)
		}
		fixed[v.ID] = v.Fixed
	}
	// The range "<4.17.21" is fixed in 4.17.21; "<=4.17.23" names no fix.
	if got := strings.Join(fixed["GHSA-35jh-r3h4-6jhm"], " "); got != "4.17.21" {
		t.Errorf("GHSA-35jh-r3h4-6jhm fixed in %q", got)
	}
	if got := fixed["GHSA-r5fr-rjxr-66jc"]; len(got) != 0 {
		t.Errorf("GHSA-r5fr-rjxr-66jc fixed in %q", got)
	}
	if doc.Secrets == nil {
		t.Error("no secrets array")
	}
	statuses := map[string]string{}
	for _, job := range doc.Jobs {
		statuses[job.Label] = job.Status + " " + job.Reason
	}
	if statuses["gamma"] != "skipped a: nothing to audit: no gamma.lock; b: nothing to audit: no gamma.lock" {
		t.Errorf("gamma: %q", statuses["gamma"])
	}
	if doc.Summary.Skipped != 1 || doc.Summary.Passed != 0 {
		t.Errorf("summary %+v", doc.Summary)
	}

	// Skipped alone: exit 0, but not passed.
	agent := runIn(t, root, append(env, "HECATRON_OUTPUT=agent"), binary, "audit", "gamma")
	want := "SKIP gamma (a/gamma/, b/gamma/): a: nothing to audit: no gamma.lock; b: nothing to audit: no gamma.lock\n"
	if agent.status != 0 || !strings.HasPrefix(agent.stdout, "hct audit: 0 passed, 1 skipped (") || !strings.HasSuffix(agent.stdout, want) {
		t.Errorf("agent: %+v", agent)
	}
	human := runIn(t, root, append(env, "HECATRON_OUTPUT=human", "NO_COLOR=1"), binary, "audit", "gamma", "beta")
	for _, part := range []string{
		"gamma: audit skipped for 2 component(s): a (nothing to audit: no gamma.lock), b (nothing to audit: no gamma.lock)",
		"beta: audit skipped for 1 component(s): b (nothing to audit: no beta.lock)", "beta: audit passed for 1 component(s)"} {
		if !strings.Contains(human.stdout, part) || human.status != 0 {
			t.Errorf("no %q in human output: %+v", part, human)
		}
	}
	all := runIn(t, root, append(env, "HECATRON_OUTPUT=human", "NO_COLOR=1"), binary, "audit", "--fail-on", "critical")
	if !strings.Contains(all.stdout, " skipped (") || !regexp.MustCompile(`(?m)^  - gamma .* skipped$`).MatchString(all.stdout) {
		t.Errorf("parallel human output: %+v", all)
	}
	junit := runIn(t, root, env, binary, "audit", "gamma", "--output", "junit")
	if !strings.Contains(junit.stdout, `<skipped message="skipped: nothing to audit: no gamma.lock"/>`) {
		t.Errorf("junit: %+v", junit)
	}
	sarif := runIn(t, root, env, binary, "audit", "gamma", "--output", "sarif")
	if !strings.Contains(sarif.stdout, `"text": "gamma/a not scanned: nothing to audit: no gamma.lock"`) ||
		!strings.Contains(sarif.stdout, `"executionSuccessful": true`) {
		t.Errorf("sarif: %+v", sarif)
	}
}
