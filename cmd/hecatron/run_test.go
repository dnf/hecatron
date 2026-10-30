package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The run commands in Go must do what bin/hecatron (run.sh, run-parallel.sh
// and each-component.sh) does: run the same hooks and tasks, with the same
// arguments, directories and environment, write the same logs, and exit
// with the same status. A fixture project's stub hooks print what they
// were given. Human output is compared with times and bars taken out; the
// live display isn't compared.

const stubHook = `#!/usr/bin/env bash
echo "$(basename "$0") $1: dir=${PWD#"${HECATRON_ROOT}"/} component=${HECATRON_COMPONENT} lang=${HECATRON_LANG}"
echo "  check=${HECATRON_FORMAT_CHECK:-} checked=${HECATRON_FORMAT_CHECKED:+set} failOn=${HECATRON_AUDIT_FAIL_ON:-} outdatedFail=${HECATRON_OUTDATED_FAIL:-}"
echo "  logs=${HECATRON_LOGS#"${HECATRON_ROOT}"} project=${HECATRON_PROJECT} components=${HECATRON_COMPONENTS} job=${HECATRON_JOB_COMMAND##*/} stdin=$(wc -c)"
echo "  stub=${STUB_VAR:-} path=${PATH%%:*}"
`

var runProject = map[string]string{
	".hecatron/config.json": `{"schema": 1, "project": "stubs", "components": ["a", "b"],
		"languages": ["alpha", "beta", "gamma", "delta", "missing", "off"],
		"tasks": {"test": {"docs": "echo docs ran in .${PWD#\"${HECATRON_ROOT}\"}", "flaky": "echo flaky failed >&2; exit 4"},
			"lint": {"style": "echo style"}}}`,
	".hecatron/local.json":                  `{"disabled": ["off"]}`,
	".hecatron/languages/alpha/config.json": `{"abbreviation": "al"}`,
	".hecatron/languages/alpha/test.sh":     stubHook,
	".hecatron/languages/alpha/clean.sh":    stubHook,
	".hecatron/languages/alpha/build.sh":    stubHook,
	".hecatron/languages/alpha/audit.sh":    stubHook,
	".hecatron/languages/alpha/outdated.sh": stubHook,
	".hecatron/languages/alpha/deps.sh":     stubHook,
	".hecatron/languages/alpha/docs.sh":     stubHook + `echo "${HECATRON_LANG}: docs: written to $1/docs"` + "\n",
	".hecatron/languages/gamma/bench.sh":    stubHook + `[ "$1" = b ] && exit 3; echo "no benchmarks in a" >"${HECATRON_SKIPPED}"` + "\n",
	// delta's hooks skip with _hooks.sh's skip: its tests, benchmarks
	// and linter everywhere, its format check in a only.
	".hecatron/languages/delta/config.json": `{}`,
	".hecatron/languages/delta/test.sh":     stubHook + skippingHook + `skip "no tests in $1"` + "\n",
	".hecatron/languages/delta/bench.sh":    stubHook + skippingHook + `skip "no benchmarks"` + "\n",
	".hecatron/languages/delta/lint.sh":     stubHook + skippingHook + `skip "no linter"` + "\n",
	".hecatron/languages/delta/format.sh": stubHook + skippingHook +
		`[ "${HECATRON_FORMAT_CHECK:-}" = 1 ] && [ "$1" = b ] && exit 0; skip "no formatter"` + "\n",
	".hecatron/languages/beta/config.json": `{"dependency": "sh", "env": {"STUB_VAR": "x"},
		"errorPatterns": [{"regex": "^(?P<file>\\S+\\.src):(?P<line>\\d+): (?P<message>.+)$"}]}`,
	".hecatron/languages/beta/test.sh":        stubHook + `[ "$1" = a ] && { echo "beta.src:3: broken" >&2; exit 2; }; exit 0` + "\n",
	".hecatron/languages/beta/lint.sh":        stubHook,
	".hecatron/languages/beta/format.sh":      stubHook,
	".hecatron/languages/gamma/config.json":   `{}`,
	".hecatron/languages/gamma/audit.sh":      stubHook + `echo "nothing to audit: no gamma.lock" >"${HECATRON_SKIPPED}"` + "\n",
	".hecatron/languages/beta/audit.sh":       stubHook + `[ "$1" = b ] && echo "nothing to audit: no beta.lock" >"${HECATRON_SKIPPED}"; exit 0` + "\n",
	".hecatron/languages/gamma/test.sh":       stubHook,
	".hecatron/languages/gamma/lint.sh":       stubHook + `[ "$1" = a ] && echo gamma >>"${HECATRON_FORMAT_CHECKED}"; exit 0` + "\n",
	".hecatron/languages/gamma/format.sh":     stubHook + `[ "${HECATRON_FORMAT_CHECK:-}" = 1 ] && [ "$1" = b ] && exit 1; exit 0` + "\n",
	".hecatron/languages/missing/config.json": `{"dependency": "hecatron-no-such-tool"}`,
	".hecatron/languages/missing/test.sh":     stubHook,
	".hecatron/languages/off/config.json":     `{}`,
	".hecatron/languages/off/test.sh":         stubHook,
	"a/alpha/x":                               "",
	"b/alpha/x":                               "",
	"a/beta/x":                                "",
	"b/beta/x":                                "",
	"b/gamma/x":                               "",
	"a/gamma/x":                               "",
	"a/missing/x":                             "",
	"a/delta/x":                               "",
	"b/delta/x":                               "",
	"a/off/x":                                 "",
}

var runInvocations = [][]string{
	{"test"}, {"test", "-j", "1"}, {"test", "-c", "b"}, {"test", "alpha"}, {"test", "al", "beta"},
	{"test", "beta", "-c", "a"}, {"test", "--component=b", "beta"}, {"test", "docs", "flaky"}, {"test", "off"},
	{"test", "missing"}, {"lint"}, {"lint", "gamma"}, {"lint", "beta", "gamma"}, {"format", "--check"}, {"format"},
	{"audit", "--fail-on", "low"}, {"audit", "--fail-on=moderate", "alpha"}, {"audit", "gamma", "beta"}, {"outdated", "--fail"}, {"deps"},
	{"build"}, {"build", "beta"}, {"clean"}, {"rebuild", "alpha"}, {"retest", "alpha"}, {"test", "gamma", "-c", "a", "-c", "b"},
	{"test", "-c", "zz"}, {"test", "nope"}, {"test", "alpha", "nope"},
	{"docs"}, {"docs", "alpha", "beta"}, {"bench"}, {"bench", "gamma", "-c", "a"}, {"bench", "alpha"},
	// Skipped: by the hooks (delta), for some components (gamma's bench,
	// delta's lint), for want of a hook (alpha's lint, gamma's docs).
	{"test", "delta"}, {"test", "delta", "--fail-on-skip"}, {"test", "delta", "alpha", "--fail-on-skip"},
	{"test", "--fail-on-skip"}, {"lint", "delta"}, {"lint", "alpha"}, {"docs", "gamma", "--fail-on-skip"},
	{"bench", "-c", "a"}, {"bench", "-c", "a", "--fail-on-skip"}, {"retest", "delta", "--fail-on-skip"},
}

// skippingHook makes a stub hook one that can skip (skip in
// languages/_hooks.sh).
const skippingHook = `source "${HECATRON_HOME}/languages/_hooks.sh"
`

var (
	durations = regexp.MustCompile(`\d+\.\ds`)
	bars      = regexp.MustCompile(` *█+`)
)

// normalized is human output without what differs between runs: the time
// axis, times and bars, trailing blanks.
func normalized(output string) string {
	lines := strings.Split(output, "\n")
	var kept []string
	for i, line := range lines {
		if i == 0 && strings.TrimSpace(line) != "" && strings.Trim(line, " 0123456789") == "" {
			continue // the time axis
		}
		line = bars.ReplaceAllString(durations.ReplaceAllString(line, "T"), "")
		kept = append(kept, strings.TrimRight(line, " "))
	}
	return strings.Join(kept, "\n")
}

// logs are the files under a project's logs, history.log's times taken out.
func logs(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(filepath.Join(root, ".logs"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, _ := os.ReadFile(path)
		text := strings.ReplaceAll(string(data), root, "ROOT")
		if info.Name() == "history.log" {
			text = regexp.MustCompile(`=\d+:`).ReplaceAllString(text, "=T:")
		}
		rel, _ := filepath.Rel(root, path)
		files[rel] = text
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestRunCommandsMatchBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	for _, tool := range []string{"bash", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home := repoHome(t)
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "COLUMNS=80", "NO_COLOR=1",
		"HECATRON_OUTPUT=human", "CLAUDECODE=", "GITHUB_ACTIONS=",
		// hct puts hecatron's bin directory on PATH, where a gitleaks would
		// add audit's secret scan, which run.sh hasn't: an empty one.
		"XDG_DATA_HOME=" + t.TempDir()}
	for _, args := range runInvocations {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var results [2]result
			var files [2]map[string]string
			for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
				root := fixture(t, runProject)
				for name := range runProject {
					if strings.HasSuffix(name, ".sh") {
						if err := os.Chmod(filepath.Join(root, name), 0o755); err != nil {
							t.Fatal(err)
						}
					}
				}
				results[i] = runIn(t, filepath.Join(root, "b"), env, append(entry, args...)...)
				results[i].stdout = normalized(strings.ReplaceAll(results[i].stdout, root, "ROOT"))
				results[i].stderr = strings.ReplaceAll(results[i].stderr, root, "ROOT")
				files[i] = logs(t, root)
			}
			if results[0] != results[1] {
				t.Errorf("bin/hecatron: status %d, stdout:\n%s\nstderr:\n%s\nGo: status %d, stdout:\n%s\nstderr:\n%s",
					results[0].status, results[0].stdout, results[0].stderr, results[1].status, results[1].stdout, results[1].stderr)
			}
			if !equalMaps(files[0], files[1]) {
				t.Errorf("logs differ:\nbin/hecatron: %s\nGo:           %s", describeLogs(files[0]), describeLogs(files[1]))
			}
		})
	}
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}
	return true
}

