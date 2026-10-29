package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The shell startup files install.sh adds hecatron's line to, in the
// user's home, and the comment that ends that line (install.sh replaces
// any line ending in it, too).
var startupFiles = []string{".profile", ".bashrc", ".bash_profile", ".bash_login", ".zshrc", ".zshenv"}

const lineMarker = "  # hecatron"

// fishFile is the file install.sh writes for fish, in the user's home.
var fishFile = filepath.Join(".config", "fish", "conf.d", "hecatron.fish")

// Uninstall is what self uninstall removes of hecatron's own installation.
type Uninstall struct {
	Home     string   // the installation, or "" when it's a checkout, which stays
	Checkout string   // the checkout hecatron runs from, kept
	Version  string   // the installation's VERSION
	Startup  []string // startup files with hecatron's line in them
	Fish     string   // fish's file, when install.sh wrote one
}

// PlanUninstall is what uninstalling hecatron at home, which exe (this
// binary) runs from, removes for the user whose home is user; else a
// *Refusal saying why it can't. An installation install.sh made goes
// entirely; a checkout install.sh was run from keeps its files, losing
// only the startup files' lines.
func PlanUninstall(home, exe, user string) (*Uninstall, error) {
	if runtime.GOOS == "windows" {
		return nil, &Refusal{"hct self uninstall doesn't run on Windows yet", "delete " + home}
	}
	u := &Uninstall{Version: readFile(filepath.Join(home, "VERSION"))}
	release := isDir(filepath.Join(home, ".git")) && isFile(filepath.Join(home, envPath)) &&
		sameFile(exe, filepath.Join(home, binaryPath)) && readFile(filepath.Join(home, commitPath)) != "dev"
	switch {
	case release:
		if changes, err := git(home, "status", "--porcelain", "--untracked-files=no"); err != nil || changes != "" {
			return nil, &Refusal{home + " has changes to hecatron's own files", "commit or discard them, or delete " + home + " yourself"}
		}
		u.Home = home
	case isDir(filepath.Join(home, ".git")) || isFile(filepath.Join(home, ".git")):
		u.Checkout = home
	default:
		return nil, &Refusal{"this hct wasn't installed by install.sh (" + home + ")", "uninstall it the way it was installed"}
	}
	for _, name := range startupFiles {
		path := filepath.Join(user, name)
		if _, found := withoutLine(readAll(path)); found {
			u.Startup = append(u.Startup, path)
		}
	}
	if path := filepath.Join(user, fishFile); strings.HasSuffix(strings.TrimSpace(readAll(path)), strings.TrimSpace(lineMarker)) {
		u.Fish = path
	}
	return u, nil
}

// Remove uninstalls: the startup files' lines first, so a failure
// partway never leaves a shell sourcing a file that's gone.
func (u *Uninstall) Remove() error {
	for _, path := range u.Startup {
		kept, _ := withoutLine(readAll(path))
		if err := os.WriteFile(path, []byte(kept), 0o644); err != nil {
			return err
		}
	}
	if u.Fish != "" {
		if err := os.Remove(u.Fish); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if u.Home != "" {
		return os.RemoveAll(u.Home)
	}
	return nil
}

// withoutLine is a startup file's text without the lines install.sh
// added, each with the blank line install.sh put before it; and whether
// there were any.
func withoutLine(text string) (string, bool) {
	lines := strings.SplitAfter(text, "\n")
	var kept []string
	found := false
	for _, line := range lines {
		if strings.HasSuffix(strings.TrimRight(line, "\r\n"), lineMarker) {
			found = true
			if n := len(kept); n > 0 && strings.TrimSpace(kept[n-1]) == "" {
				kept = kept[:n-1]
			}
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, ""), found
}

// readAll is a file's text, or "" when it can't be read.
func readAll(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
