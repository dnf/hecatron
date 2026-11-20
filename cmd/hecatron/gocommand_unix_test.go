//go:build unix

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var goProject = map[string]string{
	".hecatron/config.json": `{"schema": 1, "project": "stubs", "languages": ["alpha", "beta"],
		"implPaths": {"alpha": "alpha", "beta": "beta"}}`,
	"alpha/.keep":                           "",
	"beta/.keep":                            "",
	".hecatron/languages/alpha/config.json": `{}`,
	".hecatron/languages/alpha/install.sh": "#!/usr/bin/env bash\n" +
		"for i in 1 2 3; do echo \"Setting up libnoise$i ...\"; done\necho installed >\"${HECATRON_ROOT}/alpha-installed\"\n",
	".hecatron/languages/alpha/test.sh":    "#!/usr/bin/env bash\necho running specs\necho '3 examples, 0 failures'\n",
	".hecatron/languages/beta/config.json": `{}`,
	".hecatron/languages/beta/install.sh":  "#!/usr/bin/env bash\necho 'fetching beta'\necho 'error: beta: the download failed' >&2\nexit 1\n",
	".hecatron/languages/beta/test.sh":     "#!/usr/bin/env bash\necho '1 example, 0 failures'\n",
}

// hct go: one line for what the languages are, one for the install (its
// output in a log), then the test run's own agent output.
func TestGoCommand(t *testing.T) {
	home := repoHome(t)
	root := fixture(t, goProject)
	for name := range goProject {
		if strings.HasSuffix(name, ".sh") {
			if err := os.Chmod(filepath.Join(root, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=agent", "GITHUB_ACTIONS="}
	times := regexp.MustCompile(`\d+(?:\.\d)?s\b`)

	got := runIn(t, root, env, binary, "go", "alpha")
	want := "hct go: alpha\n" +
		"install: done, T, log .logs/go/install.log\n" +
		"hct test: 1 passed (T)\n" +
		"PASS alpha T, log .logs/test/alpha.log: 3 examples, 0 failures\n" +
		"Test again: hct test alpha\n"
	if out := times.ReplaceAllString(got.stdout, "T"); out != want || got.status != 0 {
		t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", got.status, got.stderr, out, want)
	}
	log, _ := os.ReadFile(filepath.Join(root, ".logs", "go", "install.log"))
	if !strings.Contains(string(log), "Setting up libnoise3") || strings.Contains(got.stdout, "libnoise") {
		t.Errorf("the installer's output belongs in the log, not the output:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha-installed")); err != nil {
		t.Error("alpha's installer didn't run")
	}

	// A failed install: its error lines, and the tests still run.
	got = runIn(t, root, env, binary, "go", "beta")
	out := times.ReplaceAllString(got.stdout, "T")
	if !strings.HasPrefix(out, "hct go: beta\nFAIL install exit 1, T, log .logs/go/install.log\n  error: beta: the download failed\n") ||
		!strings.Contains(out, "PASS beta T, log .logs/test/beta.log: 1 example, 0 failures\n") || got.status != 1 {
		t.Errorf("a failed install: status %d, stdout:\n%s", got.status, out)
	}

	// JSON: the test run's document alone on stdout.
	got = runIn(t, root, append(env, "HECATRON_OUTPUT=json"), binary, "go", "alpha")
	if !strings.HasPrefix(got.stdout, "{") || !strings.Contains(got.stdout, `"tally": [`) ||
		!strings.Contains(got.stderr, "hct go: alpha\ninstall: done") {
		t.Errorf("json: stdout:\n%s\nstderr:\n%s", got.stdout, got.stderr)
	}

	if got := runIn(t, root, env, binary, "go", "gamma"); got.status != 2 ||
		!strings.Contains(got.stderr, "'gamma' isn't one of the project's languages (alpha beta)") {
		t.Errorf("an unknown language: %+v", got)
	}
}
