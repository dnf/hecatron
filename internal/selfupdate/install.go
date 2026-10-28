package selfupdate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// An installation install.sh made is a git clone of hecatron at a release's
// tag, with that release's binary in bin/.hecatron-go, the commit it belongs
// to in bin/.hecatron-go.commit (bin/hecatron runs the binary only at that
// commit) and the env file it sources from the shell's startup files.
const (
	binaryPath = "bin/.hecatron-go"
	commitPath = binaryPath + ".commit"
	envPath    = "env"
)

// Install is an installation self-update can replace.
type Install struct {
	Home    string // the installation directory
	Version string // its VERSION
	Origin  string // the repository it was cloned from
}

// Refusal is why an installation can't update itself, and how to update it
// instead.
type Refusal struct {
	Reason string
	Advice string
}

func (r *Refusal) Error() string {
	if r.Advice == "" {
		return r.Reason
	}
	return r.Reason + "; " + r.Advice
}

const pullAdvice = "update it with git pull (and hct self-build for the binary)"

// IsReleaseBinary is whether exe is the release binary of an installation
// at home: in its bin/, at the commit it was downloaded for. Cheap enough to
// ask on every command (no git), for the update notice: a dev build (hct
// self-build's, or go build's) is never told about releases.
func IsReleaseBinary(home, exe string) bool {
	if !sameFile(exe, filepath.Join(home, binaryPath)) {
		return false
	}
	commit := readFile(filepath.Join(home, commitPath))
	head := readFile(filepath.Join(home, ".git", "HEAD"))
	return isCommit(commit) && head == commit
}

// Inspect is the installation at home, which exe (this binary) belongs
// to, if self-update can replace it; else a *Refusal saying why.
func Inspect(home, exe string) (*Install, error) {
	if runtime.GOOS == "windows" {
		return nil, &Refusal{"hct self-update doesn't run on Windows yet", "download the release's binary from GitHub"}
	}
	commit := readFile(filepath.Join(home, commitPath))
	switch {
	case !sameFile(exe, filepath.Join(home, binaryPath)):
		if isDir(filepath.Join(home, ".git")) || isFile(filepath.Join(home, ".git")) {
			return nil, &Refusal{"this hct is a build from the checkout " + home + ", not a release", pullAdvice}
		}
		return nil, &Refusal{"this hct isn't the binary of an installation install.sh made (" + home + ")", "update it the way it was installed"}
	case commit == "dev":
		return nil, &Refusal{"this hct is a dev build (hct self-build) in the checkout " + home, pullAdvice}
	case !isDir(filepath.Join(home, ".git")) || !isFile(filepath.Join(home, envPath)):
		return nil, &Refusal{home + " wasn't installed by install.sh", "update it the way it was installed"}
	}
	head := readFile(filepath.Join(home, ".git", "HEAD"))
	switch {
	case strings.HasPrefix(head, "ref:"):
		return nil, &Refusal{home + " is a git checkout on a branch, not a release", pullAdvice}
	case head != commit:
		return nil, &Refusal{home + "'s binary doesn't belong to the commit it's at", "run install.sh again"}
	}
	if changes, err := git(home, "status", "--porcelain", "--untracked-files=no"); err != nil || changes != "" {
		return nil, &Refusal{home + " has changes to hecatron's own files", pullAdvice + ", or commit or discard them"}
	}
	origin, err := git(home, "remote", "get-url", "origin")
	if err != nil || origin == "" {
		return nil, &Refusal{home + " has no origin to fetch the release from", "run install.sh again"}
	}
	for _, dir := range []string{home, filepath.Dir(home)} {
		if err := writable(dir); err != nil {
			return nil, &Refusal{dir + " isn't writable (" + err.Error() + ")", "run hct self-update as the user who installed hecatron"}
		}
	}
	version := readFile(filepath.Join(home, "VERSION"))
	if !ValidVersion(version) {
		return nil, &Refusal{home + "/VERSION isn't a version (" + version + ")", "run install.sh again"}
	}
	return &Install{Home: home, Version: version, Origin: origin}, nil
}

