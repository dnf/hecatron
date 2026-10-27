//go:build unix

package runner

import (
	"os/exec"
	"syscall"
	"time"
)

// Supervise makes a command stoppable as a whole: it runs in a process
// group of its own, which is sent SIGTERM (then, five seconds on, SIGKILL)
// when the command's context ends, so a hook's own children stop with it.
func Supervise(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
}