func describeLogs(files map[string]string) string {
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var out strings.Builder
	for _, name := range names {
		out.WriteString("\n--- " + name + "\n" + files[name])
	}
	return out.String()
}

// Agent and JSON output of the same run: the failures, their error lines
// and logs, and the exit status.
func TestAgentOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub hooks are bash")
	}
	home := repoHome(t)
	root := fixture(t, runProject)
	for name := range runProject {
		if strings.HasSuffix(name, ".sh") {
			if err := os.Chmod(filepath.Join(root, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "CLAUDECODE=1", "HECATRON_OUTPUT=", "GITHUB_ACTIONS="}
	got := runIn(t, root, env, binary, "test")
	want := "hct test: 3 passed, 2 failed, 3 skipped (T)\n" +
		"SKIP delta (a/delta/, b/delta/): a: no tests in a; b: no tests in b\n" +
		"FAIL beta (a/beta/) exit 1, T, log .logs/test/beta.log\n" +
		"  beta.src:3: broken\n" +
		"FAIL flaky exit 4, T, log .logs/test/flaky.log\n" +
		"  flaky failed\n" +
		"Rerun: hct test beta flaky\n"
	times := regexp.MustCompile(`\d+(?:\.\d)?s\b`)
	if out := times.ReplaceAllString(got.stdout, "T"); out != want || got.status != 1 || got.stderr != "" {
		t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", got.status, got.stderr, out, want)
	}
	passing := runIn(t, root, env, binary, "test", "alpha", "gamma")
	if !regexp.MustCompile(`^hct test: 2 passed \(\d+\.\ds\)\n$`).MatchString(passing.stdout) || passing.status != 0 {
		t.Errorf("a passing run: %+v", passing)
	}
	jsonRun := runIn(t, root, append(env, "HECATRON_OUTPUT=json"), binary, "test", "beta")
	for _, part := range []string{`"status": "failed"`, `"exitCode": 1`, `"text": "beta.src:3: broken"`, `"rerun": "hct test beta"`} {
		if !strings.Contains(jsonRun.stdout, part) {
			t.Errorf("no %s in:\n%s", part, jsonRun.stdout)
		}
	}
	// Skipped: a job, by its hooks, and for want of a hook.
	skipped := runIn(t, root, append(env, "HECATRON_OUTPUT=json"), binary, "test", "delta")
	for _, part := range []string{`"status": "skipped"`, `"exitCode": 0`, `"reason": "a: no tests in a; b: no tests in b"`,
		`"skipped": "no tests in a"`, `"skipped": 1`} {
		if !strings.Contains(skipped.stdout, part) || skipped.status != 0 {
			t.Errorf("no %s in (status %d):\n%s", part, skipped.status, skipped.stdout)
		}
	}
	noHook := runIn(t, root, env, binary, "lint", "alpha")
	if want := "hct lint: 0 passed, 1 skipped (T)\nSKIP alpha: alpha has no separate linter: its build treats warnings as errors (hecatron test alpha)\n"; times.ReplaceAllString(noHook.stdout, "T") != want || noHook.status != 0 {
		t.Errorf("lint alpha: %+v, want:\n%s", noHook, want)
	}
	strict := runIn(t, root, append(env, "HECATRON_OUTPUT=json", "HECATRON_FAIL_ON_SKIP=1"), binary, "test", "delta")
	if strict.status != 1 || !strings.Contains(strict.stdout, `"exitCode": 1`) || strict.stderr != "hecatron: nothing ran: every job skipped (--fail-on-skip)\n" {
		t.Errorf("HECATRON_FAIL_ON_SKIP=1: %+v", strict)
	}
	if mixed := runIn(t, root, env, binary, "test", "delta", "alpha", "--fail-on-skip"); mixed.status != 0 {
		t.Errorf("--fail-on-skip with a job that passed: %+v", mixed)
	}
	ci := runIn(t, root, append(env, "HECATRON_OUTPUT=", "CLAUDECODE=", "GITHUB_ACTIONS=true", "GITHUB_WORKSPACE="+root), binary, "test", "beta")
	if !strings.Contains(ci.stdout, "::error file=a/beta/beta.src,line=3,title=hct test beta::broken\n") {
		t.Errorf("no annotation in:\n%s", ci.stdout)
	}
	bad := runIn(t, root, env, binary, "test", "--output", "xml")
	if bad.status != 2 || !strings.Contains(bad.stderr, "--output must be human, agent, json or ci") {
		t.Errorf("--output xml: %+v", bad)
	}
}

// A failure whose language command isn't on PATH exits 3.
func TestMissingToolchainExitsThree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub hooks are bash")
	}
	home := repoHome(t)
	files := map[string]string{
		".hecatron/config.json":                `{"schema": 1, "languages": ["tool"]}`,
		".hecatron/languages/tool/config.json": `{"dependency": "hecatron-no-such-tool"}`,
		".hecatron/languages/tool/test.sh":     "hecatron-no-such-tool test\n",
		"tool/x":                               "",
	}
	root := fixture(t, files)
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=agent"}
	got := runIn(t, root, env, binary, "test", "tool")
	if got.status != 3 || !strings.Contains(got.stdout, "FAIL tool (tool/) toolchain not found") {
		t.Errorf("%+v", got)
	}
	everything := runIn(t, root, env, binary, "test")
	if everything.status != 0 || !strings.HasPrefix(everything.stdout, "hct test: 0 passed, 1 skipped") {
		t.Errorf("the parallel run skips it as not installed: %+v", everything)
	}
}
