package selfupdate

import (
	"fmt"
	"strings"
)

// ChecksAllowed is whether hecatron may look for a newer release in the
// background: never in CI (CI set) or with HECATRON_NO_UPDATE_CHECK=1.
func ChecksAllowed(getenv func(string) string) bool {
	return getenv("CI") == "" && getenv("HECATRON_NO_UPDATE_CHECK") != "1"
}

// quietCommands print what's read by programs or replace hecatron, so no
// notice follows them.
var quietCommands = map[string]bool{
	"self-update": true,
	"env":         true,
	"banner":      true,
	"plan":        true,
}

// NoticeAllowed is whether the update notice may be shown after command:
// checks allowed, stderr a terminal, and human output (no --output or
// HECATRON_OUTPUT naming another format, not an AI agent's session, not
// GitHub Actions).
func NoticeAllowed(getenv func(string) string, command string, args []string, stdoutTerminal, stderrTerminal bool) bool {
	if !ChecksAllowed(getenv) || !stderrTerminal || quietCommands[command] || strings.HasPrefix(command, "_") {
		return false
	}
	format := getenv("HECATRON_OUTPUT")
flags:
	for i, arg := range args {
		switch {
		case arg == "--":
			break flags
		case arg == "--output" && i+1 < len(args):
			format = args[i+1]
		case strings.HasPrefix(arg, "--output="):
			format = strings.TrimPrefix(arg, "--output=")
		}
	}
	if format != "" {
		return format == "human"
	}
	agent := getenv("CLAUDECODE") == "1" && !stdoutTerminal
	return !agent && getenv("GITHUB_ACTIONS") != "true"
}

// Notice is the line saying a newer release exists.
func Notice(current, latest string) string {
	return fmt.Sprintf("A new release of hecatron is available: %s → %s. Run `hct self-update`.", current, latest)
}
