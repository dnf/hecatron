//go:build unix

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// replaceProcess runs a program in place of this one, as bin/hecatron's
// exec does: same process, so its exit status, signals and terminal are the
// user's directly.
func replaceProcess(program string, argv, env []string) int {
	err := syscall.Exec(program, argv, env)
	fmt.Fprintln(os.Stderr, "hecatron:", err)
	return 126
}

// detach starts cmd in a session of its own: it outlives this process,
// and the terminal's Ctrl-C doesn't reach it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
