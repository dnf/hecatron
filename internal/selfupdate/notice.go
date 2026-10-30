package selfupdate

import (
	"fmt"
	"strings"

	"github.com/dnf/hecatron/internal/runner"
)

// ChecksAllowed is whether hecatron may look for a newer release in the
// background: never in CI (CI set) or with HECATRON_NO_UPDATE_CHECK=1.
func ChecksAllowed(getenv func(string) string) bool {
	return getenv("CI") == "" && getenv("HECATRON_NO_UPDATE_CHECK") != "1"
}

// quietCommands print what's read by programs or replace hecatron, so no
// notice follows them.
var quietCommands = map[string]bool{
	"env":    true,
	"banner": true,
	"plan":   true,
}

// NoticeAllowed is whether the update notice may be shown after command:
// checks allowed, and a person reading stderr (runner.ForAPerson).
func NoticeAllowed(getenv func(string) string, command string, args []string, stdoutTerminal, stderrTerminal bool) bool {
	selfUpdate := command == "self" && len(args) > 0 && args[0] == "update"
	if !ChecksAllowed(getenv) || quietCommands[command] || selfUpdate || strings.HasPrefix(command, "_") {
		return false
	}
	return runner.ForAPerson(getenv, args, stdoutTerminal, stderrTerminal)
}

// Notice is the line saying a newer release exists.
func Notice(current, latest string) string {
	return fmt.Sprintf("A new release of hecatron is available: %s → %s. Run `hct self update`.", current, latest)
}
