package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// hct advisor runs natively (no bash or Python on PATH), chooses its format
// as the runner does, exits 1 only with --fail-on, refuses unknown
// arguments and languages; bin/hecatron without the binary says it needs
// it. Its findings and output are tested in internal/advisor.
func TestAdvisor(t *testing.T) {
	home := repoHome(t)
	root := fixture(t, cliProjects["components"])
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "PATH=" + t.TempDir(), "HECATRON_OUTPUT=", "CLAUDECODE="}
	cases := []struct {
		args   []string
		env    []string
		status int
		check  func(string) bool
	}{
		{[]string{"advisor", "--output", "agent"}, nil, 0, func(out string) bool { return strings.HasPrefix(out, "advisor: ") }},
		{[]string{"advisor"}, []string{"HECATRON_OUTPUT=json"}, 0, func(out string) bool {
			var r map[string]any
			return json.Unmarshal([]byte(out), &r) == nil && r["config"] == "saved" && r["findings"] != nil
		}},
		{[]string{"advisor", "--output=human"}, nil, 0, func(out string) bool { return strings.HasPrefix(out, "hct advisor  ") }},
		{[]string{"advisor"}, []string{"CLAUDECODE=1"}, 0, func(out string) bool { return strings.HasPrefix(out, "advisor: ") }},
		{[]string{"advisor", "--output", "agent", "--fail-on", "suggestion"}, nil, 1, func(out string) bool { return strings.Contains(out, "ci/configured") }},
		{[]string{"advisor", "--output", "agent", "--fail-on=recommended"}, nil, 1, func(out string) bool { return strings.Contains(out, "ci/configured") }},
		{[]string{"advisor", "--output", "agent", "go"}, nil, 0, func(out string) bool {
			return strings.HasPrefix(out, "advisor: ") && !strings.Contains(out, "ci/configured")
		}},
		{[]string{"advisor", "help"}, nil, 0, func(out string) bool { return strings.HasPrefix(out, "Usage: hecatron advisor") }},
		{[]string{"advisor", "--bogus"}, nil, 2, func(out string) bool { return out == "" }},
		{[]string{"advisor", "--fail-on", "warning"}, nil, 2, func(out string) bool { return out == "" }},
		{[]string{"advisor", "--fail-on"}, nil, 2, func(out string) bool { return out == "" }},
		{[]string{"advisor", "--output", "xml"}, nil, 2, func(out string) bool { return out == "" }},
		{[]string{"advisor", "cobol"}, nil, 2, func(out string) bool { return out == "" }},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " ")+strings.Join(c.env, " "), func(t *testing.T) {
			got := runIn(t, filepath.Join(root, "age"), append(append([]string(nil), env...), c.env...), append([]string{binary}, c.args...)...)
			if got.status != c.status || !c.check(got.stdout) {
				t.Errorf("status %d, stdout:\n%s\nstderr:\n%s", got.status, got.stdout, got.stderr)
			}
		})
	}
	t.Run("bin/hecatron without the binary", func(t *testing.T) {
		got := runIn(t, root, []string{"HECATRON_HOME=" + home, "HECATRON_NO_GO=1"}, "bash", filepath.Join(home, "bin", "hecatron"), "advisor")
		if got.status != 1 || !strings.Contains(got.stderr, "advisor needs hecatron's binary") {
			t.Errorf("%+v", got)
		}
	})
	t.Run("check validates advisor.ignore", func(t *testing.T) {
		root := fixture(t, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": [],
			"advisor": {"ignore": [{"id": "go/sum-committed", "reason": "vendored"}, {"id": "go/nope", "reason": "r"}]}}`})
		got := runIn(t, root, env, binary, "check")
		if got.status != 1 || !strings.Contains(got.stdout, ".hecatron/config.json: advisor.ignore go/nope: no such rule") || strings.Contains(got.stdout, "sum-committed") {
			t.Errorf("%+v", got)
		}
	})
}
