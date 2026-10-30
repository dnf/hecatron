// Command hecatron (also hct) is the Go core (docs/contributing/design.md §19), growing
// in place of bin/hecatron one command at a time. It finds HECATRON_HOME and
// the project root as bin/hecatron does and sets the same environment; the
// commands ported so far run here, and every other command runs the
// existing implementation (libexec/commands/*.sh, manage.py...) with the
// same arguments, environment and exit status, so nothing changes for the
// user but the speed.
//
// Ported: --version, help, the run commands (test, lint, build, clean,
// rebuild, retest, audit, format, deps, outdated, docs, bench: run.go), doctor
// (doctor.go; each language's doctor.sh stays bash), version (version.go; each
// language's version.sh stays bash), list,
// env (with a language, its environment for a component), status, sync, advisor, the
// project commands (init, add, remove, enable, disable, check, dockerfile,
// workflows: manage.go, init.go), banner (banner.go), and install and uninstall
// (install.go; each language's install.sh and uninstall.sh stay bash),
// and self update (selfupdate.go), which only the binary has. The
// prototype's `hecatron plan <lang> <action> [dir]` is kept, unlisted.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dnf/hecatron/internal/install"
	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
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
	ci            string // workflows --ci: the CI system to generate for
	notice        string // a newer release's notice, shown when the command is done
}

func run(argv []string) int {
	linksOnPath()
	c := &cli{home: project.NewHome(findHome()), command: "help"}
	if len(argv) > 0 {
		c.command, c.args = argv[0], argv[1:]
	}
	c.prepareUpdateNotice()
	status := c.dispatch()
	c.showUpdateNotice()
	return status
}

// linksOnPath puts hecatron's bin directory (install.LinksDir), where the
// installers put the tools they install without root, first on PATH, as
// install.sh's env and bin/hecatron do: what hecatron runs, and
// finds, includes them where no shell sourced the env (a git hook a GUI
// runs, CI).
func linksOnPath() {
	links, path := install.LinksDir(os.Environ()), os.Getenv("PATH")
	if !slices.Contains(filepath.SplitList(path), links) {
		_ = os.Setenv("PATH", links+string(os.PathListSeparator)+path)
	}
}

func (c *cli) dispatch() int {
	switch c.command {
	case "help", "-h", "--help":
		if len(c.args) > 0 && c.args[0] == "agents" {
			return c.agentsHelp()
		}
		return c.usage()
	case "--version", "-V", "version-of-hecatron":
		return c.version()
	case "plan":
		return plan(c.home, c.args)
	case "status":
		return c.status()
	case "sync":
		return c.sync()
	case "licenses":
		return c.licenses()
	case "sbom":
		return c.sbomCommand()
	case "exec":
		return c.execCommand()
	case "skill":
		return c.skill()
	case "self":
		return c.self()
	case "_update-check":
		return c.updateCheck()
	case "_write-env":
		return writeEnv(c.args)
	case "advisor":
		return c.advisor()
	case "_audit":
		return c.auditTool()
	case "_outdated":
		return c.outdatedTool()
	case "_install":
		return c.installHidden()
	case "_gotest":
		return c.goTestFilter()
	case "self-test": // self test's earlier name
		return c.delegate(bashScript("libexec/commands/self-test.sh"))
	case "self-build": // self build's earlier name
		return c.delegate(bashScript("libexec/commands/self-build.sh"))
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
	if handler, ok := nativeCommands[c.command]; ok && (c.project != nil || c.command == "init" || c.command == "banner") {
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
	"list":       (*cli).list,
	"env":        (*cli).env,
	"test":       (*cli).runCommand,
	"lint":       (*cli).runCommand,
	"build":      (*cli).runCommand,
	"clean":      (*cli).runCommand,
	"rebuild":    (*cli).runCommand,
	"retest":     (*cli).runCommand,
	"audit":      (*cli).runCommand,
	"format":     (*cli).runCommand,
	"deps":       (*cli).runCommand,
	"outdated":   (*cli).runCommand,
	"docs":       (*cli).runCommand,
	"bench":      (*cli).runCommand,
	"doctor":     (*cli).doctor,
	"init":       (*cli).init,
	"add":        (*cli).add,
	"remove":     (*cli).remove,
	"enable":     (*cli).enable,
	"disable":    (*cli).disable,
	"check":      (*cli).check,
	"dockerfile": (*cli).dockerfile,
	"workflows":  (*cli).workflows,
	"version":    (*cli).versionCommand,
	"banner":     (*cli).banner,
	"install":    (*cli).install,
	"uninstall":  (*cli).uninstall,
}

// findProject finds the project (like git). A git repository with no
// config of its own is read as detected, which only project.py does: the
// root then comes from it, and the Go model reads what it detected from
// its cache (or the command is delegated). A hecatron the project doesn't
// pin is warned about.
func (c *cli) findProject() int {
	if !c.locateProject(false) && !c.detectProject() {
		if !c.rootFromPython(true) {
			return 1
		}
		if p, err := project.LoadDetected(c.root, c.home); err == nil {
			c.project = p
		}
	}
	c.warnPin()
	return 0
}

// detectProject reads a git repository without a config anywhere above the
// working directory as detected, in Go: its root is the repository's top,
// as project.py's find_root has it, and the first time it's detected the
// model says so on stderr, as project.py does.
func (c *cli) detectProject() bool {
	cwd, err := workingDir()
	if err != nil {
		return false
	}
	if _, err := project.FindRoot(cwd); err == nil {
		return false // a config that didn't load: project.py says why
	}
	top, ok := gitTopLevel()
	if !ok {
		return false
	}
	p, err := project.LoadDetected(top, c.home)
	if err != nil {
		return false
	}
	if p.Announcement != "" && runner.ForAPerson(os.Getenv, c.args, runner.IsTerminal(os.Stdout), runner.IsTerminal(os.Stderr)) {
		fmt.Fprintln(os.Stderr, p.Announcement)
	}
	c.root, c.project = top, p
	return true
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
