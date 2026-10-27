package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The audit and outdated hooks read their tools' reports with the binary
// when the Go runner runs them (HECATRON_BINARY), with audit.py and
// outdated.py when bin/hecatron does; the two print the same. Stub hooks
// use languages/_audit.sh and _outdated.sh with recorded reports: a real
// npm audit report, one an accepted risk covers, and one that isn't a
// report.

const auditStub = `#!/usr/bin/env bash
source "${HECATRON_HOME}/languages/_audit.sh"
echo "${HECATRON_BINARY:+binary}" >"${HECATRON_ROOT}/reader-audit-$1"
cp "${HECATRON_ROOT}/reports/audit-$1" "${_audit_report_file}"
audit_finish npm
`

const outdatedStub = `#!/usr/bin/env bash
source "${HECATRON_HOME}/languages/_outdated.sh"
echo "${HECATRON_BINARY:+binary}" >"${HECATRON_ROOT}/reader-outdated-$1"
cp "${HECATRON_ROOT}/reports/outdated-$1" "${_outdated_report_file}"
outdated_finish npm
`

func TestReportReadersMatchPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	for _, tool := range []string{"bash", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home := repoHome(t)
	npmReport, err := os.ReadFile(filepath.Join(home, "tests", "audit", "reports", "npm.json"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".hecatron/config.json": `{"schema": 1, "project": "reports", "components": ["a", "b", "c"], "languages": ["alpha"],
			"audit": {"ignore": [{"id": "GHSA-35jh-r3h4-6jhm", "reason": "not called", "until": "2999-01-01"},
				{"id": "CVE-2020-28500", "reason": "old", "until": "2001-01-01"}]}}`,
		".hecatron/languages/alpha/config.json": `{}`,
		".hecatron/languages/alpha/audit.sh":    auditStub,
		".hecatron/languages/alpha/outdated.sh": outdatedStub,
		"reports/audit-a":                       string(npmReport),
		"reports/audit-b":                       `{"auditReportVersion": 2, "vulnerabilities": {}}`,
		"reports/audit-c":                       "npm ERR! code ENOLOCK\r\nnpm ERR! audit This command requires a lock file.\r\n",
		"reports/outdated-a":                    `{"left-pad": {"current": "1.2.0", "wanted": "1.2.0", "latest": "1.3.0"}}`,
		"reports/outdated-b":                    ``,
		"reports/outdated-c":                    `npm ERR! nope`,
		"a/alpha/x":                             "",
		"b/alpha/x":                             "",
		"c/alpha/x":                             "",
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "COLUMNS=80", "NO_COLOR=1",
		"HECATRON_OUTPUT=human", "CLAUDECODE=", "GITHUB_ACTIONS=", "HECATRON_BINARY="}
	for _, args := range [][]string{{"audit", "alpha"}, {"audit", "--fail-on", "low", "alpha"}, {"audit", "alpha", "-c", "b"},
		{"outdated", "alpha"}, {"outdated", "--fail", "alpha", "-c", "a"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var results [2]result
			var readers [2]string
			for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
				root := fixture(t, files)
				for _, name := range []string{"audit.sh", "outdated.sh"} {
					if err := os.Chmod(filepath.Join(root, ".hecatron", "languages", "alpha", name), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				results[i] = runIn(t, root, env, append(entry, args...)...)
				results[i].stdout = normalized(strings.ReplaceAll(results[i].stdout, root, "ROOT"))
				results[i].stderr = strings.ReplaceAll(results[i].stderr, root, "ROOT")
				data, _ := os.ReadFile(filepath.Join(root, "reader-"+args[0]+"-a"))
				if args[len(args)-1] == "b" {
					data, _ = os.ReadFile(filepath.Join(root, "reader-"+args[0]+"-b"))
				}
				readers[i] = strings.TrimSpace(string(data))
			}
			if results[0] != results[1] {
				t.Errorf("bin/hecatron: status %d, stdout:\n%s\nstderr:\n%s\nGo: status %d, stdout:\n%s\nstderr:\n%s",
					results[0].status, results[0].stdout, results[0].stderr, results[1].status, results[1].stdout, results[1].stderr)
			}
			if readers != [2]string{"", "binary"} {
				t.Errorf("readers: bin/hecatron %q, Go %q; want Python, then the binary", readers[0], readers[1])
			}
			if results[1].stdout == "" {
				t.Error("no output")
			}
		})
	}
}

// The commands themselves, as the hooks call them: the report file read
// with universal newlines, the project's config from HECATRON_ROOT, the
// usage errors.
func TestReportCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compares with audit.py and outdated.py")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	home := repoHome(t)
	root := fixture(t, map[string]string{
		".hecatron/config.json": `{"audit": {"failOn": "low"}}`,
		"bundle.txt":            "Name: rack\r\nVersion: 2.0\r\nCVE: CVE-2022-1234\r\nCriticality: Medium\r\nTitle: t\r\n",
		"mix.txt":               "Dependency  Current  Latest  Status\rjason  1.2.2  1.4.4  Update possible\r",
		"releases.txt":          "3.15.1\t\n3.9.25\teol\n",
	})
	env := []string{"HECATRON_HOME=" + home, "HECATRON_ROOT=" + root, "HECATRON_AUDIT_FAIL_ON="}
	pyAudit := filepath.Join(home, "libexec", "lib", "audit.py")
	pyOutdated := filepath.Join(home, "libexec", "lib", "outdated.py")
	cases := []struct {
		python, golang []string
		stdin          string
	}{
		{[]string{"python3", pyAudit, "report", "bundle-audit", "bundle.txt", "ruby", "."},
			[]string{binary, "_audit", "report", "bundle-audit", "bundle.txt", "ruby", "."}, ""},
		{[]string{"python3", pyOutdated, "report", "mix", "mix.txt", "elixir", "c"},
			[]string{binary, "_outdated", "report", "mix", "mix.txt", "elixir", "c"}, ""},
		{[]string{"python3", pyAudit, "eol", "python", "3.9.7"},
			[]string{binary, "_audit", "eol", "python", "3.9.7"}, "3.15.1\t\n3.9.25\teol\n"},
		{[]string{"python3", pyAudit, "eol", "typescript", "16.1.0"},
			[]string{binary, "_audit", "eol", "typescript", "16.1.0"}, "24.1.0\tlts\n16.20.2\teol\n"},
	}
	for _, c := range cases {
		var results [2]result
		for i, argv := range [][]string{c.python, c.golang} {
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Dir = root
			cmd.Env = append(cmd.Environ(), env...)
			cmd.Stdin = strings.NewReader(c.stdin)
			var stdout, stderr strings.Builder
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			status := 0
			if exit, ok := err.(*exec.ExitError); ok {
				status = exit.ExitCode()
			}
			results[i] = result{stdout.String(), stderr.String(), status}
		}
		if results[0] != results[1] || results[1].stdout == "" {
			t.Errorf("%v:\nPython %+v\nGo     %+v", c.golang[1:], results[0], results[1])
		}
	}
	for _, args := range [][]string{{"_audit"}, {"_audit", "report", "nope", "f", "l", "c"}, {"_outdated", "collect"}} {
		if got := runIn(t, root, env, append([]string{binary}, args...)...); got.status != 2 || got.stderr == "" {
			t.Errorf("%v: %+v", args, got)
		}
	}
}
