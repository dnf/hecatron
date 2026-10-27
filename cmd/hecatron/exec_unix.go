//go:build unix

package main

import (
	"fmt"
	"os"
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
