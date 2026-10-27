// Command hecatron (also hct) is the Go core (docs/design.md §19), growing
// in place of bin/hecatron one command at a time. It finds HECATRON_HOME and
// the project root as bin/hecatron does and sets the same environment; the
// commands ported so far run here, and every other command runs the
// existing implementation (libexec/commands/*.sh, manage.py...) with the
// same arguments, environment and exit status, so nothing changes for the
// user but the speed.
//
// Ported: --version, help, each run command's, doctor's and version's
// help, list, and env without a language. The prototype's
// `hecatron plan <lang> <action> [dir]` is kept, unlisted.
package main

import (
	"fmt"
	"os"

	"github.com/dnf/hecatron/internal/project"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// cli is one invocation: the command, its arguments and hecatron's home.
type cli struct {
	home    project.Home
	command string
	args    []string

	root    string           // the project root, once found
	project *project.Project // nil when the Go model can't read the project

	pythonChecked bool
}

func run(argv []string) int {
	c := &cli{home: project.NewHome(findHome()), command: "help"}
	if len(argv) > 0 {
		c.command, c.args = argv[0], argv[1:]
	}
	switch c.command {
	case "help", "-h", "--help":
		return c.usage()
	case "--version", "-V", "version-of-hecatron":
		return c.version()
	case "plan":
		return plan(c.home, c.args)
	case "self-test":
		return c.delegate(bashScript("libexec/commands/self-test.sh"))
	}
	// Every command but init and banner runs inside a project; find it
	// (like git).
	if c.command != "init" && c.command != "banner" {
		if status := c.findProject(); status != 0 {
			return status
		}
	}
	if handler, ok := nativeCommands[c.command]; ok && c.project != nil {
		if status, handled := handler(c); handled {
			return status
		}
	}
	implementation, known := delegated[c.command]
	if !known {
		if status := c.requirePython(); status != 0 {
			return status
		}
		fmt.Fprintf(os.Stderr, "hecatron: unknown command '%s' (try 'hecatron help')\n", c.command)
		return 2
	}
	return c.delegate(implementation)
}

// nativeCommands run in Go. A handler reports handled=false for what it
// doesn't cover yet (an argument form, a project the model declines), and
// the command is delegated instead, before anything is printed.
var nativeCommands = map[string]func(*cli) (status int, handled bool){
	"list":    (*cli).list,
	"env":     (*cli).env,
	"test":    (*cli).runHelp,
	"lint":    (*cli).runHelp,
	"build":   (*cli).runHelp,
	"clean":   (*cli).runHelp,
	"rebuild": (*cli).runHelp,
	"retest":  (*cli).runHelp,
	"audit":   (*cli).runHelp,
	"doctor":  (*cli).doctorHelp,
	"version": (*cli).versionHelp,
}

// findProject sets the project root and, when the Go model reads it, the
// project. A project only project.py reads is found by project.py, which
// also reports why one can't be read, as bin/hecatron does.
func (c *cli) findProject() int {
	cwd, err := workingDir()
	if err == nil {
		c.root, err = project.FindRoot(cwd)
	}
	if err == nil {
		c.project, err = project.Load(c.root, c.home)
	}
	if err == nil {
		return 0
	}
	if e, ok := err.(*project.Error); ok {
		fmt.Fprintln(os.Stderr, "hecatron:", e.Message)
		return 1
	}
	c.project = nil
	return c.rootFromPython()
}
