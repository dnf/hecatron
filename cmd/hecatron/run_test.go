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
		"languages": ["alpha", "beta", "gamma", "missing", "off"],
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
	".hecatron/languages/beta/config.json": `{"dependency": "sh", "env": {"STUB_VAR": "x"},
		"errorPatterns": [{"regex": "^(?P<file>\\S+\\.src):(?P<line>\\d+): (?P<message>.+)$"}]}`,
	".hecatron/languages/beta/test.sh":        stubHook + `[ "$1" = a ] && { echo "beta.src:3: broken" >&2; exit 2; }; exit 0` + "\n",
	".hecatron/languages/beta/lint.sh":        stubHook,
	".hecatron/languages/beta/format.sh":      stubHook,
	".hecatron/languages/gamma/config.json":   `{}`,
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
	"a/off/x":                                 "",
}

var runInvocations = [][]string{
	{"test"}, {"test", "-j", "1"}, {"test", "-c", "b"}, {"test", "alpha"}, {"test", "al", "beta"},
	{"test", "beta", "-c", "a"}, {"test", "--component=b", "beta"}, {"test", "docs", "flaky"}, {"test", "off"},
	{"test", "missing"}, {"lint"}, {"lint", "gamma"}, {"lint", "beta", "gamma"}, {"format", "--check"}, {"format"},
	{"audit", "--fail-on", "low"}, {"audit", "--fail-on=moderate", "alpha"}, {"outdated", "--fail"}, {"deps"},
	{"build"}, {"build", "beta"}, {"clean"}, {"rebuild", "alpha"}, {"retest", "alpha"}, {"test", "gamma", "-c", "a", "-c", "b"},
	{"test", "-c", "zz"}, {"test", "nope"}, {"test", "alpha", "nope"},
}

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
	filepath.Walk(filepath.Join(root, ".logs"), func(path string, info os.FileInfo, err error) error {
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
		"HECATRON_OUTPUT=human", "CLAUDECODE=", "GITHUB_ACTIONS="}
	for _, args := range runInvocations {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var results [2]result
			var files [2]map[string]string
			for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
				root := fixture(t, runProject)
				for name := range runProject {
					if strings.HasSuffix(name, ".sh") {
						os.Chmod(filepath.Join(root, name), 0o755)
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
			os.Chmod(filepath.Join(root, name), 0o755)
		}
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "CLAUDECODE=1", "HECATRON_OUTPUT=", "GITHUB_ACTIONS="}
	got := runIn(t, root, env, binary, "test")
	want := "hct test: 3 passed, 2 failed, 2 skipped (T)\n" +
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
