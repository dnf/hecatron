package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/runner"
	"github.com/dnf/hecatron/internal/status"
)

const statusUsage = `Usage: hecatron status [--output human|agent|json]

What to know first in a repository: hecatron's version (and a newer
release when the last daily check found one; hct self update), the project (its
config saved or only detected), and for each language its directories, the
toolchain version it resolves to and why, whether that toolchain is
installed, the last run of each action, and its uncommitted changes; then
the commands worth running next. Runs one git status and nothing that
installs. The output format is chosen as for hct test: --output, else
HECATRON_OUTPUT, else agent output for an AI agent, else human.
`

// status is `hecatron status` (#121). It exists only in the Go core.
func (c *cli) status() int {
	flag := ""
	for i := 0; i < len(c.args); i++ {
		switch arg := c.args[i]; {
		case arg == "help" || arg == "-h" || arg == "--help":
			fmt.Print(statusUsage)
			return 0
		case arg == "--output" && i+1 < len(c.args):
			i++
			flag = c.args[i]
		case strings.HasPrefix(arg, "--output="):
			flag = strings.TrimPrefix(arg, "--output=")
		default:
			fmt.Fprintf(os.Stderr, "hecatron: status: unexpected argument '%s'\n%s", arg, statusUsage)
			return 2
		}
	}
	if flag == "" && len(c.args) > 0 && c.args[len(c.args)-1] == "--output" {
		fmt.Fprintln(os.Stderr, "hecatron: --output needs a format: human, agent or json")
		return 2
	}
	format, err := runner.SelectFormat(flag, os.Getenv, runner.IsTerminal(os.Stdout))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 2
	}
	if status := c.findProject(); status != 0 {
		return status
	}
	if c.project == nil {
		fmt.Fprintln(os.Stderr, "hecatron: status can't read this project's configuration yet; hct list and hct doctor can")
		return 1
	}
	s, err := status.Gather(c.project, c.environment(nil), time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	s.SetUpdate(c.knownUpdate())
	if err := status.Write(os.Stdout, s, string(format)); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	return 0
}
