package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/selfupdate"
)

// pinWarned is set in the environment once a command has warned that the
// running hecatron isn't the project's pinned one, so the hct commands it
// runs (a task, a hook) don't warn again.
const pinWarned = "HECATRON_PIN_WARNED"

// pinMismatch is what's wrong when the running hecatron isn't the one the
// project pins (config.json's "hecatron": "0.6"), "" when it is or there's
// no pin.
func (c *cli) pinMismatch() string {
	if c.project == nil {
		return ""
	}
	pin, _ := c.project.HecatronPin()
	version := c.home.Version()
	if pin == "" || project.PinMatches(pin, version) {
		return ""
	}
	release := pin
	if !project.ExactPin(pin) {
		release += strings.Repeat(".N", 2-strings.Count(pin, "."))
	}
	return fmt.Sprintf("this project pins hecatron %s, but this is %s (hct self update --version %s installs one that matches)",
		pin, version, release)
}

// pinExempt are the commands that don't warn: hct check reports the
// mismatch itself, and version sets the project's own version.
var pinExempt = map[string]bool{"check": true, "version": true}

// warnPin says on stderr, once, that the running hecatron isn't the one
// the project pins.
func (c *cli) warnPin() {
	if pinExempt[c.command] || strings.HasPrefix(c.command, "_") || os.Getenv(pinWarned) == "1" {
		return
	}
	if mismatch := c.pinMismatch(); mismatch != "" {
		fmt.Fprintln(os.Stderr, "hecatron: warning:", mismatch)
		_ = os.Setenv(pinWarned, "1")
	}
}

// pinProblems are hct check's: a "hecatron" setting that's neither a pin
// nor a minimum, and a running hecatron the pin doesn't take. A dev build
// (hct self build's, whose VERSION says nothing of the checkout's commit)
// only warns.
func (c *cli) pinProblems() []string {
	_, problem := c.project.HecatronPin()
	if problem != "" {
		return []string{problem}
	}
	mismatch := c.pinMismatch()
	if mismatch == "" {
		return nil
	}
	if selfupdate.IsDevBuild(c.home.Dir) {
		fmt.Fprintln(os.Stderr, "hecatron: warning:", mismatch+"; a dev build, so hct check passes")
		return nil
	}
	return []string{mismatch}
}
