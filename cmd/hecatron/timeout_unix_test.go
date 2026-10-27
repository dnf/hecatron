//go:build unix

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// A job past its time limit is stopped, with the processes its hook
// started, and reported as timed out with its limit and where it came
// from; the run exits 1. A job's own limit comes from its history.
func TestTimeLimit(t *testing.T) {
	home := repoHome(t)
	files := map[string]string{
		".hecatron/config.json":                 `{"schema": 1, "languages": ["slow", "quick"]}`,
		".hecatron/languages/slow/config.json":  `{}`,
		".hecatron/languages/slow/test.sh":      "sleep 30 &\necho $! >\"${HECATRON_ROOT}/child.pid\"\necho waiting\nwait\n",
		".hecatron/languages/quick/config.json": `{}`,
		".hecatron/languages/quick/test.sh":     "echo quick\n",
		"slow/x":                                "",
		"quick/x":                               "",
	}
	root := fixture(t, files)
	childGone := func() {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, "child.pid"))
		if err != nil {
			t.Fatal("the hook didn't start its child")
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if syscall.Kill(pid, 0) == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Errorf("the hook's child (pid %d) outlived the job", pid)
		}
		if err := os.Remove(filepath.Join(root, "child.pid")); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=agent", "HECATRON_TIMEOUT="}
	times := regexp.MustCompile(`\d+(?:\.\d)?s\b`)

	got := runIn(t, root, env, binary, "test", "--timeout", "1s")
	want := "hct test: 1 passed, 1 timed out (T)\n" +
		"TIMEOUT slow (slow/) stopped at its T limit (--timeout), T, log .logs/test/slow.log\n" +
		"  waiting\n" +
		"Rerun: hct test slow\n"
	if out := times.ReplaceAllString(got.stdout, "T"); out != want || got.status != 1 {
		t.Errorf("status %d, stdout:\n%s\nwant:\n%s", got.status, out, want)
	}
	childGone()

	named := runIn(t, root, append(env, "HECATRON_OUTPUT=json", "HECATRON_TIMEOUT=1"), binary, "test", "slow")
	for _, part := range []string{`"status": "timed out"`, `"timedOut": 1`, `"source": "HECATRON_TIMEOUT"`} {
		if !strings.Contains(named.stdout, part) {
			t.Errorf("no %s in:\n%s", part, named.stdout)
		}
	}
	if named.status != 1 {
		t.Errorf("named run: status %d", named.status)
	}
	childGone()

	// quick passed in the first run, so it now has a limit of its own;
	// --timeout 0 turns limits off.
	quick := runIn(t, root, append(env, "HECATRON_OUTPUT=json"), binary, "test", "quick")
	if !strings.Contains(quick.stdout, `"source": "history"`) || !strings.Contains(quick.stdout, `"description": "the 10m minimum`) {
		t.Errorf("no limit from history in:\n%s", quick.stdout)
	}
	off := runIn(t, root, append(env, "HECATRON_OUTPUT=json", "HECATRON_TIMEOUT=1s"), binary, "test", "quick", "--timeout=0")
	if strings.Contains(off.stdout, "timeLimit") || off.status != 0 {
		t.Errorf("--timeout 0: %+v", off)
	}
	bad := runIn(t, root, env, binary, "test", "--timeout", "soon")
	if bad.status != 2 || !strings.Contains(bad.stderr, "--timeout must be a duration such as 90s") {
		t.Errorf("--timeout soon: %+v", bad)
	}
}