// Replace installs release, with its verified binary, in place of the
// installation: the release's tag is fetched into the installation's
// clone (its files untouched), cloned beside it with the binary and the
// installation's own files (env, the update cache...), and the two swapped
// by renaming. Until the rename nothing the installation runs has changed,
// so a failure (a network error, a full disk) leaves it working; a failed
// rename is undone.
func Replace(install *Install, release *Release, binary []byte) error {
	home := install.Home
	parent, base := filepath.Dir(home), filepath.Base(home)
	if !strings.HasPrefix(base, ".") {
		base = "." + base // hidden beside the installation
	}
	if _, err := git(home, "fetch", "--quiet", "--force", "--no-tags", install.Origin,
		"+refs/tags/"+release.Tag+":refs/tags/"+release.Tag); err != nil {
		return fmt.Errorf("fetching %s from %s: %w", release.Tag, install.Origin, err)
	}
	staging, err := os.MkdirTemp(parent, base+".update-")
	if err != nil {
		return err
	}
	swapped := false
	defer func() {
		if !swapped {
			_ = os.RemoveAll(staging)
		}
	}()
	if _, err := git("", "-c", "advice.detachedHead=false", "clone", "--quiet", "--branch", release.Tag, home, staging); err != nil {
		return fmt.Errorf("checking out %s: %w", release.Tag, err)
	}
	if _, err := git(staging, "remote", "set-url", "origin", install.Origin); err != nil {
		return err
	}
	if got := readFile(filepath.Join(staging, "VERSION")); got != release.Version {
		return fmt.Errorf("%s's VERSION is %q, not %s", release.Tag, got, release.Version)
	}
	commit, err := git(staging, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if err := copyUntracked(home, staging); err != nil {
		return fmt.Errorf("copying the installation's own files: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(staging, "bin"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, binaryPath), binary, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, commitPath), []byte(commit+"\n"), 0o644); err != nil {
		return err
	}
	if err := writeNewEnv(staging, home); err != nil {
		return fmt.Errorf("writing env: %w", err)
	}
	// Two renames, not one atomic step: POSIX can't swap two directories,
	// so for a moment there is no installation. A failed second rename
	// puts the old one back.
	old := filepath.Join(parent, base+".old-"+strings.TrimPrefix(filepath.Base(staging), base+".update-"))
	if err := os.Rename(home, old); err != nil {
		return err
	}
	if err := os.Rename(staging, home); err != nil {
		if undo := os.Rename(old, home); undo != nil {
			return fmt.Errorf("%w; and moving %s back to %s failed: %v", err, old, home, undo)
		}
		return err
	}
	swapped = true
	_ = os.RemoveAll(old)
	return nil
}

// writeNewEnv writes env and env.fish as the release's install.sh would:
// with this binary's writer, then the release's own binary's (`hct
// _write-env`), which knows that release's PATH entries; a release from
// before that command keeps this one's files. A new PATH entry (the
// toolchains' links) so reaches shells without rerunning install.sh.
func writeNewEnv(staging, home string) error {
	if err := WriteEnv(staging, home, os.Getenv); err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(staging, binaryPath), "_write-env", staging, home)
	cmd.Env = append(os.Environ(), "HECATRON_HOME="+staging, "HECATRON_NO_UPDATE_CHECK=1")
	if cmd.Run() != nil {
		return WriteEnv(staging, home, os.Getenv) // whatever it left half-written
	}
	return nil
}

// copyUntracked copies the files in home that git doesn't track (env,
// env.fish, the update cache, the skill record...) into staging, except
// the old binary and Python's caches, and what the new version tracks.
func copyUntracked(home, staging string) error {
	out, err := gitRaw(home, "ls-files", "-z", "--others")
	if err != nil {
		return err
	}
	for _, name := range strings.Split(out, "\x00") {
		if name == "" || name == binaryPath || name == commitPath || strings.Contains("/"+name+"/", "/__pycache__/") {
			continue
		}
		source, target := filepath.Join(home, filepath.FromSlash(name)), filepath.Join(staging, filepath.FromSlash(name))
		if _, err := os.Lstat(target); err == nil {
			continue
		}
		if err := copyEntry(source, target); err != nil {
			return err
		}
	}
	return nil
}

func copyEntry(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(link, target)
	case info.Mode().IsRegular():
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	}
	return nil
}

func git(dir string, args ...string) (string, error) {
	out, err := gitRaw(dir, args...)
	return strings.TrimSpace(out), err
}

func gitRaw(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("%s", message)
		}
		return "", err
	}
	return string(out), nil
}

// writable is whether a file can be created in dir.
func writable(dir string) error {
	file, err := os.CreateTemp(dir, ".hecatron-write-test-")
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return pathErr.Err
		}
		return err
	}
	_ = file.Close()
	return os.Remove(file.Name())
}

func sameFile(a, b string) bool {
	x, errA := os.Stat(a)
	y, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(x, y)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isCommit(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
