package deps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The Go port must print what libexec/lib/audit.py and outdated.py print,
// byte for byte, with the same exit status, on the same reports: every
// report captured from a real tool (tests/audit/reports, tests/outdated/
// reports, tests/output/<tool>/<version>/), the shapes the Python tests
// write by hand, and each captured JSON report with one value at a time
// replaced by another type or removed, so a report's odd shapes give the
// same findings or the same error. testdata/report.py runs the Python side.

type parityCase struct {
	Kind    string `json:"kind"` // audit or outdated
	Format  string `json:"format"`
	Text    string `json:"text"`
	Config  any    `json:"config,omitempty"`
	FailOn  string `json:"failOn,omitempty"`
	Today   string `json:"today"`
	Direct  string `json:"direct,omitempty"`
	Fail    bool   `json:"fail,omitempty"`
	name    string
	mutated bool
}

type parityResult struct {
	Out      string `json:"out"`
	Err      string `json:"err"`
	Status   int    `json:"status"`
	Uncaught string `json:"uncaught"`
}

func repoHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// capturedReports are the real tools' reports: kind, format and file.
func capturedReports(t *testing.T, home string) [][3]string {
	var reports [][3]string
	auditFormats := map[string]string{
		"bundle-audit.txt": "bundle-audit", "composer.json": "composer", "dotnet.json": "dotnet",
		"govulncheck.json": "govulncheck", "npm.json": "npm", "osv-java.json": "osv", "osv-rust.json": "osv",
		"pip-audit.json": "pip-audit",
	}
	for name, format := range auditFormats {
		reports = append(reports, [3]string{"audit", format, filepath.Join(home, "tests", "audit", "reports", name)})
	}
	outdatedFiles, _ := filepath.Glob(filepath.Join(home, "tests", "outdated", "reports", "*.txt"))
	for _, path := range outdatedFiles {
		reports = append(reports, [3]string{"outdated", strings.TrimSuffix(filepath.Base(path), ".txt"), path})
	}
	// tests/output/<tool>/<version>/audit-<format>-<case>.* and
	// outdated-<format>-<case>.*: captured with tests/output/capture-deps.sh.
	samples, _ := filepath.Glob(filepath.Join(home, "tests", "output", "*", "*", "*-*-*"))
	for _, path := range samples {
		name := filepath.Base(path)
		kind, rest, _ := strings.Cut(name, "-")
		if kind != "audit" && kind != "outdated" {
			continue
		}
		format := ""
		for candidate := range auditParsers {
			if kind == "audit" && strings.HasPrefix(rest, candidate+"-") && len(candidate) > len(format) {
				format = candidate
			}
		}
		for candidate := range outdatedParsers {
			if kind == "outdated" && strings.HasPrefix(rest, candidate+"-") && len(candidate) > len(format) {
				format = candidate
			}
		}
		if format == "" {
			t.Errorf("%s: no format in its name (audit-<format>-<case>)", path)
			continue
		}
		reports = append(reports, [3]string{kind, format, path})
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i][2]+reports[i][1] < reports[j][2]+reports[j][1] })
	return reports
}

