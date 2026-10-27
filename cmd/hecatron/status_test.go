package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// hct status runs natively (no bash or Python on PATH), chooses its format
// as the runner does, and refuses unknown arguments; bin/hecatron without
// the binary says it needs it. Its output is tested in internal/status.
func TestStatus(t *testing.T) {
	home := repoHome(t)
	root := fixture(t, cliProjects["components"])
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "PATH=" + t.TempDir(), "HECATRON_OUTPUT=", "CLAUDECODE="}
	cases := []struct {
		args   []string
		env    []string
		status int
		check  func(string) bool
	}{
		{[]string{"status", "--output", "agent"}, nil, 0, func(out string) bool {
			return strings.HasPrefix(out, "hecatron ") && strings.Contains(out, "\ngo ") && strings.Contains(out, "Next: ")
		}},
		{[]string{"status"}, []string{"HECATRON_OUTPUT=json"}, 0, func(out string) bool {
			var s map[string]any
			return json.Unmarshal([]byte(out), &s) == nil && s["config"] == "saved"
		}},
		{[]string{"status", "--output=human"}, nil, 0, func(out string) bool { return strings.Contains(out, "\nNext:\n") }},
		{[]string{"status"}, []string{"CLAUDECODE=1"}, 0, func(out string) bool { return strings.Contains(out, "\nNext: ") }},
		{[]string{"status", "help"}, nil, 0, func(out string) bool { return strings.HasPrefix(out, "Usage: hecatron status") }},
		{[]string{"status", "--bogus"}, nil, 2, func(out string) bool { return out == "" }},
		{[]string{"status", "--output", "xml"}, nil, 2, func(out string) bool { return out == "" }},
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
		got := runIn(t, root, []string{"HECATRON_HOME=" + home, "HECATRON_NO_GO=1"}, "bash", filepath.Join(home, "bin", "hecatron"), "status")
		if got.status != 1 || !strings.Contains(got.stderr, "status needs hecatron's binary") {
			t.Errorf("%+v", got)
		}
	})
}
