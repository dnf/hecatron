//go:build unix

package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var updatePlan = flag.Bool("update-plan", false, "rewrite testdata/plan's golden files")

// hct test --changed --plan says what the run would do (each changed file
// and what it selects, the jobs, the skips known up front) and runs
// nothing: the hooks and tasks here would leave a file behind. The layout
// is #260's: a Cargo workspace at the root beside clients/go, a stray file,
// an agent's worktree.
func TestRunPlan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := repoHome(t)
	hook := "touch \"${HECATRON_ROOT}/ran-${HECATRON_LANG}\"\n"
	root := fixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["."], "languages": ["go", "rust", "missing", "off"],
			"implPath": "clients/{lang}", "implPaths": {"rust": "."},
			"tasks": {"test": {"codegen": "touch ran-codegen", "always": "touch ran-always"}},
			"taskPaths": {"codegen": ["codegen/"]}}`,
		".hecatron/local.json":                    `{"disabled": ["off"]}`,
		".hecatron/languages/go/config.json":      `{"detect": ["go.mod"], "dependency": "sh"}`,
		".hecatron/languages/go/test.sh":          hook,
		".hecatron/languages/rust/config.json":    `{"detect": ["Cargo.toml"], "sourceFiles": ["*.rs", "Cargo.lock"], "dependency": "sh"}`,
		".hecatron/languages/rust/test.sh":        hook,
		".hecatron/languages/rust/clean.sh":       hook,
		".hecatron/languages/missing/config.json": `{"dependency": "hecatron-no-such-tool"}`,
		".hecatron/languages/missing/test.sh":     hook,
		".hecatron/languages/off/config.json":     `{}`,
		".hecatron/languages/off/test.sh":         hook,
		".gitignore":                              ".logs/\nran-*\n",
		"Cargo.toml":                              "[workspace]\nmembers = [\"crates/*\"]\n",
		"crates/a/Cargo.toml":                     "[package]\n",
		"crates/a/src/lib.rs":                     "",
		"clients/go/go.mod":                       "module x\n",
		"clients/missing/x":                       "",
		"clients/off/x":                           "",
		"codegen/gen.txt":                         "",
		"scripts/x.sh":                            "",
	})
	gitEnv := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + filepath.Join(root, ".no-gitconfig"),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com"}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, append(os.Environ(), gitEnv...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(root, "init", "-q", "-b", "main")
	git(root, "add", "-A")
	git(root, "commit", "-qm", "main")
	git(root, "checkout", "-qb", "work")
	for name, content := range map[string]string{
		"crates/a/src/lib.rs":             "pub fn a() {}\n",
		"clients/go/notes.txt":            "",
		"clients/missing/x":               "x\n",
		"clients/off/x":                   "x\n",
		"codegen/gen.txt":                 "x\n",
		"scripts/x.sh":                    "x\n",
		"erl_crash.dump":                  "",
		".claude/worktrees/y/src/lib.rs":  "",
		".claude/worktrees/x/.git":        "gitdir: /nowhere/.git/worktrees/x\n",
		".claude/worktrees/x/src/main.rs": "",
	} {
		writeFile(t, filepath.Join(root, name), content)
	}
	git(filepath.Join(root, ".claude/worktrees/y"), "init", "-q")

	env := append([]string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "CI=", "TF_BUILD=", "GITHUB_ACTIONS=",
		"GITHUB_BASE_REF=", "CI_MERGE_REQUEST_DIFF_BASE_SHA=", "BITBUCKET_PR_DESTINATION_BRANCH=", "SYSTEM_PULLREQUEST_TARGETBRANCH="}, gitEnv...)
	mergeBase := regexp.MustCompile(`\b[0-9a-f]{40}\b|merge base [0-9a-f]{10}`)
	golden := func(name string, args ...string) {
		t.Helper()
		got := runIn(t, root, env, append([]string{binary}, args...)...)
		if got.status != 0 || got.stderr != "" {
			t.Fatalf("%v: status %d, stderr %s", args, got.status, got.stderr)
		}
		out := mergeBase.ReplaceAllString(got.stdout, "MERGE-BASE")
		path := filepath.Join("testdata", "plan", name)
		if *updatePlan {
			writeFile(t, path, out)
			return
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if out != string(want) {
			t.Errorf("%v:\n%s\nwant (%s; go test -run TestRunPlan -update-plan rewrites it):\n%s", args, out, path, want)
		}
	}
	golden("changed.json", "test", "--changed", "--plan", "--output", "json")
	golden("changed.agent", "test", "--changed", "--plan", "--output", "agent")
	golden("changed.human", "test", "--changed", "--plan", "--output=human")
	golden("all.human", "test", "--plan", "--output", "human")
	golden("retest.agent", "retest", "--plan", "--output", "agent", "rust")

	ran, _ := filepath.Glob(filepath.Join(root, "ran-*"))
	if len(ran) > 0 {
		t.Errorf("--plan ran something: %v", ran)
	}
	if _, err := os.Stat(filepath.Join(root, ".logs")); !os.IsNotExist(err) {
		t.Errorf("--plan wrote logs: %v", err)
	}

	for _, bad := range [][]string{{"test", "--plan", "--output", "junit"}, {"test", "nope", "--plan"}, {"audit", "--changed", "--plan"}} {
		if got := runIn(t, root, env, append([]string{binary}, bad...)...); got.status != 2 || got.stderr == "" {
			t.Errorf("%v: %+v", bad, got)
		}
	}
	// The bash runner has neither --plan nor --changed.
	for _, flag := range []string{"--plan", "--changed"} {
		got := runIn(t, root, append(env, "HECATRON_NO_GO=1"), filepath.Join(home, "bin", "hecatron"), "test", flag)
		if got.status != 2 || !strings.Contains(got.stderr, flag+" needs hecatron's binary") {
			t.Errorf("run.sh %s: %+v", flag, got)
		}
	}
	if ran, _ := filepath.Glob(filepath.Join(root, "ran-*")); len(ran) > 0 {
		t.Errorf("run.sh ran something: %v", ran)
	}
}