func jsonObject(t *testing.T, text string) any {
	t.Helper()
	value, err := jsonLoads(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// configs are the project configs each captured audit report is read with.
func configs(t *testing.T) []any {
	return []any{
		nil,
		jsonObject(t, `{"audit": {"failOn": "critical"}}`),
		jsonObject(t, `{"audit": {"failOn": "low", "ignore": [
			{"id": "GHSA-35jh-r3h4-6jhm", "reason": "not called", "until": "2027-01-01"},
			{"id": "CVE-2021-44228", "reason": "not reachable", "until": "2026-10-26"},
			{"id": "RUSTSEC-2020-0071", "reason": "reviewed", "until": "2026-10-27"},
			{"id": "PYSEC-2023-74", "reason": "reviewed", "until": "2030-01-01"},
			{"id": "GO-2021-0113", "until": "2030-01-01"},
			"not an entry", {"reason": "no id"}]}}`),
		jsonObject(t, `{"audit": {"failOn": "severe"}}`),
		jsonObject(t, `{"audit": []}`),
		jsonObject(t, `[]`),
		jsonObject(t, `{"audit": {"ignore": 5}}`),
	}
}

func parityCases(t *testing.T, home string) []parityCase {
	var cases []parityCase
	add := func(c parityCase) {
		if c.Today == "" {
			c.Today = "2026-10-27"
		}
		cases = append(cases, c)
	}
	for _, report := range capturedReports(t, home) {
		kind, format, path := report[0], report[1], report[2]
		text := readFile(t, path)
		name := strings.TrimPrefix(path, home+"/")
		if kind == "audit" {
			for i, config := range configs(t) {
				add(parityCase{Kind: kind, Format: format, Text: text, Config: config, name: fmt.Sprintf("%s config %d", name, i)})
			}
			for _, failOn := range []string{"critical", "moderate", "low", "bogus"} {
				add(parityCase{Kind: kind, Format: format, Text: text, FailOn: failOn, name: name + " --fail-on " + failOn})
			}
		} else {
			add(parityCase{Kind: kind, Format: format, Text: text, name: name})
			add(parityCase{Kind: kind, Format: format, Text: text, Fail: true, name: name + " --fail"})
		}
		for i, mutated := range mutations(text) {
			add(parityCase{Kind: kind, Format: format, Text: mutated, name: fmt.Sprintf("%s mutation %d", name, i), mutated: true})
		}
		// Each report read as every other format: an error, or nothing.
		parsers := []string{}
		if kind == "audit" {
			for other := range auditParsers {
				parsers = append(parsers, other)
			}
		} else {
			for other := range outdatedParsers {
				parsers = append(parsers, other)
			}
		}
		sort.Strings(parsers)
		for _, other := range parsers {
			if other != format {
				add(parityCase{Kind: kind, Format: other, Text: text, name: name + " as " + other})
			}
		}
	}
	for _, c := range handWritten() {
		add(c)
	}
	return cases
}

// handWritten are the reports the Python tests write: formats no fixture
// produces, and what isn't a report.
func handWritten() []parityCase {
	var cases []parityCase
	audit := func(format, text string) {
		cases = append(cases, parityCase{Kind: "audit", Format: format, Text: text, name: "audit " + format + " " + text})
	}
	outdated := func(format, text string) {
		cases = append(cases, parityCase{Kind: "outdated", Format: format, Text: text, name: "outdated " + format + " " + text})
	}
	audit("pnpm", `{"advisories": {"1": {"id": 1, "module_name": "minimist", "severity": "moderate", "title": "Prototype Pollution", "github_advisory_id": "GHSA-vh95-rmgr-6w4m", "cves": ["CVE-2020-7598"], "findings": [{"version": "0.0.8"}, {"version": "1.2.0"}, {"version": "0.0.8"}], "url": "https://github.com/advisories/GHSA-vh95-rmgr-6w4m"}, "2": {"id": 2, "module_name": "x", "severity": "info", "cves": "CVE-2020-1", "findings": []}}}`)
	line := `{"type": "auditAdvisory", "data": {"advisory": {"id": 7, "module_name": "minimist", "severity": "high", "github_advisory_id": "GHSA-xvch-5gv4-984h", "cves": [], "findings": [{"version": "1.2.5"}]}}}`
	audit("yarn-classic", line+"\n"+line+"\n"+`{"type": "auditSummary", "data": {}}`)
	audit("yarn-classic", line+"  \u00a0\n"+`{"type": "auditAdvisory", "data": {"advisory": {"id": 7.0, "module_name": "other"}}}`)
	audit("yarn-berry", `{"value": "minimist", "children": {"ID": 1096287, "Issue": "Prototype Pollution", "URL": "https://github.com/advisories/GHSA-xvch-5gv4-984h", "Severity": "critical", "Tree Versions": ["1.2.5", 2]}}`+"\n"+`{"value": "y", "children": {"ID": 5, "URL": null, "Severity": "moderate", "Tree Versions": "12"}}`)
	audit("bun", `{"minimist": [{"id": 1, "url": "https://github.com/advisories/GHSA-xvch-5gv4-984h", "title": "Prototype Pollution", "severity": "critical", "vulnerable_versions": "<1.2.6"}], "other": {"x": 1}, "z": [{"id": 3, "url": "", "severity": "9.8"}]}`)
	audit("osv", "")
	audit("osv", "  \n")
	audit("osv", `{"results": [{"packages": [{"package": {"name": "a", "version": "1"}, "vulnerabilities": [{"id": "CVE-2020-1", "aliases": ["GHSA-aaaa-bbbb-cccc"], "database_specific": {"severity": "MEDIUM"}, "summary": "s"}]}]}]}`)
	audit("osv", `{"results": [{"packages": [{"package": {"name": "a"}, "groups": [{"ids": []}]}]}]}`)
	audit("npm", `{"auditReportVersion": 2, "vulnerabilities": {"a": {"via": ["b", {"source": 5, "name": "a", "severity": "high", "url": "https://x/GHSA-cfgh-jmpq-rvwx CVE-2021-1234"}]}, "b": {"via": [{"source": 6, "severity": "7.5", "title": null}]}}}`)
	audit("composer", `{"advisories": []}`)
	audit("composer", `{"advisories": {"a/b": {"0": {"advisoryId": "PKSA-1", "cve": "CVE-2022-1111", "link": "https://github.com/advisories/GHSA-2345-6789-cfgh", "title": "t", "severity": "medium", "affectedVersions": "<1"}}}}`)
	audit("dotnet", `{"version": 1, "projects": [{"frameworks": [{"topLevelPackages": [{"id": "A", "resolvedVersion": "1", "vulnerabilities": [{"severity": "Critical", "advisoryurl": null}]}]}]}]}`)
	audit("bundle-audit", "No vulnerabilities found\n")
	audit("bundle-audit", "Name: rack\nVersion: 2.0\nAdvisory: OSVDB-1\nCriticality: Unknown\nTitle: x\n\nName: \nName: b\nGHSA: GHSA-2345-6789-cfgh\r\nCriticality: High\n")
	audit("govulncheck", `{"progress": {}}`)
	audit("pip-audit", `{"dependencies": [{"name": "a", "version": "1", "vulns": [{"id": "PYSEC-2020-1", "description": "# Heading\n\n`+strings.Repeat("long text ", 20)+`", "aliases": ["CVE-2020-2"]}]}]}`)
	for _, format := range []string{"npm", "pip-audit", "govulncheck", "composer", "bundle-audit", "dotnet", "osv", "bun", "yarn-classic", "yarn-berry"} {
		for _, text := range []string{"", "npm ERR! something", "{}", "[]", "null", "\ufeff{}", "{\"a\": 1} x", "{\"a\": [1, 2}", "\"unterminated", "{\"a\": \"\\q\"}", "{\"dependencies\": 5}", "{\"advisories\": 5, \"projects\": 5, \"results\": 5, \"vulnerabilities\": 5}", strings.Repeat("x", 5000)} {
			audit(format, text)
		}
	}
	outdated("pnpm", `{"left-pad": {"current": "1.2.0", "latest": "1.3.0", "wanted": "1.2.0"}, "b": [{"current": "v1", "latest": "v2"}, {"current": "1", "latest": "1"}], "A": {"current": null, "latest": "3"}}`)
	outdated("yarn-classic", `{"type": "info", "data": "Color legend"}`+"\nnot json\n"+`{"type": "table", "data": {"head": ["Package", "Current", "Wanted", "Latest", "Package Type", "URL"], "body": [["left-pad", "1.2.0", "1.2.0", "1.3.0", "dependencies", "https://x"], ["b", "1"]]}}`)
	outdated("uv-pip", `[{"name": "six", "version": "1.15.0", "latest_version": "1.17.0"}]`)
	outdated("uv-pip", ``)
	outdated("uv-pip", `{}`)
	outdated("uv-tree", "fixture v0.1.0\n├── six v1.15.0 (latest: v1.17.0)\n└── pytest v7.0.0 (group: dev) (latest: v8.3.0)\n")
	outdated("cargo", "   Unchanged itoa v1.0.1 (available: v1.0.18)\n    Updating serde v1.0.100 -> v1.0.200\n   Unchanged memchr v2.0.0 (available: v2.7.0)\n     Locking 1 package\n")
	outdated("mix", "Dependency  Only  Current  Latest  Status\nex_doc      dev   0.30.0   0.34.0  Update possible\njason             1.4.0    1.4.0   Up-to-date\nplug  1.0  2.0  Update not possible\n")
	outdated("maven", "The following dependencies in Dependencies have newer versions:\n  com.example.something.very.long:artifact-with-a-long-name ...\n                                                        1.0 -> 2.0\n  junit:junit ............................................ 4.12 -> 4.13.2\n[INFO]   a:b ... 1 -> 2\n  c:d ...\nnothing\n")
	outdated("gradle", `{"outdated": {"dependencies": [{"group": "org.slf4j", "name": "slf4j-api", "version": "1.7.36", "available": {"release": "2.0.16"}}, {"name": "n", "available": {"release": null, "milestone": "3.0-M1"}}]}}`)
	outdated("cabal", "Outdated dependencies:\nbase >=4 && <4.15 (latest: 4.20.0.0)\ntext ==1.2.* (latest: 2.1.2)\n")
	outdated("bundler", "rack (newest 3.1.8, installed 2.2.3, requested = 2.2.3)\nrake (newest 13.0, installed 12.0)\n")
	outdated("dart", `{"packages": [{"package": "path", "kind": "direct", "current": {"version": "1.8.0"}, "upgradable": {"version": "1.8.0"}, "latest": {"version": "1.9.0"}}, {"package": "t", "kind": "transitive", "latest": {"version": "9"}}, {"package": "d", "kind": "dev", "current": null, "latest": {"version": "2"}}]}`)
	outdated("go", `{"Path": "m", "Main": true}`+"\n"+`{"Path": "a", "Version": "v1.0.0", "Update": {"Version": "v1.1.0"}}`+"\n"+`{"Path": "b", "Version": "v1.0.0", "Indirect": true, "Update": {"Version": "v2.0.0"}}`)
	outdated("composer", `{"locked": [{"name": "psr/log", "version": "1.1.3", "latest": "3.0.0", "latest-status": "update-possible"}, {"name": "x", "version": "1", "latest": "1", "latest-status": "up-to-date"}]}`)
	outdated("dotnet", `{"projects": [{"frameworks": [{"topLevelPackages": [{"id": "A", "resolvedVersion": "1", "latestVersion": "2"}]}, {"topLevelPackages": [{"id": "A", "resolvedVersion": "1", "latestVersion": "3"}]}]}]}`)
	for _, format := range []string{"npm", "uv-pip", "go", "composer", "dotnet", "dart", "gradle"} {
		for _, text := range []string{"", "npm ERR! something", "{}", "[]", "null", "5", "{\"a\": 1} x", "[1, 2"} {
			outdated(format, text)
		}
	}
	return cases
}

// mutations are a JSON report (or a stream of them) with one value at a
// time replaced by a value of another type, or an object's key removed.
func mutations(text string) []string {
	stream, err := jsonValues(text)
	if err != nil || len(stream) == 0 {
		return nil
	}
	replacements := []any{nil, Number{"0", false}, "", []any{}, newObject(), "x", Number{"9.8", true}, true,
		[]any{"x", Number{"7", false}}, Number{"7", false}, "GHSA-2345-6789-cfgh"}
	var paths [][]any
	var walk func(value any, path []any)
	walk = func(value any, path []any) {
		paths = append(paths, append([]any(nil), path...))
		switch v := value.(type) {
		case *Object:
			for _, key := range v.Keys {
				walk(v.Values[key], append(path, key))
			}
		case []any:
			for i, item := range v {
				walk(item, append(path, i))
			}
		}
	}
	for i, value := range stream {
		walk(value, []any{i})
	}
	const most = 120
	step := 1
	if len(paths) > most {
		step = len(paths) / most
	}
	var out []string
	for k := 0; k < len(paths); k += step {
		path := paths[k]
		for r, replacement := range []any{replacements[k%len(replacements)], replacements[(k+5)%len(replacements)], "delete"} {
			if replacement == "delete" && (len(path) < 2 || r != 2) {
				continue
			}
			copied := copyValue(stream).([]any)
			if !replace(copied, path, replacement) {
				continue
			}
			var parts []string
			for _, value := range copied {
				parts = append(parts, encode(value))
			}
			out = append(out, strings.Join(parts, "\n"))
		}
	}
	return out
}

func copyValue(value any) any {
	switch v := value.(type) {
	case *Object:
		o := newObject()
		for _, key := range v.Keys {
			o.set(key, copyValue(v.Values[key]))
		}
		return o
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = copyValue(item)
		}
		return list
	}
	return value
}

// replace sets the value at path (removes it, for "delete" in an object).
func replace(root []any, path []any, value any) bool {
	var parent any = root
	for _, step := range path[:len(path)-1] {
		switch p := parent.(type) {
		case []any:
			parent = p[step.(int)]
		case *Object:
			parent = p.Values[step.(string)]
		}
	}
	last := path[len(path)-1]
	switch p := parent.(type) {
	case []any:
		if value == "delete" {
			return false
		}
		p[last.(int)] = value
	case *Object:
		key := last.(string)
		if value == "delete" {
			delete(p.Values, key)
			for i, k := range p.Keys {
				if k == key {
					p.Keys = append(p.Keys[:i], p.Keys[i+1:]...)
					break
				}
			}
			return true
		}
		p.Values[key] = copyValue(value)
	}
	return true
}

// encode writes a value as JSON.
func encode(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case Number:
		return v.Text
	case string:
		return pyJSONString(v)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = encode(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Object:
		parts := make([]string, len(v.Keys))
		for i, key := range v.Keys {
			parts[i] = pyJSONString(key) + ": " + encode(v.Values[key])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

// encodable is a case as JSON for report.py (its config as JSON text).
func (c parityCase) MarshalJSON() ([]byte, error) {
	type plain parityCase
	data, err := json.Marshal(plain(c))
	if err != nil || c.Config == nil {
		return data, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	fields["config"] = json.RawMessage(encode(c.Config))
	return json.Marshal(fields)
}

func goResult(c parityCase) parityResult {
	var out, errOut bytes.Buffer
	var status int
	if c.Kind == "audit" {
		status = RunAuditReport(&out, &errOut, c.Format, c.Text, "lang", "comp", c.Config, c.FailOn, c.Today)
	} else {
		status = RunOutdatedReport(&out, &errOut, c.Format, c.Text, "lang", "comp", c.Direct, "", c.Fail)
	}
	return parityResult{Out: out.String(), Err: errOut.String(), Status: status}
}

func pythonResults(t *testing.T, home string, cases []parityCase) []parityResult {
	t.Helper()
	input, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", filepath.Join(home, "internal", "deps", "testdata", "report.py"))
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("report.py: %v", err)
	}
	var results []parityResult
	if err := json.Unmarshal(output, &results); err != nil {
		t.Fatal(err)
	}
	return results
}

// sameError says whether two error outputs differ only where Python's own
// wording can't be matched: which pair of values a failed sort names.
func sameError(python, golang string) bool {
	if python == golang {
		return true
	}
	cut := func(s string) string {
		// Python 3.14 says which use of the value was unhashable.
		s = unhashableUse.ReplaceAllString(s, "$1")
		if i := strings.Index(s, "('<' not supported between instances of"); i >= 0 {
			if j := strings.Index(s[i:], "); its output was:"); j >= 0 {
				return s[:i] + s[i+j:]
			}
		}
		return s
	}
	return cut(python) == cut(golang)
}

var unhashableUse = regexp.MustCompile(`cannot use '\w+' as a (?:set element|dict key) \((unhashable type: '\w+')\)`)

func TestReportsMatchPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compares with audit.py and outdated.py, which the tests run on macOS and Linux")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	home := repoHome(t)
	direct := filepath.Join(t.TempDir(), "metadata.json")
	if err := os.WriteFile(direct, []byte(`{"packages": [{"dependencies": [{"name": "itoa"}, {"name": "serde"}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := parityCases(t, home)
	for i := range cases {
		if cases[i].Format == "cargo" && i%2 == 0 {
			cases[i].Direct = direct
		}
	}
	want := pythonResults(t, home, cases)
	compared, mutated, uncaught := 0, 0, 0
	for i, c := range cases {
		got := goResult(c)
		w := want[i]
		if w.Uncaught != "" {
			// main() would end in a traceback: Go must fail too.
			uncaught++
			if got.Status != 1 || got.Err == "" {
				t.Errorf("%s: Python fails with %s; Go: status %d, %q", c.name, w.Uncaught, got.Status, got.Out)
			}
			continue
		}
		compared++
		if c.mutated {
			mutated++
		}
		if got.Status != w.Status || got.Out != w.Out || !sameError(w.Err, got.Err) {
			t.Errorf("%s:\nPython (%d):\n%s%s\nGo (%d):\n%s%s", c.name, w.Status, w.Out, w.Err, got.Status, got.Out, got.Err)
		}
	}
	t.Logf("%d reports compared with Python (%d of them mutated), %d where Python fails with a traceback", compared, mutated, uncaught)
}
