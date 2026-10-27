package deps

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ports of tests/test_audit.py and tests/test_outdated.py: the behaviour
// the parity tests check against Python, stated.

func reportText(t *testing.T, name string) string {
	return readFile(t, filepath.Join(repoHome(t), "tests", "audit", "reports", name))
}

func findingReport(severities ...string) string {
	var entries []string
	for i, severity := range severities {
		value := `null`
		if severity != "" {
			value = `"` + severity + `"`
		}
		entries = append(entries, strings.NewReplacer("N", string(rune('0'+i)), "SEV", value).Replace(
			`"N": {"id": N, "module_name": "pkgN", "severity": SEV, "github_advisory_id": "GHSA-aaaa-bbbb-000N", "cves": ["CVE-2020-100N"], "findings": [{"version": "1.0.0"}]}`))
	}
	return `{"advisories": {` + strings.Join(entries, ", ") + `}}`
}

func runAudit(t *testing.T, text, config, failOn string) (int, string) {
	t.Helper()
	var c any
	if config != "" {
		c = jsonObject(t, config)
	}
	var out bytes.Buffer
	status, err := AuditReport(&out, "pnpm", text, "lang", "comp", c, failOn, "2026-10-27")
	if err != nil {
		t.Fatal(err)
	}
	return status, out.String()
}

func TestCapturedAuditReports(t *testing.T) {
	findings, err := ParseAudit("govulncheck", reportText(t, "govulncheck.json"))
	if err != nil || len(findings) != 1 || findings[0].ID != "GO-2021-0113" || findings[0].Package != "golang.org/x/text" {
		t.Errorf("govulncheck reports only called code: %+v %v", findings, err)
	}
	findings, _ = ParseAudit("osv", reportText(t, "osv-java.json"))
	found := false
	for _, f := range findings {
		if f.covers("CVE-2021-44228") {
			found = true
			if f.Severity != "critical" {
				t.Errorf("Log4Shell rated %q", f.Severity)
			}
		}
	}
	if !found {
		t.Error("no Log4Shell in osv-java.json")
	}
	findings, _ = ParseAudit("pip-audit", reportText(t, "pip-audit.json"))
	for _, f := range findings {
		if f.Severity != "" {
			t.Errorf("pip-audit finding rated: %+v", f)
		}
	}
}

func TestThreshold(t *testing.T) {
	if status, _ := runAudit(t, findingReport("moderate", "low"), "", ""); status != 0 {
		t.Error("fails below high")
	}
	status, out := runAudit(t, findingReport("high", "low"), "", "")
	if status != 1 || !strings.Contains(out, "comp: lang 2 vulnerabilities (critical 0, high 1, moderate 0, low 1)") {
		t.Errorf("at high: %d\n%s", status, out)
	}
	if status, _ := runAudit(t, findingReport("high"), "", "critical"); status != 0 {
		t.Error("--fail-on critical fails on high")
	}
	if status, _ := runAudit(t, findingReport("high"), `{"audit": {"failOn": "critical"}}`, ""); status != 0 {
		t.Error("the project's failOn doesn't apply")
	}
	if status, _ := runAudit(t, findingReport("high"), `{"audit": {"failOn": "critical"}}`, "high"); status != 1 {
		t.Error("the flag doesn't override the project's failOn")
	}
	if status, out := runAudit(t, findingReport(""), "", ""); status != 1 || !strings.Contains(out, "unrated 1") {
		t.Errorf("unrated: %d\n%s", status, out)
	}
}

func TestAcceptedRisks(t *testing.T) {
	status, out := runAudit(t, findingReport("critical"),
		`{"audit": {"ignore": [{"id": "GHSA-aaaa-bbbb-0000", "reason": "not called", "until": "2027-01-01"}]}}`, "")
	if status != 0 || !strings.Contains(out, "[accepted until 2027-01-01: not called]") || !strings.Contains(out, ", 1 accepted") {
		t.Errorf("accepted: %d\n%s", status, out)
	}
	if status, _ := runAudit(t, findingReport("critical"),
		`{"audit": {"ignore": [{"id": "CVE-2020-1000", "reason": "r", "until": "2027-01-01"}]}}`, ""); status != 0 {
		t.Error("an alias doesn't match")
	}
	status, out = runAudit(t, findingReport("critical"),
		`{"audit": {"ignore": [{"id": "GHSA-aaaa-bbbb-0000", "reason": "r", "until": "2026-10-26"}]}}`, "")
	if status != 1 || !strings.Contains(out, "the accepted risk GHSA-aaaa-bbbb-0000 expired on 2026-10-26") {
		t.Errorf("expired: %d\n%s", status, out)
	}
	if status, _ := runAudit(t, findingReport("critical"),
		`{"audit": {"ignore": [{"id": "GHSA-aaaa-bbbb-0000", "reason": "r", "until": "2026-10-27"}]}}`, ""); status != 0 {
		t.Error("an entry ends before its until date")
	}
}

