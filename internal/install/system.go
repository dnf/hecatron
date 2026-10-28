package install

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// System is what the executor does to the machine: run commands and ask
// the package manager. Tests replace it.
type System interface {
	// Succeeds runs a check: output discarded, stdin closed, a minute at most.
	Succeeds(argv []string, dir string, env []string) bool
	// Run runs a command that changes something, its output shown (and
	// stdin the user's, for sudo's password).
	Run(argv []string, dir string, env []string) error
	// Output runs a query and returns its stdout.
	Output(argv []string, env []string) (string, error)
	// IsRoot says whether commands run as root (no sudo needed).
	IsRoot() bool
	// Fetch is a URL's body (downloads, checksums).
	Fetch(url string) (io.ReadCloser, error)
	// AptListsAge is how old apt's package lists are, ok false when there
	// are none (apt_refresh's test in _common.sh).
	AptListsAge() (age time.Duration, ok bool)
}

// OS is the real machine.
type OS struct {
	Stdout, Stderr io.Writer
}

const checkTimeout = time.Minute

// Succeeds implements System.
func (OS) Succeeds(argv []string, dir string, env []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	path, ok := LookPath(argv[0], getenv(env, "PATH"))
	if !ok {
		return false
	}
	cmd := exec.CommandContext(ctx, path, argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	return cmd.Run() == nil
}

// Run implements System.
func (s OS) Run(argv []string, dir string, env []string) error {
	path, ok := LookPath(argv[0], getenv(env, "PATH"))
	if !ok {
		return fmt.Errorf("%s: command not found", argv[0])
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, s.Stdout, s.Stderr
	return cmd.Run()
}

// Output implements System.
func (OS) Output(argv []string, env []string) (string, error) {
	path, ok := LookPath(argv[0], getenv(env, "PATH"))
	if !ok {
		return "", fmt.Errorf("%s: command not found", argv[0])
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.Env = env
	out, err := cmd.Output()
	return string(out), err
}

// IsRoot implements System.
func (OS) IsRoot() bool { return os.Geteuid() == 0 }

// AptListsAge implements System: missing when /var/lib/apt/lists has no
// Release files, else the age of its partial/ directory.
func (OS) AptListsAge() (time.Duration, bool) {
	releases, _ := filepath.Glob("/var/lib/apt/lists/*Release")
	if len(releases) == 0 {
		return 0, false
	}
	info, err := os.Stat("/var/lib/apt/lists/partial")
	if err != nil {
		return 0, true
	}
	return time.Since(info.ModTime()), true
}

// Manager is a system package manager.
type Manager struct {
	Name   string // "apt", "brew"
	system System
	env    []string
}

// ManagerFor is the package manager the executor uses on platform: apt on
// Linux where there is apt-get, Homebrew on macOS; one named "" elsewhere,
// which installs nothing (a definition needing packages then runs its
// script, as it does on any other Linux).
func ManagerFor(platform Platform, system System, env []string) *Manager {
	path := getenv(env, "PATH")
	switch platform.OS {
	case "linux":
		if _, ok := LookPath("apt-get", path); ok {
			return &Manager{"apt", system, env}
		}
	case "macos":
		if _, ok := LookPath("brew", path); ok {
			return &Manager{"brew", system, env}
		}
	}
	return &Manager{"", system, env}
}

// Installed says which of the packages are installed.
func (m *Manager) Installed(packages []string) map[string]bool {
	installed := map[string]bool{}
	if len(packages) == 0 {
		return installed
	}
	switch m.Name {
	case "apt":
		// dpkg-query fails when one isn't known, but still lists the others.
		out, _ := m.system.Output(append([]string{"dpkg-query", "-W", "-f=${Package}\t${Status}\n"}, packages...), m.env)
		for _, line := range strings.Split(out, "\n") {
			name, status, _ := strings.Cut(line, "\t")
			name, _, _ = strings.Cut(name, ":") // multiarch: name:arch
			if status == "install ok installed" || status == "hold ok installed" {
				installed[name] = true
			}
		}
	case "brew":
		for _, name := range packages {
			installed[name] = m.system.Succeeds([]string{"brew", "list", "--versions", name}, "", m.env)
		}
	}
	return installed
}

// Missing are the packages not installed, in order.
func (m *Manager) Missing(packages []string) []string {
	installed := m.Installed(packages)
	var missing []string
	for _, name := range packages {
		if !installed[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// aptListsMaxAge is apt_refresh's: lists more than an hour old are
// refreshed first.
const aptListsMaxAge = time.Hour

// InstallCommands are the commands installing packages: for apt, a refresh
// of old package lists first.
func (m *Manager) InstallCommands(packages []string) [][]string {
	switch m.Name {
	case "apt":
		var commands [][]string
		if age, ok := m.system.AptListsAge(); !ok || age > aptListsMaxAge {
			commands = append(commands, m.sudo("apt-get", "-o", "Acquire::Retries=3", "update", "-qq"))
		}
		return append(commands, m.sudo(append([]string{"apt-get", "-o", "Acquire::Retries=3", "install", "-y"}, packages...)...))
	case "brew":
		return [][]string{append([]string{"env", brewNoAsk, "brew", "install"}, packages...)}
	}
	return nil
}

// brewNoAsk stops Homebrew asking before it installs or removes, as it
// does by default since 2025: the command is the request (apt-get's -y).
const brewNoAsk = "HOMEBREW_NO_ASK=1"

// RemoveCommand is the command removing packages.
func (m *Manager) RemoveCommand(packages []string) []string {
	if m.Name == "brew" {
		return append([]string{"env", brewNoAsk, "brew", "uninstall"}, packages...)
	}
	return m.sudo(append([]string{"apt-get", "remove", "-y"}, packages...)...)
}

// sudo prefixes a command needing root with sudo, unless running as root.
func (m *Manager) sudo(argv ...string) []string {
	if m.system.IsRoot() {
		return argv
	}
	return append([]string{"sudo"}, argv...)
}
