package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/runner"
	"github.com/dnf/hecatron/internal/selfupdate"
)

const selfUpdateUsage = `Usage: hecatron self update [--check] [--version X.Y.Z]

Update hecatron to the latest release: its binary for this machine,
checked against the release's SHA256SUMS, and its files, prepared beside
the installation and swapped in, so a failure leaves the old version
working. The Agent Skill hct skill install put in ~/.claude/skills or
another --dir is refreshed too. Only for an installation install.sh made;
a git checkout updates with git pull (and hct self build).

Hecatron never updates itself: it generates CI workflows and a Dockerfile,
which a new version can change. Once a day at most it looks for a newer
release in the background and says so after a command (never in CI, in
agent or JSON output, or with HECATRON_NO_UPDATE_CHECK=1).

Flags:
  --check            Only say whether a newer release exists. Exit status:
                     0 up to date, 3 a newer release exists, 1 the check failed
  --version X.Y.Z    Install that release instead (older ones too)

GITHUB_TOKEN, when set, is sent to GitHub's API (a higher rate limit).
`

// The exit status of self update --check when a newer release exists.
const statusUpdateAvailable = 3

// selfUpdate is `hecatron self update`. It exists only in the Go core.
func (c *cli) selfUpdate() int {
	check, version := false, ""
	for i := 0; i < len(c.args); i++ {
		switch arg := c.args[i]; {
		case isHelp(arg) || arg == "-h":
			fmt.Print(selfUpdateUsage)
			return 0
		case arg == "--check":
			check = true
		case arg == "--version" && i+1 < len(c.args):
			i++
			version = c.args[i]
		case strings.HasPrefix(arg, "--version="):
			version = strings.TrimPrefix(arg, "--version=")
		default:
			return selfUpdateUsageError("unexpected argument '" + arg + "'")
		}
	}
	version = strings.TrimPrefix(version, "v")
	switch {
	case len(c.args) > 0 && c.args[len(c.args)-1] == "--version":
		return selfUpdateUsageError("--version needs a version (X.Y.Z)")
	case version != "" && !selfupdate.ValidVersion(version):
		return selfUpdateUsageError("--version takes X.Y.Z (got '" + version + "')")
	case check && version != "":
		return selfUpdateUsageError("--check looks for the latest release; it takes no --version")
	}
	current := c.home.Version()
	exe, err := executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: self update:", err)
		return 1
	}
	install, refusal := selfupdate.Inspect(c.home.Dir, exe)
	client := selfupdate.NewClient(os.Getenv, current, &http.Client{Timeout: 5 * time.Minute})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if check {
		return c.checkForUpdate(ctx, client, current, refusal)
	}
	if refusal != nil {
		fmt.Fprintln(os.Stderr, "hecatron: self update:", refusal)
		return 1
	}
	var release *selfupdate.Release
	if version != "" {
		release, err = client.Get(ctx, version)
	} else {
		release, err = client.Latest(ctx)
	}
	if selfupdate.IsNoRelease(err) && version != "" {
		fmt.Fprintf(os.Stderr, "hecatron: self update: there is no release %s\n", version)
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: self update: finding the release:", err)
		return 1
	}
	if version == "" {
		_ = selfupdate.WriteCache(install.Home, selfupdate.Cache{Checked: time.Now().UTC(), Latest: release.Version})
	}
	if release.Version == install.Version || (version == "" && !selfupdate.Newer(release.Version, install.Version)) {
		fmt.Printf("hecatron %s is already up to date\n", install.Version)
		return 0
	}
	fmt.Printf("Updating hecatron %s → %s...\n", install.Version, release.Version)
	binary, err := client.Binary(ctx, release)
	if err == nil {
		err = selfupdate.Replace(install, release, binary)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hecatron: self update: %v\nhecatron %s is unchanged.\n", err, install.Version)
		return 1
	}
	fmt.Printf("Updated hecatron %s → %s\n", install.Version, release.Version)
	for _, line := range refreshSkills(install.Home) {
		fmt.Println(line)
	}
	fmt.Println("Generated files can change with a release: run hct check in your projects.")
	return 0
}

