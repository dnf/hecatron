//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hct self uninstall, on a fake installation (fakeInstall) and a user's
// home with install.sh's lines in its startup files.

const userProfile = "export EDITOR=vi\n"

// uninstallUser is a user's home as install.sh leaves it for home, with
// hecatron's skill installed and a toolchain unpacked.
func uninstallUser(t *testing.T, home string) (user string, env []string) {
	user = t.TempDir()
	line := "\n. \"" + home + "/env\"  # hecatron\n"
	writeFiles(t, user, map[string]string{
		".profile":                          userProfile + line,
		".zshrc":                            line,
		".config/fish/conf.d/hecatron.fish": "source \"" + home + "/env.fish\"  # hecatron\n",
		".local/share/hecatron/toolchains/zig/0.17.0/zig": "",
		".cache/hecatron/downloads/x":                     "",
		".bashrc":                                         "# no hecatron here\n",
	})
	env = []string{"HECATRON_HOME=" + home, "HOME=" + user, "XDG_DATA_HOME=", "XDG_CACHE_HOME=",
		"CI=", "GITHUB_ACTIONS=", "CLAUDECODE=", "HECATRON_OUTPUT=", "HECATRON_NO_UPDATE_CHECK=1"}
	if got := runIn(t, t.TempDir(), env, filepath.Join(home, "bin", ".hecatron-go"), "skill", "install"); got.status != 0 {
		t.Fatalf("skill install: %+v", got)
	}
	return user, env
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestSelfUninstall(t *testing.T) {
	home := fakeInstall(t)
	user, env := uninstallUser(t, home)
	hct := filepath.Join(home, "bin", ".hecatron-go")
	skill := filepath.Join(user, ".claude", "skills", "hecatron")

	// Not on a terminal, without --yes: it says what it would remove, and doesn't.
	got := runIn(t, t.TempDir(), env, hct, "self", "uninstall")
	for _, want := range []string{home + " (hecatron 0.5.0)", "hecatron's line in " + filepath.Join(user, ".profile"),
		"hecatron's line in " + filepath.Join(user, ".zshrc"), skill, "--all removes it"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("plan lacks %q: %+v", want, got)
		}
	}
	if got.status != 1 || strings.Contains(got.stdout, ".bashrc") || !exists(home) || !exists(skill) {
		t.Fatalf("without --yes: %+v", got)
	}

	got = runIn(t, t.TempDir(), env, hct, "self", "uninstall", "--yes")
	if got.status != 0 || !strings.Contains(got.stdout, "Removed hecatron") {
		t.Fatalf("--yes: %+v", got)
	}
	for _, gone := range []string{home, skill, filepath.Join(user, ".config/fish/conf.d/hecatron.fish")} {
		if exists(gone) {
			t.Errorf("%s is still there", gone)
		}
	}
	if profile := readText(t, filepath.Join(user, ".profile")); profile != userProfile {
		t.Errorf(".profile is %q", profile)
	}
	if zshrc := readText(t, filepath.Join(user, ".zshrc")); zshrc != "" {
		t.Errorf(".zshrc is %q", zshrc)
	}
	if !exists(filepath.Join(user, ".local/share/hecatron/toolchains/zig")) || !exists(filepath.Join(user, ".cache/hecatron")) {
		t.Errorf("removed the toolchains without --all")
	}
}

func TestSelfUninstallAll(t *testing.T) {
	home := fakeInstall(t)
	user, env := uninstallUser(t, home)
	got := runIn(t, t.TempDir(), env, filepath.Join(home, "bin", ".hecatron-go"), "self", "uninstall", "--all", "--yes")
	if got.status != 0 {
		t.Fatalf("%+v", got)
	}
	for _, gone := range []string{home, filepath.Join(user, ".local/share/hecatron"), filepath.Join(user, ".cache/hecatron")} {
		if exists(gone) {
			t.Errorf("%s is still there", gone)
		}
	}
}

func TestSelfUninstallKeeps(t *testing.T) {
	t.Run("a checkout", func(t *testing.T) {
		checkout := t.TempDir()
		mustGit(t, checkout, "init", "-q")
		writeFiles(t, checkout, map[string]string{"VERSION": "0.5.0\n", "env": ""})
		user, env := uninstallUser(t, fakeInstall(t))
		env[0] = "HECATRON_HOME=" + checkout
		writeFiles(t, user, map[string]string{".profile": userProfile + "\n. \"" + checkout + "/env\"  # hecatron\n"})
		got := runIn(t, t.TempDir(), env, binary, "self", "uninstall", "--yes")
		if got.status != 0 || !strings.Contains(got.stdout, "It keeps the checkout "+checkout) || !exists(filepath.Join(checkout, "VERSION")) {
			t.Fatalf("%+v", got)
		}
		if profile := readText(t, filepath.Join(user, ".profile")); profile != userProfile {
			t.Errorf(".profile is %q", profile)
		}
	})
	t.Run("changes to hecatron's files", func(t *testing.T) {
		home := fakeInstall(t)
		_, env := uninstallUser(t, home)
		writeFiles(t, home, map[string]string{"VERSION": "0.5.0-mine\n"})
		got := runIn(t, t.TempDir(), env, filepath.Join(home, "bin", ".hecatron-go"), "self", "uninstall", "--yes")
		if got.status != 1 || !strings.Contains(got.stderr, "has changes to hecatron's own files") || !exists(home) {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("not install.sh's", func(t *testing.T) {
		got := runIn(t, t.TempDir(), []string{"HECATRON_HOME=" + t.TempDir(), "HOME=" + t.TempDir()}, binary, "self", "uninstall", "--yes")
		if got.status != 1 || !strings.Contains(got.stderr, "wasn't installed by install.sh") {
			t.Fatalf("%+v", got)
		}
	})
}
