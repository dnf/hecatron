//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hct test --changed runs only the languages and components a change
// touches, and says what it selected and why.
func TestChanged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := repoHome(t)
	hook := "echo \"$1\" >>\"${HECATRON_ROOT}/ran-${HECATRON_LANG}\"\n"
	root := fixture(t, map[string]string{
		".hecatron/config.json":               `{"schema": 1, "components": ["a", "b"], "languages": ["one", "two"]}`,
		".hecatron/languages/one/config.json": `{}`,
		".hecatron/languages/one/test.sh":     hook,
		".hecatron/languages/one/lint.sh":     hook,
		".hecatron/languages/two/config.json": `{}`,
		".hecatron/languages/two/test.sh":     hook,
		".gitignore":                          ".logs/\nran-*\n",
		"a/one/x":                             "",
		"b/one/x":                             "",
		"a/two/x":                             "",
		"README.md":                           "",
	})
	gitEnv := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + filepath.Join(root, ".no-gitconfig"),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com"}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), gitEnv...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-qm", "main")
	git("checkout", "-qb", "work")
	writeFile(t, filepath.Join(root, "b/one/x"), "changed\n")

	env := append([]string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=agent", "CI=", "TF_BUILD=",
		"GITHUB_BASE_REF=", "CI_MERGE_REQUEST_DIFF_BASE_SHA=", "BITBUCKET_PR_DESTINATION_BRANCH=", "SYSTEM_PULLREQUEST_TARGETBRANCH="}, gitEnv...)
	ran := func() map[string]string {
		got := map[string]string{}
		for _, lang := range []string{"one", "two"} {
			data, _ := os.ReadFile(filepath.Join(root, "ran-"+lang))
			if len(data) > 0 {
				got[lang] = strings.TrimSpace(string(data))
			}
			if err := os.Remove(filepath.Join(root, "ran-"+lang)); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
		return got
	}
	times := regexp.MustCompile(`\(\d+\.\ds\)|merge base [0-9a-f]+`)

	got := runIn(t, root, env, binary, "test", "--changed")
	want := "Changed since main (the default branch, X): 1 file; running one [b] (b/one/x)\n" +
		"hct test: 1 passed X\n"
	if out := times.ReplaceAllString(got.stdout, "X"); out != want || got.status != 0 {
		t.Errorf("status %d, stdout:\n%s\nwant:\n%s\nstderr: %s", got.status, out, want, got.stderr)
	}
	if r := ran(); len(r) != 1 || r["one"] != "b" {
		t.Errorf("ran %v, want one in b only", r)
	}

	// -c narrows further; nothing left is nothing to run.
	if got := runIn(t, root, env, binary, "test", "--changed", "-c", "a"); got.status != 0 || len(ran()) != 0 ||
		!strings.HasSuffix(got.stdout, "running one [b] (b/one/x); none in -c a\n") {
		t.Errorf("-c a: %+v", got)
	}

	git("commit", "-qam", "one")
	writeFile(t, filepath.Join(root, "README.md"), "x\n")
	nothing := runIn(t, root, env, binary, "lint", "--changed", "HEAD")
	if nothing.status != 0 || len(ran()) != 0 || times.ReplaceAllString(nothing.stdout, "X") !=
		"Changed since HEAD (--changed, X): 1 file; nothing to run: none is in a language's directory or a build file (README.md)\n" {
		t.Errorf("nothing relevant: %+v", nothing)
	}
	jsonRun := runIn(t, root, append(env, "HECATRON_OUTPUT=json"), binary, "test", "--changed=main")
	if !strings.Contains(jsonRun.stdout, `"changed": {`) || !strings.Contains(jsonRun.stdout, `"languages": [`) || jsonRun.status != 0 {
		t.Errorf("json: %+v", jsonRun)
	}
	ran()

	writeFile(t, filepath.Join(root, ".hecatron/languages/two/test.sh"), hook+"# changed\n")
	if got := runIn(t, root, env, binary, "test", "--changed"); got.status != 0 || !equalMaps(ran(), map[string]string{"one": "b", "two": "a"}) {
		t.Errorf("a hook: %+v", got)
	}
	human := runIn(t, root, append(env, "HECATRON_OUTPUT=human"), binary, "test", "--changed")
	if !strings.HasPrefix(human.stdout, "Changed since main (the default branch, merge base ") {
		t.Errorf("human output: %q", human.stdout)
	}
	ran()

	for _, bad := range [][]string{{"test", "--changed", "nope"}, {"test", "one", "--changed"}, {"audit", "--changed"}} {
		if got := runIn(t, root, env, append([]string{binary}, bad...)...); got.status != 2 || got.stderr == "" {
			t.Errorf("%v: %+v", bad, got)
		}
	}
}
