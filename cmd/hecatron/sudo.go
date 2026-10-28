package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/install"
	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
)

// needRoot are the languages among choices whose install may need root: a
// script (which may run apt-get, or write to /usr/local or /opt), and from
// data, every one when the plan runs a command with sudo.
func needRoot(p *project.Project, choices []installerChoice, env []string, manager *install.Manager) []string {
	var langs, data []string
	var dataChoices []installerChoice
	for _, choice := range choices {
		if choice.def == nil {
			langs = append(langs, choice.lang)
		} else {
			data = append(data, choice.lang)
			dataChoices = append(dataChoices, choice)
		}
	}
	if len(dataChoices) > 0 {
		plan, _ := planData(p, dataChoices, env, manager)
		for _, command := range plan.Commands() {
			if strings.HasPrefix(command, "sudo ") {
				langs = append(data, langs...)
				break
			}
		}
	}
	return langs
}

// sudoKeepAlive is how often the credentials are refreshed: well inside
// sudo's default 5-minute timeout.
const sudoKeepAlive = time.Minute

// adminRights asks for the password sudo needs once, before installers
// that may need root run, saying which and why, so no prompt turns up
// halfway through; and keeps sudo's credentials fresh until stop is
// called, so a long install doesn't ask again. Nothing to ask as root,
// without sudo, with sudo needing no password (CI) or off a terminal
// (where the installers fare as before). false: the user declined.
func adminRights(langs []string) (stop func(), ok bool) {
	stop = func() {}
	if len(langs) == 0 || runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return stop, true
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return stop, true
	}
	if exec.Command("sudo", "-n", "true").Run() != nil {
		if !runner.IsTerminal(os.Stdin) {
			return stop, true
		}
		fmt.Fprintf(os.Stderr, "hecatron: installing %s may need administrator rights:\n", sentenceList(langs))
		fmt.Fprintln(os.Stderr, "  installers add system packages and put tools in /usr/local and /opt, which belong to root.")
		fmt.Fprintln(os.Stderr, "  sudo asks for your password once, now, and hecatron keeps it valid until they finish.")
		sudo := exec.Command("sudo", "-v")
		sudo.Stdin, sudo.Stdout, sudo.Stderr = os.Stdin, os.Stdout, os.Stderr
		if sudo.Run() != nil {
			return stop, false
		}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(sudoKeepAlive)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = exec.Command("sudo", "-n", "-v").Run()
			}
		}
	}()
	return func() { close(done) }, true
}

// sentenceList is "a", "a and b", "a, b and c".
func sentenceList(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