func TestEOLWarning(t *testing.T) {
	releases := "3.15.1\t\n3.14.10\t\n3.9.25\teol\n"
	if w := EOLWarning("python", "3.9.7", releases, 0); !strings.Contains(w, "past its end of life (line 3.9)") {
		t.Errorf("3.9.7: %q", w)
	}
	if w := EOLWarning("python", "3.14.2", releases, 0); w != "" {
		t.Errorf("3.14.2: %q", w)
	}
	if w := EOLWarning("python", "3.90.1", releases, 0); w != "" {
		t.Errorf("3.90.1: %q", w)
	}
	if w := EOLWarning("node", "16.9.1", "24.22.1\tlts\n16.20.2\teol\n", 1); !strings.Contains(w, "line 16") {
		t.Errorf("node 16: %q", w)
	}
}

func TestEOLWarningMatchesPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compares with audit.py")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	type eolCase struct {
		Lang     string `json:"lang"`
		Version  string `json:"version"`
		Releases string `json:"releases"`
		Parts    int    `json:"parts"`
	}
	var cases []eolCase
	for _, releases := range []string{"3.15.1\t\n3.14.10\t\n3.9.25\teol\n", "24.22.1\tlts\n16.20.2\teol\n", "10.0.100\t\n8.0.400\teol,lts\n",
		"1\teol\n", "", "\n\t\n", "3.9\r\n3.8\teol\r\n", "v1.2.3\teol\n"} {
		for _, version := range []string{"3.9.7", "3.14.2", "3.90.1", "16.9.1", "8.0.401", "1", "v3.8.1", "3.8", "", "vv1.2.3"} {
			for _, parts := range []int{0, 1, 2, 3, 5} {
				cases = append(cases, eolCase{"lang", version, releases, parts})
			}
		}
	}
	input, _ := json.Marshal(cases)
	script := `
import json, sys
sys.path.insert(0, sys.argv[1])
import audit
print(json.dumps([audit.eol_warning(c["lang"], c["version"], c["releases"], c["parts"] or None) or "" for c in json.load(sys.stdin)]))
`
	cmd := exec.Command("python3", "-c", script, filepath.Join(repoHome(t), "libexec", "lib"))
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	if err := json.Unmarshal(output, &want); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		if got := EOLWarning(c.Lang, c.Version, c.Releases, c.Parts); got != want[i] {
			t.Errorf("%+v: Python %q, Go %q", c, want[i], got)
		}
	}
}

// The rows saved for --json, and collect's array, as outdated.py writes them.
func TestOutdatedJSONMatchesPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compares with outdated.py")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	text := `{"left-pad": {"current": "1.2.0", "wanted": "1.2.0", "latest": "1.3.0"}, "caf\u00e9-\ud83d\ude00\"\\": {"current": "1", "latest": "2"}}`
	goDir, pyDir := t.TempDir(), t.TempDir()
	for _, component := range []string{"comp", "a/b c", "."} {
		var out bytes.Buffer
		if _, err := OutdatedReport(&out, "npm", text, "typescript", component, "", goDir, false); err != nil {
			t.Fatal(err)
		}
		script := `
import sys, io, contextlib
sys.path.insert(0, sys.argv[1])
import outdated
with contextlib.redirect_stdout(io.StringIO()):
    outdated.report("npm", sys.stdin.read(), "typescript", sys.argv[3], json_dir=sys.argv[2])
`
		cmd := exec.Command("python3", "-c", script, filepath.Join(repoHome(t), "libexec", "lib"), pyDir, component)
		cmd.Stdin = strings.NewReader(text)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, output)
		}
	}
	entries, _ := os.ReadDir(pyDir)
	if len(entries) != 3 {
		t.Fatalf("Python wrote %d files", len(entries))
	}
	for _, entry := range entries {
		if got, want := readFile(t, filepath.Join(goDir, entry.Name())), readFile(t, filepath.Join(pyDir, entry.Name())); got != want {
			t.Errorf("%s:\nPython %s\nGo     %s", entry.Name(), want, got)
		}
	}
	collected, err := Collect(goDir)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", filepath.Join(repoHome(t), "libexec", "lib", "outdated.py"), "collect", pyDir)
	want, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if collected+"\n" != string(want) {
		t.Errorf("collect:\nPython %s\nGo     %s", want, collected)
	}
}

func TestCapturedOutdatedReports(t *testing.T) {
	expected := map[string][3]string{
		"npm": {"left-pad", "1.2.0", "1.2.0"}, "uv-tree": {"six", "1.15.0", ""}, "go": {"github.com/google/uuid", "1.3.0", ""},
		"cargo": {"itoa", "1.0.1", ""}, "bundler": {"rack", "2.2.3", ""}, "composer": {"psr/log", "1.1.3", ""},
		"dotnet": {"Newtonsoft.Json", "12.0.3", ""}, "mix": {"jason", "1.2.2", ""}, "dart": {"path", "1.8.0", "1.8.0"},
		"maven": {"org.apache.commons:commons-lang3", "3.12.0", ""},
	}
	for format, want := range expected {
		text := readFile(t, filepath.Join(repoHome(t), "tests", "outdated", "reports", format+".txt"))
		rows, err := ParseOutdated(format, text, "")
		if err != nil || len(rows) != 1 || rows[0].Package != want[0] || rows[0].Current != want[1] || rows[0].Wanted != want[2] {
			t.Errorf("%s: %+v %v", format, rows, err)
		}
	}
}
