//go:build !unix

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// replaceProcess runs a program and exits with its status: Windows can't
// replace a running process.
func replaceProcess(program string, argv, env []string) int {
	cmd := exec.Command(program, argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = env
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 126
	}
	return 0
}
