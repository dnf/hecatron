package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hct workflows writes the CI generated.ci names (or --ci), hct add and
// remove regenerate it, and hct check flags it out of date and reports bad
// CI settings; bin/hecatron without the binary says these need it. What
// the files hold is tested in internal/generate.
func TestGeneratedCI(t *testing.T) {
	home := repoHome(t)
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES="}
	hct := func(root string, args ...string) result {
		return runIn(t, root, env, append([]string{binary}, args...)...)
	}
	root := fixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["age", "date"], "languages": ["go"],
			"generated": {"ci": "gitlab"}, "ci": {"cache": true}}`,
		"age/go/go.mod":        "module a\n",
		"date/rust/Cargo.toml": "",
	})
	gitlab := filepath.Join(root, ".gitlab-ci.yml")

	if got := hct(root, "workflows"); got.status != 0 || got.stdout != "wrote .gitlab-ci.yml\n" {
		t.Fatalf("workflows: %+v", got)
	}
	if got := hct(root, "check"); got.status != 0 {
		t.Errorf("check after workflows: %+v", got)
	}
	if got := hct(root, "add", "rust"); got.status != 0 || !strings.Contains(got.stdout, "wrote .gitlab-ci.yml") {
		t.Errorf("add: %+v", got)
	}
	if text, _ := os.ReadFile(gitlab); !strings.Contains(string(text), "\nrust:\n") {
		t.Errorf("add didn't regenerate the pipeline:\n%s", text)
	}
	if err := os.WriteFile(gitlab, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := hct(root, "check"); got.status != 1 || !strings.Contains(got.stdout, "out of date: .gitlab-ci.yml") {
		t.Errorf("check with a stale pipeline: %+v", got)
	}
	if got := hct(root, "workflows", "--check"); got.status != 1 {
		t.Errorf("workflows --check with a stale pipeline: %+v", got)
	}
	if got := hct(root, "workflows", "--ci", "github"); got.status != 1 ||
		!strings.Contains(got.stderr, "no workflow path configured") {
		t.Errorf("--ci github without generated.workflows: %+v", got)
	}
	if got := hct(root, "workflows", "--ci=jenkins"); got.status != 2 || !strings.Contains(got.stderr, "--ci must be github or gitlab") {
		t.Errorf("--ci jenkins: %+v", got)
	}

	bad := fixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "languages": [], "generated": {"ci": "jenkins"}, "ci": {"cache": 1}}`,
	})
	got := hct(bad, "check")
	if got.status != 1 || !strings.Contains(got.stdout, ".hecatron/config.json: generated.ci must be github or gitlab") ||
		!strings.Contains(got.stdout, ".hecatron/config.json: ci.cache must be true or false") {
		t.Errorf("check with bad CI settings: %+v", got)
	}
	if got := hct(bad, "workflows"); got.status != 1 || !strings.Contains(got.stderr, "generated.ci must be") {
		t.Errorf("workflows with bad CI settings: %+v", got)
	}

	plain := fixture(t, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": ["go"]}`, "go/go.mod": ""})
	if got := hct(plain, "workflows", "--ci", "gitlab"); got.status != 0 || got.stdout != "wrote .gitlab-ci.yml\n" {
		t.Errorf("--ci gitlab without config: %+v", got)
	}

	t.Run("bin/hecatron without the binary", func(t *testing.T) {
		script := filepath.Join(home, "bin", "hecatron")
		noGo := []string{"HECATRON_HOME=" + home, "HECATRON_NO_GO=1"}
		for _, c := range []struct {
			dir  string
			args []string
			want string
		}{
			{root, []string{"workflows"}, "config.json's generated.ci, ci needs hecatron's binary"},
			{root, []string{"check"}, "needs hecatron's binary"},
			{plain, []string{"workflows", "--ci", "gitlab"}, "hecatron workflows --ci needs hecatron's binary"},
		} {
			got := runIn(t, c.dir, noGo, append([]string{"bash", script}, c.args...)...)
			if got.status != 1 || !strings.Contains(got.stderr, c.want) {
				t.Errorf("%v: %+v", c.args, got)
			}
		}
	})
}
