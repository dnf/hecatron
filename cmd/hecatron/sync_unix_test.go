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

var updateSync = flag.Bool("update-sync", false, "rewrite testdata/sync's golden files")

// hct sync after commits that change a lock file, a pin file, the config
// and nothing relevant: deps run through the hooks (stubs here), versions
// said with whether they're installed, hct check run, and one line when
// nothing matters.
func TestSync(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := repoHome(t)
	root := fixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "languages": ["one", "two"]}`,
		".hecatron/languages/one/config.json": `{"cache": {"lockFiles": ["one.lock"]},
			"pinFiles": [{"file": ".one-version", "format": "python-version"}]}`,
		".hecatron/languages/one/installed.sh": "printf '1.2.7\\t/opt/one\\n'\n",
		".hecatron/languages/one/deps.sh": "echo \"fetching for $1\"\necho \"$1\" >>\"${HECATRON_ROOT}/ran-${HECATRON_LANG}\"\n" +
			"if [ -e \"${HECATRON_ROOT}/fail\" ]; then echo 'error: lock file out of date'; exit 1; fi\n",
		".hecatron/languages/two/config.json": `{"cache": {"lockFiles": ["two.lock"]}}`,
		".hecatron/languages/two/deps.sh":     "echo \"$1\" >>\"${HECATRON_ROOT}/ran-${HECATRON_LANG}\"\n",
		".gitignore":                          ".logs/\nran-*\nfail\n",
		"a/one/one.lock":                      "1\n",
		"a/one/.one-version":                  "1.2\n",
		"b/one/x":                             "",
		"a/two/two.lock":                      "1\n",
		"b/two/x":                             "",
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
	commit := func(files map[string]string) {
		t.Helper()
		for name, content := range files {
			writeFile(t, filepath.Join(root, name), content)
		}
		git("add", "-A")
		git("commit", "-qm", "change")
	}
	env := append([]string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=agent", "HECATRON_AS_OF=off",
		"HECATRON_TIMEOUT=", "CLAUDECODE="}, gitEnv...)
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
	sync := func(extra []string, args ...string) result {
		t.Helper()
		return runIn(t, filepath.Join(root, "b"), append(append([]string(nil), env...), extra...), append([]string{binary, "sync"}, args...)...)
	}
	hashes := regexp.MustCompile(`\b[0-9a-f]{40}\b|\b[0-9a-f]{10}\b`)
	normal := func(out string) string {
		out = strings.ReplaceAll(out, root, "ROOT")
		out = hashes.ReplaceAllString(out, "COMMIT")
		out = regexp.MustCompile(`"duration": [0-9.e-]+`).ReplaceAllString(out, `"duration": 0`)
		out = regexp.MustCompile(`"description": "[^"]*"`).ReplaceAllString(out, `"description": "LIMIT"`)
		return regexp.MustCompile(`\b\d+\.\ds\b`).ReplaceAllString(out, "TIME")
	}
	golden := func(name, got string) {
		t.Helper()
		path := filepath.Join("testdata", "sync", name)
		if *updateSync {
			writeFile(t, path, got)
			return
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("%s differs (go test -run TestSync -update-sync rewrites it):\n%s\nwant:\n%s", name, got, want)
		}
	}

	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-qm", "main")
	if got := sync(nil); got.status != 0 || got.stdout != "hct sync: no previous HEAD in the reflog to compare with; name a ref (hct sync ORIG_HEAD)\n" {
		t.Errorf("no previous HEAD: %+v", got)
	}

	// A lock file and a pin file: deps run for that component; the version
	// is said, not installed.
	commit(map[string]string{"a/one/one.lock": "2\n", "a/one/.one-version": "1.3\n"})
	got := sync(nil)
	golden("lock-and-pin.agent", normal(got.stdout))
	if got.status != 0 || !equalMaps(ran(), map[string]string{"one": "a"}) {
		t.Errorf("lock and pin: %+v", got)
	}
	got = sync([]string{"HECATRON_OUTPUT="}, "--output", "json")
	golden("lock-and-pin.json", normal(got.stdout))
	if got.status != 0 || !equalMaps(ran(), map[string]string{"one": "a"}) {
		t.Errorf("json: %+v", got)
	}
	if got := sync(nil, "--no-deps"); got.status != 0 || len(ran()) != 0 ||
		!strings.Contains(got.stdout, "  deps: one [a] (a/one/one.lock) changed: run hct deps one -c a\n") {
		t.Errorf("--no-deps: %+v", got)
	}
	writeFile(t, filepath.Join(root, "fail"), "")
	got = sync(nil)
	golden("deps-failed.agent", normal(got.stdout))
	if got.status != 1 {
		t.Errorf("deps failed: %+v", got)
	}
	if err := os.Remove(filepath.Join(root, "fail")); err != nil {
		t.Fatal(err)
	}
	ran()

	// Two commits back, named: both languages' lock files, the pin now
	// installed.
	commit(map[string]string{"a/two/two.lock": "2\n", "a/one/.one-version": "1.2.7\n"})
	got = sync(nil, "HEAD~2")
	golden("ref.agent", normal(got.stdout))
	if got.status != 0 || !equalMaps(ran(), map[string]string{"one": "a", "two": "a"}) {
		t.Errorf("a ref: %+v", got)
	}

	// The config: hct check runs, passing, then failing (a generated
	// Dockerfile that isn't there).
	commit(map[string]string{".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "languages": ["one", "two"], "x": 1}`})
	if got := sync(nil); got.status != 0 || len(ran()) != 0 || normal(got.stdout) !=
		"hct sync: since COMMIT (HEAD@{1}, 1 commit): 1 file changed\n  check: .hecatron/config.json changed; hct check passed\n" {
		t.Errorf("config: %+v", got)
	}
	commit(map[string]string{".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "languages": ["one", "two"], "generated": {"dockerfile": "Dockerfile"}}`})
	got = sync(nil)
	if got.status != 1 || !strings.Contains(got.stdout, "  check: .hecatron/config.json changed; hct check failed:\n    ") {
		t.Errorf("check failed: %+v", got)
	}

	// Nothing relevant; nothing at all.
	commit(map[string]string{"README.md": "x\n", "b/one/x": "y\n"})
	if got := sync(nil); got.status != 0 || len(ran()) != 0 || normal(got.stdout) !=
		"hct sync: since COMMIT (HEAD@{1}, 1 commit): 2 files changed; nothing for hecatron to do\n" {
		t.Errorf("nothing relevant: %+v", got)
	}
	if got := sync(nil, "HEAD"); got.status != 0 || normal(got.stdout) != "hct sync: nothing changed since COMMIT (HEAD)\n" {
		t.Errorf("nothing changed: %+v", got)
	}
	if got := sync([]string{"HECATRON_OUTPUT=json"}); got.status != 0 || !strings.Contains(got.stdout, `"nothing": true`) {
		t.Errorf("nothing, json: %+v", got)
	}

	for _, bad := range [][]string{{"nope"}, {"HEAD", "HEAD"}, {"--bogus"}, {"--output", "xml"}, {"--output"}} {
		if got := sync(nil, bad...); got.status != 2 || got.stderr == "" || got.stdout != "" {
			t.Errorf("%v: %+v", bad, got)
		}
	}
	if got := sync(nil, "help"); got.status != 0 || !strings.HasPrefix(got.stdout, "Usage: hecatron sync") {
		t.Errorf("help: %+v", got)
	}
	t.Run("bin/hecatron without the binary", func(t *testing.T) {
		got := runIn(t, root, []string{"HECATRON_HOME=" + home, "HECATRON_NO_GO=1"}, "bash", filepath.Join(home, "bin", "hecatron"), "sync")
		if got.status != 1 || !strings.Contains(got.stderr, "sync needs hecatron's binary") {
			t.Errorf("%+v", got)
		}
	})
}
