package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dnf/hecatron/internal/install"
	"github.com/dnf/hecatron/internal/runner"
	"github.com/dnf/hecatron/internal/selfupdate"
)

const selfUninstallUsage = `Usage: hecatron self uninstall [--all] [--yes]

Remove hecatron: the installation install.sh made (~/.hecatron), its line
in your shell's startup files and fish's conf.d, and the Agent Skill hct
skill install put in ~/.claude/skills or another --dir. Run from a
checkout, it keeps the checkout and removes only the rest.

The toolchains hecatron installed stay, as your projects use them: run
'hct uninstall <language>' in a project first to remove one, which knows
what each installed.

Flags:
  --all    Also remove hecatron's data (~/.local/share/hecatron: the
           toolchains it unpacked, its install receipts and links) and its
           download cache (~/.cache/hecatron). Packages installed through
           Homebrew or apt stay.
  --yes    Don't ask first (needed when not on a terminal)
`

// selfUninstall is `hecatron self uninstall`. It exists only in the Go core.
func (c *cli) selfUninstall() int {
	all, yes := false, false
	for _, arg := range c.args {
		switch {
		case isHelp(arg) || arg == "-h":
			fmt.Print(selfUninstallUsage)
			return 0
		case arg == "--all":
			all = true
		case arg == "--yes" || arg == "-y":
			yes = true
		default:
			fmt.Fprintf(os.Stderr, "hecatron: self uninstall: unexpected argument '%s'\n%s", arg, selfUninstallUsage)
			return 2
		}
	}
	exe, err := executable()
	if err == nil {
		var user string
		if user, err = os.UserHomeDir(); err == nil {
			return c.uninstallFrom(exe, user, all, yes)
		}
	}
	fmt.Fprintln(os.Stderr, "hecatron: self uninstall:", err)
	return 1
}

func (c *cli) uninstallFrom(exe, user string, all, yes bool) int {
	plan, err := selfupdate.PlanUninstall(c.home.Dir, exe, user)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: self uninstall:", err)
		return 1
	}
	skills := installedSkills(c.home.Dir, user)
	env := os.Environ()
	data := []string{filepath.Dir(install.ReceiptsDir(env)), filepath.Dir(install.DownloadsDir(env))}

	fmt.Println("This removes:")
	if plan.Home != "" {
		fmt.Printf("  %s (hecatron %s)\n", plan.Home, plan.Version)
	}
	for _, path := range plan.Startup {
		fmt.Printf("  hecatron's line in %s\n", path)
	}
	for _, path := range append(optional(plan.Fish), skills...) {
		fmt.Printf("  %s\n", path)
	}
	if all {
		for _, dir := range existing(data) {
			fmt.Printf("  %s\n", dir)
		}
	}
	if plan.Checkout != "" {
		fmt.Printf("It keeps the checkout %s.\n", plan.Checkout)
	}
	if left := existing(data); !all && len(left) > 0 {
		fmt.Printf("It keeps the toolchains hecatron installed, and %s (--all removes it).\n", strings.Join(left, " and "))
	}
	if !yes && !confirm("Remove hecatron?") {
		fmt.Fprintln(os.Stderr, "hecatron: self uninstall: nothing removed (--yes to remove without asking)")
		return 1
	}

	if err := plan.Remove(); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: self uninstall:", err)
		return 1
	}
	removals := skills
	if all {
		removals = append(removals, data...)
	}
	for _, path := range removals {
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintln(os.Stderr, "hecatron: self uninstall:", err)
			return 1
		}
	}
	fmt.Println("Removed hecatron. Open a new shell to drop it from PATH.")
	return 0
}

// installedSkills are the copies of hecatron's skill hct skill install
// made: the recorded ones and ~/.claude/skills/hecatron.
func installedSkills(home, user string) []string {
	var skills []string
	seen := map[string]bool{}
	for _, dir := range append(recordedSkills(home), filepath.Join(user, ".claude", "skills", "hecatron")) {
		if !seen[dir] && isHecatronSkill(dir) {
			seen[dir] = true
			skills = append(skills, dir)
		}
	}
	return skills
}

// confirm asks a yes-or-no question on a terminal; elsewhere it's no.
func confirm(question string) bool {
	if !runner.IsTerminal(os.Stdin) {
		return false
	}
	fmt.Printf("%s [y/N] ", question)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func optional(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}

func existing(paths []string) []string {
	var found []string
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		}
	}
	return found
}
