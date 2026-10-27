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
	"os/exec"
	"strings"

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
	implementation, known := delegated[c.command]
	if !known {
		if status := c.requirePython(); status != 0 {
			return status
		}
		fmt.Fprintf(os.Stderr, "hecatron: unknown command '%s' (try 'hecatron help')\n", c.command)
		return 2
	}
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
	return c.delegate(implementation)
}

// nativeCommands run in Go. A handler reports handled=false for what it
// doesn't cover yet (an argument form, a project the model declines), and
// the command is delegated instead, before anything is printed.
var nativeCommands = map[string]func(*cli) (status int, handled bool){
	"list":     (*cli).list,
	"env":      (*cli).env,
	"test":     (*cli).runHelp,
	"lint":     (*cli).runHelp,
	"build":    (*cli).runHelp,
	"clean":    (*cli).runHelp,
	"rebuild":  (*cli).runHelp,
	"retest":   (*cli).runHelp,
	"audit":    (*cli).runHelp,
	"format":   (*cli).runHelp,
	"deps":     (*cli).runHelp,
	"outdated": (*cli).runHelp,
	"doctor":   (*cli).doctorHelp,
	"version":  (*cli).versionHelp,
}

// findProject finds the project (like git), setting it up first in a git
// repository that hecatron hasn't set up yet, as `hecatron init` would at
// the repository's top level; init's report goes to stderr, so the
// command's own output stays as it is.
func (c *cli) findProject() int {
	if c.locateProject(false) {
		return 0
	}
	if top, ok := gitTopLevel(); ok {
		if status := c.requirePython(); status != 0 {
			return status
		}
		cmd := exec.Command("python3", c.homePath("libexec/commands/manage.py"), "init", "--auto", top)
		cmd.Env = c.environment(nil)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if cmd.Run() != nil {
			return 1
		}
	}
	if c.locateProject(true) {
		return 0
	}
	return 1
}

// locateProject sets the project root and, when the Go model reads it, the
// project. A project only project.py reads is found by project.py. With
// report, why there is none is said, in project.py's words.
func (c *cli) locateProject(report bool) bool {
	c.root, c.project = "", nil
	cwd, err := workingDir()
	var root string
	if err == nil {
		root, err = project.FindRoot(cwd)
	}
	var p *project.Project
	if err == nil {
		p, err = project.Load(root, c.home)
	}
	if err == nil {
		c.root, c.project = root, p
		return true
	}
	if e, ok := err.(*project.Error); ok {
		if report {
			fmt.Fprintln(os.Stderr, "hecatron:", e.Message)
		}
		return false
	}
	return c.rootFromPython(report)
}

// gitTopLevel is the top of the git repository around the working
// directory, if there is one.
func gitTopLevel() (string, bool) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(out), "\n"), true
}
