package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dnf/hecatron/internal/selfupdate"
)

const skillUsage = `Usage: hecatron skill install [--user | --project | --dir DIR]
       hecatron skill path

Install hecatron's Agent Skill, the instructions an AI coding agent loads to
use hct, where the agent finds it:
  --user      ~/.claude/skills/hecatron (the default): Claude Code, for you
  --project   .claude/skills/hecatron in this repository, to commit, so
              everyone who opens it in Claude Code gets it
  --dir DIR   DIR/hecatron: another agent that reads Agent Skills (its
              documentation says where it looks)
Installing again replaces the copy, so it matches this hecatron. A folder
there that isn't hecatron's skill is left alone. hct self-update refreshes
the --user and --dir copies; a --project copy is the repository's to update.

hecatron skill path prints where the skill is inside hecatron. In Claude
Code, /plugin install hecatron --marketplace dnf/hecatron installs it as a
plugin instead, updated with each release.
`

// The skill inside hecatron (docs/design.md §19 and plugins/hecatron/).
const skillSource = "plugins/hecatron/skills/hecatron"

// skill is `hecatron skill`. It exists only in the Go core.
func (c *cli) skill() int {
	if len(c.args) == 0 || c.args[0] == "help" || c.args[0] == "-h" || c.args[0] == "--help" {
		fmt.Print(skillUsage)
		return 0
	}
	source := c.homePath(skillSource)
	switch c.args[0] {
	case "path":
		if len(c.args) > 1 {
			return skillUsageError("unexpected argument '" + c.args[1] + "'")
		}
		fmt.Println(source)
		return 0
	case "install":
		parent, err := skillParent(c.args[1:])
		if err != nil {
			return skillUsageError(err.Error())
		}
		destination := filepath.Join(parent, "hecatron")
		if err := installSkill(source, destination); err != nil {
			fmt.Fprintln(os.Stderr, "hecatron: skill:", err)
			return 1
		}
		if len(c.args) < 2 || c.args[1] != "--project" {
			c.recordSkill(destination)
		}
		version, _ := c.readVersion()
		fmt.Printf("Installed hecatron's skill (%s) in %s\n", strings.TrimSpace(string(version)), destination)
		return 0
	}
	return skillUsageError("unknown subcommand '" + c.args[0] + "'")
}

func skillUsageError(message string) int {
	fmt.Fprintf(os.Stderr, "hecatron: skill: %s\n%s", message, skillUsage)
	return 2
}

// skillParent is the skills directory install's flags name.
func skillParent(args []string) (string, error) {
	switch {
	case len(args) == 0 || (len(args) == 1 && args[0] == "--user"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude", "skills"), nil
	case len(args) == 1 && args[0] == "--project":
		top, ok := gitTopLevel()
		if !ok {
			return "", fmt.Errorf("--project needs a git repository")
		}
		return filepath.Join(top, ".claude", "skills"), nil
	case len(args) == 2 && args[0] == "--dir":
		return filepath.Abs(args[1])
	case len(args) == 1 && strings.HasPrefix(args[0], "--dir="):
		return filepath.Abs(strings.TrimPrefix(args[0], "--dir="))
	}
	return "", fmt.Errorf("unexpected arguments '%s'", strings.Join(args, " "))
}

// installSkill replaces destination with a copy of source, unless
// destination holds something other than hecatron's skill.
func installSkill(source, destination string) error {
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); err != nil {
		return fmt.Errorf("no skill in %s", source)
	}
	if _, err := os.Stat(destination); err == nil {
		if !isHecatronSkill(destination) {
			return fmt.Errorf("%s exists and isn't hecatron's skill; move it or install elsewhere (--dir)", destination)
		}
		if err := os.RemoveAll(destination); err != nil {
			return err
		}
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func isHecatronSkill(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	return err == nil && strings.Contains(string(data), "\nname: hecatron\n")
}

// skillRecord lists, in a release installation, where hct skill install
// put the skill (--user, --dir), for hct self-update to refresh.
const skillRecord = ".skill-installs"

// recordSkill adds destination to the record, in a release installation
// only (a checkout updates its skill copies itself). Best effort: a
// record that can't be written only means no refresh.
func (c *cli) recordSkill(destination string) {
	exe, err := executable()
	if err != nil || !selfupdate.IsReleaseBinary(c.home.Dir, exe) {
		return
	}
	for _, recorded := range recordedSkills(c.home.Dir) {
		if recorded == destination {
			return
		}
	}
	file, err := os.OpenFile(filepath.Join(c.home.Dir, skillRecord), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(file, destination)
	_ = file.Close()
}

func recordedSkills(home string) []string {
	data, err := os.ReadFile(filepath.Join(home, skillRecord))
	if err != nil {
		return nil
	}
	var dirs []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			dirs = append(dirs, line)
		}
	}
	return dirs
}

// refreshSkills replaces each copy of the skill hct skill install made
// (the recorded ones and ~/.claude/skills/hecatron) that is still
// hecatron's with the skill in home, and says what it did.
func refreshSkills(home string) []string {
	candidates := recordedSkills(home)
	if user, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(user, ".claude", "skills", "hecatron"))
	}
	var lines []string
	seen := map[string]bool{}
	for _, destination := range candidates {
		if seen[destination] || !isHecatronSkill(destination) {
			continue
		}
		seen[destination] = true
		if err := installSkill(filepath.Join(home, filepath.FromSlash(skillSource)), destination); err != nil {
			lines = append(lines, fmt.Sprintf("Couldn't refresh hecatron's skill in %s: %v (hct skill install --dir %s)", destination, err, filepath.Dir(destination)))
			continue
		}
		lines = append(lines, "Refreshed hecatron's skill in "+destination)
	}
	return lines
}
