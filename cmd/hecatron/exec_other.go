//go:build !unix

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

// replaceProcess runs a program and exits with its status: Windows can't
// replace a running process. Ctrl-C reaches the program from the console
// itself; this process waits for it to finish rather than dying first.
func replaceProcess(program string, argv, env []string) int {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
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

// detach starts cmd on its own; on Windows a child outlives its parent
// already.
func detach(cmd *exec.Cmd) {}
