//go:build unix

package runner

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// StopGrace is how long a stopped job's processes have to exit after
// SIGTERM before they are sent SIGKILL.
var StopGrace = 5 * time.Second

// jobTokens is the variable that marks a job's processes: a token per
// supervised command, appended to the one it inherited, so a process that
// leaves the process group (setsid, a daemon) is still found on Linux.
const jobTokens = "HECATRON_JOB_TOKENS"

// Supervise makes a command stoppable as a whole: it runs in a process
// group of its own, and when the command's context ends (a time limit,
// Ctrl-C) the group, and on Linux every process carrying the command's
// token, is sent SIGTERM, then SIGKILL after StopGrace if any is left. The
// returned wait, called after the command's Wait, returns once they are
// gone, so nothing outlives the job.
func Supervise(cmd *exec.Cmd) (wait func()) {
	token := newToken()
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	inherited := getenv(env, jobTokens)
	if inherited != "" {
		token = inherited + "," + token
	}
	cmd.Env = append(append([]string(nil), env...), jobTokens+"="+token)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var once sync.Once
	stopped := make(chan struct{})
	cancelled := false
	var mu sync.Mutex
	own := token[strings.LastIndexByte(token, ',')+1:]
	cmd.Cancel = func() error {
		mu.Lock()
		cancelled = true
		mu.Unlock()
		pgid := cmd.Process.Pid
		signalAll(pgid, own, syscall.SIGTERM)
		go once.Do(func() {
			defer close(stopped)
			for deadline := time.Now().Add(StopGrace); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
				if !anyLeft(pgid, own) {
					return
				}
			}
			signalAll(pgid, own, syscall.SIGKILL)
			for i := 0; i < 50 && anyLeft(pgid, own); i++ {
				time.Sleep(20 * time.Millisecond)
			}
		})
		return nil
	}
	cmd.WaitDelay = StopGrace + time.Second
	return func() {
		mu.Lock()
		wasCancelled := cancelled
		mu.Unlock()
		if wasCancelled {
			<-stopped
		}
	}
}

func newToken() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getenv(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(env[i], name+"="); ok {
			return value
		}
	}
	return ""
}

func signalAll(pgid int, token string, sig syscall.Signal) {
	syscall.Kill(-pgid, sig)
	for _, pid := range marked(token) {
		syscall.Kill(pid, sig)
	}
}

// anyLeft reports whether the group or a marked process still exists.
func anyLeft(pgid int, token string) bool {
	if syscall.Kill(-pgid, 0) == nil {
		return true
	}
	return len(marked(token)) > 0
}

// marked are the processes whose environment carries the token, found in
// /proc (Linux; elsewhere none, so only the process group is stopped).
// Zombies are left out: they have exited and only wait to be reaped.
func marked(token string) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		environ, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "environ"))
		if err != nil || !carries(environ, token) || zombie(entry.Name()) {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

func carries(environ []byte, token string) bool {
	for _, entry := range bytes.Split(environ, []byte{0}) {
		if value, ok := bytes.CutPrefix(entry, []byte(jobTokens+"=")); ok {
			for _, t := range bytes.Split(value, []byte(",")) {
				if string(t) == token {
					return true
				}
			}
		}
	}
	return false
}

func zombie(pid string) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", pid, "stat"))
	if err != nil {
		return true
	}
	end := bytes.LastIndexByte(stat, ')')
	return end < 0 || end+2 >= len(stat) || stat[end+2] == 'Z'
}
