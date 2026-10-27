//go:build !unix

package runner

import "os/exec"

// Supervise makes a command stoppable: off Unix, its context ending kills
// the process itself, not the processes it started (a job object would;
// not done yet: docs/runner.md).
func Supervise(cmd *exec.Cmd) (wait func()) { return func() {} }
