//go:build unix

package runner

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// alive reports whether a process exists and isn't a zombie.
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return runtime.GOOS != "linux"
	}
	end := strings.LastIndexByte(string(stat), ')')
	return end >= 0 && stat[end+2] != 'Z'
}

// A job past its limit is stopped with everything it started: a child in
// its process group, one that ignores SIGTERM, and (on Linux) one that
// left the group with setsid. It is marked timed out, says so in its log,
// and is recorded in history.log as timed out.
func TestTimeLimitStopsTheJobAndItsChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups")
	}
	defer func(grace time.Duration) { StopGrace = grace }(StopGrace)
	StopGrace = 300 * time.Millisecond
	dir := t.TempDir()
	script := `sleep 30 & echo $! >child
(trap '' TERM; sleep 30) & echo $! >stubborn
`
	wantChildren := 2
	if _, err := exec.LookPath("setsid"); err == nil && runtime.GOOS == "linux" {
		script += "setsid sleep 30 & echo $! >escaped\n"
		wantChildren = 3
	}
	script += "echo started; wait\n"
	job := Job{Label: "slow", Limit: Limit{Duration: 500 * time.Millisecond, Source: LimitFlag},
		Exec: func(ctx context.Context, out io.Writer) Outcome {
			cmd := exec.CommandContext(ctx, "bash", "-c", script)
			cmd.Dir, cmd.Stdout, cmd.Stderr = dir, out, out
			stopped := Supervise(cmd)
			err := cmd.Run()
			stopped()
			if err != nil {
				return Outcome{ExitCode: 1}
			}
			return Outcome{}
		}}
	var pids []int
	t.Cleanup(func() { // never leave one running, whatever failed
		for _, pid := range pids {
			if alive(pid) {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	began := time.Now()
	results, _ := Run(context.Background(), []Job{job}, Options{LogDir: dir})
	for _, name := range []string{"child", "stubborn", "escaped"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		pids = append(pids, pid)
		if alive(pid) {
			t.Errorf("%s (pid %d) still running", name, pid)
		}
	}
	if len(pids) < wantChildren {
		t.Fatalf("the hook didn't start its children: %v", pids)
	}
	if took := time.Since(began); took > 5*time.Second {
		t.Errorf("took %v", took)
	}
	r := results[0]
	if r.Status != TimedOut || ExitStatus(results) != 1 {
		t.Errorf("status %q, exit %d", r.Status, ExitStatus(results))
	}
	log, _ := os.ReadFile(r.Log)
	if !strings.Contains(string(log), "started\n") || !strings.HasSuffix(string(log), "hecatron: slow timed out: stopped at its 500ms limit (--timeout)\n") {
		t.Errorf("log:\n%s", log)
	}
	history, _ := os.ReadFile(filepath.Join(dir, "history.log"))
	if !strings.HasSuffix(strings.TrimSpace(string(history)), ":124") {
		t.Errorf("history.log: %q", history)
	}
}