// checkForUpdate is self update --check: the latest release against this
// one, and (when it can't update itself) how this installation updates.
func (c *cli) checkForUpdate(ctx context.Context, client *selfupdate.Client, current string, refusal error) int {
	release, err := client.Latest(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: self update: finding the latest release:", err)
		return 1
	}
	if refusal == nil {
		_ = selfupdate.WriteCache(c.home.Dir, selfupdate.Cache{Checked: time.Now().UTC(), Latest: release.Version})
	}
	status := 0
	if selfupdate.Newer(release.Version, current) {
		status = statusUpdateAvailable
		if refusal == nil {
			fmt.Printf("hecatron %s is available (this is %s): run hct self update\n", release.Version, current)
		} else {
			fmt.Printf("hecatron %s is available (this is %s)\n", release.Version, current)
		}
	} else {
		fmt.Printf("hecatron %s is up to date (the latest release is %s)\n", current, release.Version)
	}
	if refusal != nil {
		fmt.Printf("hct self update can't update this one: %v\n", refusal)
	}
	return status
}

func selfUpdateUsageError(message string) int {
	fmt.Fprintf(os.Stderr, "hecatron: self update: %s\n%s", message, selfUpdateUsage)
	return 2
}

func isSelfUpdate(command string, args []string) bool {
	return command == "self" && len(args) > 0 && args[0] == "update"
}

// executable is this binary's path, links followed.
func executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// prepareUpdateNotice runs before every command of a release binary: from
// the last check's cached result, the notice to show when the command is
// done (when a notice may be shown at all), and once a day a new check, in
// a process of its own, so no command waits on the network.
func (c *cli) prepareUpdateNotice() {
	if !selfupdate.ChecksAllowed(os.Getenv) || isSelfUpdate(c.command, c.args) || strings.HasPrefix(c.command, "_") {
		return
	}
	exe, err := executable()
	if err != nil || !selfupdate.IsReleaseBinary(c.home.Dir, exe) {
		return
	}
	cache := selfupdate.ReadCache(c.home.Dir)
	now := time.Now().UTC()
	if cache.Due(now) {
		startUpdateCheck(c.home.Dir, exe, cache, now)
	}
	current := c.home.Version()
	if latest := cache.Newer(current); latest != "" &&
		selfupdate.NoticeAllowed(os.Getenv, c.command, c.args, runner.IsTerminal(os.Stdout), runner.IsTerminal(os.Stderr)) {
		c.notice = selfupdate.Notice(current, latest)
	}
}

// showUpdateNotice prints the notice, once: after a native command, or
// before a delegated one replaces this process.
func (c *cli) showUpdateNotice() {
	if c.notice != "" {
		fmt.Fprintln(os.Stderr, c.notice)
		c.notice = ""
	}
}

// startUpdateCheck starts `hct _update-check` on its own and doesn't wait
// for it. The check's time is written first, so the commands that follow
// don't start another while it runs, and a failed check isn't retried for
// a day; a cache that can't be written means no check.
func startUpdateCheck(home, exe string, cache selfupdate.Cache, now time.Time) {
	cache.Checked = now
	if selfupdate.WriteCache(home, cache) != nil {
		return
	}
	cmd := exec.Command(exe, "_update-check")
	cmd.Env = setEnv(os.Environ(), "HECATRON_HOME", home)
	detach(cmd)
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
}

// updateCheck is `hecatron _update-check`: ask GitHub for the latest
// release, with a short timeout, and cache it. Silent: it runs on its own.
func (c *cli) updateCheck() int {
	client := selfupdate.NewClient(os.Getenv, c.home.Version(), &http.Client{Timeout: 10 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	release, err := client.Latest(ctx)
	if err != nil {
		return 1
	}
	if selfupdate.WriteCache(c.home.Dir, selfupdate.Cache{Checked: time.Now().UTC(), Latest: release.Version}) != nil {
		return 1
	}
	return 0
}

// knownUpdate is the newer release the last check found, from the cache
// only, for hct status; "" when none is known or this is a dev build.
func (c *cli) knownUpdate() (latest string, checked time.Time) {
	exe, err := executable()
	if err != nil || !selfupdate.IsReleaseBinary(c.home.Dir, exe) {
		return "", time.Time{}
	}
	cache := selfupdate.ReadCache(c.home.Dir)
	return cache.Newer(c.home.Version()), cache.Checked
}

// writeEnv is `hecatron _write-env DIR HOME`: install.sh's env and
// env.fish for an installation at HOME, written into DIR. hct self update
// runs the new release's binary with it, so the files are that release's.
func writeEnv(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: hecatron _write-env DIR HOME")
		return 2
	}
	if err := selfupdate.WriteEnv(args[0], args[1], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	return 0
}
