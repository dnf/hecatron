package reports

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dnf/hecatron/internal/advisor"
	"github.com/dnf/hecatron/internal/licenses"
	"github.com/dnf/hecatron/internal/runner"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./internal/reports -update writes it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs (go test ./internal/reports -update rewrites it):\n%s", name, got)
	}
}

// goPatterns are languages/go/config.json's error patterns.
var goPatterns = &runner.Patterns{List: []runner.Pattern{
	{Regex: regexp.MustCompile(`^\s*(?P<file>[^\s:]+\.go):(?P<line>\d+)(?::(?P<col>\d+))?: (?P<message>.+)$`)},
	{Regex: regexp.MustCompile(`^--- FAIL: (?P<message>.+)$`)},
}}

func patterns(lang string) *runner.Patterns {
	if lang == "go" {
		return goPatterns
	}
	return nil
}

var started = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

// testReport is a run of hct test: go failed in one component of two,
// python's pytest wrote a JUnit report, rust isn't installed, a task
// failed and another passed.
func testReport() *runner.Report {
	return &runner.Report{Action: "test", Root: "/repo", Cwd: "/repo", LogDir: "/repo/.logs/test", Wall: 14 * time.Second,
		Results: []runner.Result{
			{Label: "go", Lang: "go", Status: runner.Failed, ExitCode: 1, Duration: 12 * time.Second, Log: "testdata/logs/test-go.log",
				Components: []runner.Component{
					{Name: "age", Dir: "/repo/age/go", Failed: true, Started: started, Duration: 7 * time.Second},
					{Name: "date", Dir: "/repo/date/go", Started: started.Add(7 * time.Second), Duration: 5 * time.Second}},
				Excerpt: runner.Excerpt{Source: "patterns", Lines: []runner.ErrorLine{
					{Text: "--- FAIL: TestAdd (0.00s)", Component: "age", Message: "TestAdd (0.00s)"},
					{Text: "calc_test.go:7: Add(2, 3) = -1, want 5", Component: "age", File: "calc_test.go", Line: 7, Message: "Add(2, 3) = -1, want 5"}}}},
			{Label: "python", Lang: "python", Status: runner.Failed, ExitCode: 1, Duration: 1500 * time.Millisecond, Log: "testdata/logs/none.log",
				Components: []runner.Component{{Name: ".", Dir: "/repo/python", Failed: true}}},
			{Label: "rust", Lang: "rust", Status: runner.StatusNotInstall},
			{Label: "javascript", Lang: "javascript", Status: runner.Skipped, Duration: 200 * time.Millisecond, Log: "/repo/.logs/test/javascript.log",
				Components: []runner.Component{{Name: ".", Dir: "/repo/js", Skipped: "package.json has no test script"}}},
			{Label: "elm", Lang: "elm", Status: runner.Skipped, Duration: 100 * time.Millisecond, Log: "/repo/.logs/test/elm.log",
				Skipped: "no test runner for elm"},
			{Label: "---", Status: runner.StatusSeparator},
			{Label: "docs", Task: true, Status: runner.Passed, Duration: 300 * time.Millisecond, Log: "/repo/.logs/test/docs.log"},
			{Label: "flaky", Task: true, Status: runner.Failed, ExitCode: 4, Duration: 100 * time.Millisecond, Log: "testdata/logs/test-flaky.log",
				Excerpt: runner.Excerpt{Source: "tail", Lines: []runner.ErrorLine{{Text: "flaky failed: | pipes & <tags> ```fence```"}}}},
		}}
}

func testSuites() []Suite {
	return Suites(testReport(), SuiteOptions{Patterns: patterns, TestReports: func(lang, component string) string {
		if lang == "python" {
			return filepath.Join("..", "..", "tests", "reports", "pytest", "9.2.0")
		}
		return ""
	}})
}

func TestJUnit(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJUnit(&out, "test", 14*time.Second, testSuites()); err != nil {
		t.Fatal(err)
	}
	golden(t, "test.junit.xml", out.String())
	checkJUnitShape(t, out.Bytes())
}

