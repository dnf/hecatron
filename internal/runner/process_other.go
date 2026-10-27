//go:build !unix

package runner

import "os/exec"

// Supervise makes a command stoppable: off Unix, its context ending kills
// the process itself.
func Supervise(cmd *exec.Cmd) {}