// checkJUnitShape checks a document against the shape of the JUnit XML
// schema CI systems read (Jenkins' junit.xsd, GitLab's parser): the root
// <testsuites>, each <testsuite> named with tests, failures, errors,
// skipped and time that agree with its <testcase>s, each case named, with
// a classname and time, and at most one of failure, error or skipped.
func checkJUnitShape(t *testing.T, data []byte) {
	t.Helper()
	type problem struct {
		Message *string `xml:"message,attr"`
		Type    *string `xml:"type,attr"`
	}
	type testcase struct {
		Name      *string   `xml:"name,attr"`
		Classname *string   `xml:"classname,attr"`
		Time      *float64  `xml:"time,attr"`
		Failures  []problem `xml:"failure"`
		Errors    []problem `xml:"error"`
		Skipped   []problem `xml:"skipped"`
	}
	type testsuite struct {
		Name     *string    `xml:"name,attr"`
		Tests    int        `xml:"tests,attr"`
		Failures int        `xml:"failures,attr"`
		Errors   int        `xml:"errors,attr"`
		Skipped  int        `xml:"skipped,attr"`
		Time     *float64   `xml:"time,attr"`
		Cases    []testcase `xml:"testcase"`
	}
	var doc struct {
		XMLName  xml.Name
		Tests    int         `xml:"tests,attr"`
		Failures int         `xml:"failures,attr"`
		Suites   []testsuite `xml:"testsuite"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.XMLName.Local != "testsuites" || len(doc.Suites) == 0 {
		t.Fatalf("root %s with %d suites", doc.XMLName.Local, len(doc.Suites))
	}
	tests, failures := 0, 0
	for _, s := range doc.Suites {
		if s.Name == nil || *s.Name == "" || s.Time == nil {
			t.Errorf("a suite without a name or time: %+v", s)
		}
		f, e, k := 0, 0, 0
		for _, c := range s.Cases {
			if c.Name == nil || *c.Name == "" || c.Classname == nil || c.Time == nil {
				t.Errorf("suite %s: a case without name, classname or time", *s.Name)
			}
			if len(c.Failures)+len(c.Errors)+len(c.Skipped) > 1 {
				t.Errorf("suite %s, case %s: more than one result", *s.Name, *c.Name)
			}
			for _, p := range append(c.Failures, c.Errors...) {
				if p.Message == nil || p.Type == nil {
					t.Errorf("suite %s, case %s: a failure without message or type", *s.Name, *c.Name)
				}
			}
			f, e, k = f+len(c.Failures), e+len(c.Errors), k+len(c.Skipped)
		}
		if s.Tests != len(s.Cases) || s.Failures != f || s.Errors != e || s.Skipped != k {
			t.Errorf("suite %s: counts %d/%d/%d/%d, cases %d/%d/%d/%d", *s.Name, s.Tests, s.Failures, s.Errors, s.Skipped, len(s.Cases), f, e, k)
		}
		tests, failures = tests+s.Tests, failures+s.Failures
	}
	if doc.Tests != tests || doc.Failures != failures {
		t.Errorf("testsuites counts %d/%d, suites' %d/%d", doc.Tests, doc.Failures, tests, failures)
	}
}

func TestRunSummary(t *testing.T) {
	golden(t, "test.summary.md", RunSummary(testReport(), testSuites()))
}

func lintReport() *runner.Report {
	return &runner.Report{Action: "lint", Root: "/repo", Wall: 3 * time.Second, Results: []runner.Result{
		{Label: "go", Lang: "go", Status: runner.Failed, ExitCode: 1, Duration: 3 * time.Second, Log: "testdata/logs/lint-go.log",
			Components: []runner.Component{{Name: "age", Dir: "/repo/age/go", Failed: true}, {Name: "date", Dir: "/repo/date/go", Failed: true}}},
		{Label: "python", Lang: "python", Status: runner.Passed, Log: "testdata/logs/none.log"},
	}}
}

func TestLintReports(t *testing.T) {
	findings := LintFindings(lintReport(), patterns, "/repo")
	if len(findings) != 3 || findings[2].Path != "date/go/date.go" || findings[2].Line != 4 || findings[2].Column != 7 {
		t.Fatalf("%+v", findings)
	}
	var sarif, quality bytes.Buffer
	if err := WriteLintSARIF(&sarif, "0.5.0", findings); err != nil {
		t.Fatal(err)
	}
	if err := WriteCodeQuality(&quality, findings); err != nil {
		t.Fatal(err)
	}
	golden(t, "lint.sarif", sarif.String())
	golden(t, "lint.codequality.json", quality.String())
	var issues []map[string]any
	if err := json.Unmarshal(quality.Bytes(), &issues); err != nil || len(issues) != 3 {
		t.Fatalf("%v: %d issues", err, len(issues))
	}
	for _, issue := range issues {
		for _, key := range []string{"description", "check_name", "fingerprint", "severity", "location"} {
			if _, ok := issue[key]; !ok {
				t.Errorf("an issue without %s: %v", key, issue)
			}
		}
	}
	var empty bytes.Buffer
	if err := WriteLintSARIF(&empty, "0.5.0", nil); err != nil || !strings.Contains(empty.String(), `"results": []`) {
		t.Errorf("no findings: %v\n%s", err, empty.String())
	}
}

var auditFindings = []AuditFinding{
	{ID: "GHSA-p6mc-m468-83gw", Aliases: []string{"CVE-2020-8203"}, Package: "lodash", Version: "4.17.15", Severity: "high",
		Title: "Prototype Pollution in lodash", URL: "https://github.com/advisories/GHSA-p6mc-m468-83gw", Lang: "typescript",
		Component: "web", Format: "npm", File: "web/package-lock.json", Line: 12},
	{ID: "RUSTSEC-2024-0001", Package: "smallvec", Version: "1.6.0", Severity: "", Title: "Unrated | advisory", Lang: "rust",
		Component: "core", Format: "osv", File: "core/Cargo.lock", Line: 40},
	{ID: "GHSA-29mw-wpgm-hmr9", Package: "lodash", Version: "4.17.15", Severity: "moderate", Title: "ReDoS in lodash",
		Lang: "typescript", Component: "web", Format: "npm", File: "web/package-lock.json", Line: 12, Accepted: true,
		Reason: "not reachable", Until: "2027-01-01"},
	{ID: "GO-2024-0002", Package: "stdlib", Version: "1.22.0", Severity: "critical", Title: "net/http", Lang: "go",
		Component: "api", Format: "govulncheck"},
}

func TestAuditReports(t *testing.T) {
	var sarif bytes.Buffer
	if err := WriteAuditSARIF(&sarif, "0.5.0", auditFindings, nil); err != nil {
		t.Fatal(err)
	}
	golden(t, "audit.sarif", sarif.String())
	golden(t, "audit.summary.md", AuditSummary(auditFindings))
}

// Findings are placed at their lock file, relative to the workspace, on
// the line naming the package; one resolved in a scratch directory moves
// to the component's own manifest.
func TestPlaceAudit(t *testing.T) {
	workspace := t.TempDir()
	web := filepath.Join(workspace, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "package.json"), []byte("{\n  \"dependencies\": {\n    \"lodash\": \"^4\"\n  }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := AuditFile("npm", web); got != filepath.Join(web, "package.json") {
		t.Errorf("AuditFile: %s", got)
	}
	findings := []AuditFinding{
		{Package: "lodash", Lang: "typescript", Component: "web", Format: "npm", File: filepath.Join(workspace, "web", "package.json")},
		{Package: "lodash", Lang: "typescript", Component: "web", Format: "npm", File: filepath.Join(t.TempDir(), "package-lock.json")},
		{Package: "x", Lang: "go", Component: "api", Format: "govulncheck"},
	}
	placed := PlaceAudit(findings, workspace, func(f AuditFinding) string {
		if f.Component == "web" {
			return AuditFile(f.Format, web)
		}
		return ""
	})
	for i, want := range []struct {
		file string
		line int
	}{{"web/package.json", 3}, {"web/package.json", 3}, {"", 0}} {
		if placed[i].File != want.file || placed[i].Line != want.line {
			t.Errorf("%d: %s:%d, want %s:%d", i, placed[i].File, placed[i].Line, want.file, want.line)
		}
	}
	dir := t.TempDir()
	if err := WriteAuditRecord(dir, auditFindings[:2]); err != nil {
		t.Fatal(err)
	}
	if err := WriteAuditRecord(dir, auditFindings[2:]); err != nil {
		t.Fatal(err)
	}
	if got := ReadAuditRecords(dir); len(got) != 4 || got[0].ID != "GO-2024-0002" {
		t.Errorf("records: %+v", got)
	}
	osv, err := os.ReadFile(filepath.Join("..", "..", "tests", "audit", "reports", "osv-rust.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range OSVSources(string(osv)) {
		if source != "/project/rust/Cargo.lock" {
			t.Errorf("%s: %s", name, source)
		}
	}
}

func TestOtherSummaries(t *testing.T) {
	l := &licenses.Report{
		Packages: []licenses.Package{{Ecosystem: "npm", Name: "left-pad", Version: "1.3.0", License: "WTFPL", Language: "typescript", Component: "web"}},
		Licenses: []licenses.Rollup{{License: "WTFPL", Packages: 1, Languages: []string{"typescript"}}},
		Policy: &licenses.Verdict{Violations: []licenses.Violation{{Package: "npm:left-pad", Version: "1.3.0", License: "WTFPL",
			Reason: "not in allow", Language: "typescript", Component: "web"}}},
		NotCovered: []licenses.NotCovered{{Language: "zig", Reason: "no registry"}},
	}
	a := &advisor.Report{Findings: []advisor.Finding{{ID: "go/toolchain-pinned", Level: advisor.Recommended, Dir: "age/go",
		Languages: []string{"go"}, Message: "Pin Go's version", Link: "https://go.dev/doc/toolchain"}}}
	golden(t, "other.summary.md", LicensesSummary(l)+AdvisorSummary(a))
}

// go test -json's events, through GoTestFilter, print what go test without
// -json printed for the same module (testdata/gotest/module: subtests,
// a skip, a panic, a build failure; captured with Go 1.26), but for times
// and addresses.
func TestGoTestFilterMatchesGoTest(t *testing.T) {
	events, err := os.ReadFile(filepath.Join("testdata", "gotest", "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := os.ReadFile(filepath.Join("testdata", "gotest", "plain.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var out, raw bytes.Buffer
	if err := GoTestFilter(bytes.NewReader(events), &out, &raw); err != nil {
		t.Fatal(err)
	}
	varying := regexp.MustCompile(`\d+\.\d+s|0x[0-9a-f?]+`)
	if got, want := varying.ReplaceAllString(out.String(), "N"), varying.ReplaceAllString(string(plain), "N"); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if !bytes.Equal(raw.Bytes(), events) {
		t.Error("the events weren't saved as they came")
	}
	cases := ReadGoTest(bytes.NewReader(events))
	results := map[string]string{}
	for _, c := range cases {
		switch {
		case c.Failure != nil:
			results[c.Name] = "fail"
		case c.Skipped != nil:
			results[c.Name] = "skip: " + *c.Skipped
		default:
			results[c.Name] = "pass"
		}
	}
	want := map[string]string{"TestSubs": "fail", "TestSubs/ok": "pass", "TestSubs/bad": "fail", "TestSubs/nested": "fail",
		"TestSubs/nested/deep": "fail", "TestPass": "pass", "TestSkip": "skip: a_test.go:16: not today", "TestFail": "fail",
		"TestPanics": "fail", "TestD": "pass"}
	if len(results) != len(want) {
		t.Errorf("%d cases: %v", len(results), results)
	}
	for name, result := range want {
		if results[name] != result {
			t.Errorf("%s: %q, want %q", name, results[name], result)
		}
	}
}

// The reports tests/reports/capture.sh saved from real tools read as their
// cases: each fixture's adding test fails (and, but for Kotlin's, which
// has one test, another passes).
func TestReadsCapturedReports(t *testing.T) {
	paths, _ := filepath.Glob(filepath.Join("..", "..", "tests", "reports", "*", "*", "*"))
	if len(paths) < 5 {
		t.Fatalf("only %d samples", len(paths))
	}
	for _, path := range paths {
		var cases []Case
		switch filepath.Ext(path) {
		case ".xml":
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if cases, err = ReadJUnit(data); err != nil {
				t.Errorf("%s: %v", path, err)
			}
		case ".json":
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			cases = ReadGoTest(f)
			_ = f.Close()
		default:
			continue
		}
		failed, passed := 0, 0
		for _, c := range cases {
			if c.Name == "" || c.Class == "" {
				t.Errorf("%s: %+v", path, c)
			}
			if c.Failure != nil {
				failed++
				if !strings.Contains(strings.ToLower(c.Name), "add") || c.Failure.Text == "" {
					t.Errorf("%s: failed %+v", path, c)
				}
			} else {
				passed++
			}
		}
		if failed == 0 || passed == 0 && !strings.Contains(path, "kotlin") {
			t.Errorf("%s: %d failed, %d passed", path, failed, passed)
		}
	}
}

// Build tools' reports are found under report directories, written during
// the component's run, outside dependency and hidden directories.
func TestBuildReports(t *testing.T) {
	dir := t.TempDir()
	sample, err := os.ReadFile(filepath.Join("..", "..", "tests", "reports", "maven", "3.9.11", "maven-junit-fail.xml"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	files := map[string]time.Time{
		"target/surefire-reports/TEST-calc.CalcTest.xml":       now,
		"core/build/test-results/test/TEST-calc.CoreTest.xml":  now,
		"target/surefire-reports/TEST-calc.OldTest.xml":        now.Add(-time.Hour),
		"node_modules/x/target/surefire-reports/TEST-y.xml":    now,
		"target/surefire-reports/calc.CalcTest.txt":            now,
		"src/TEST-not-a-report.xml":                            now,
		"target/test-reports/TEST-sbt.xml":                     now,
		".hidden/target/surefire-reports/TEST-calc.Hidden.xml": now,
	}
	for name, modified := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, sample, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, path := range BuildReports(dir, now.Add(-time.Minute), now.Add(time.Minute)) {
		rel, _ := filepath.Rel(dir, path)
		got = append(got, filepath.ToSlash(rel))
	}
	want := "core/build/test-results/test/TEST-calc.CoreTest.xml target/surefire-reports/TEST-calc.CalcTest.xml target/test-reports/TEST-sbt.xml"
	if strings.Join(got, " ") != want {
		t.Errorf("%v", got)
	}
}
