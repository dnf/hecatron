package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hct skill install copies the skill where an agent looks, replaces its own
// earlier copy, and leaves a folder that isn't hecatron's alone.
func TestSkillInstall(t *testing.T) {
	home := repoHome(t)
	user := t.TempDir()
	env := []string{"HECATRON_HOME=" + home, "HOME=" + user, "USERPROFILE=" + user}
	installed := filepath.Join(user, ".claude", "skills", "hecatron", "SKILL.md")

	got := runIn(t, t.TempDir(), env, binary, "skill", "install")
	if got.status != 0 || !strings.Contains(got.stdout, "Installed hecatron's skill") {
		t.Fatalf("install: %+v", got)
	}
	want, _ := os.ReadFile(filepath.Join(home, skillSource, "SKILL.md"))
	if data, err := os.ReadFile(installed); err != nil || string(data) != string(want) {
		t.Fatalf("SKILL.md not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(installed), "references", "reference.md")); err != nil {
		t.Errorf("references not copied: %v", err)
	}

	// An earlier copy, with a file the skill no longer has, is replaced.
	stale := filepath.Join(filepath.Dir(installed), "stale.md")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runIn(t, t.TempDir(), env, binary, "skill", "install", "--user"); got.status != 0 {
		t.Fatalf("reinstall: %+v", got)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("an earlier copy's files stayed")
	}

	// Someone else's folder of that name is left alone.
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "hecatron"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "hecatron", "SKILL.md"), []byte("---\nname: mine\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = runIn(t, t.TempDir(), env, binary, "skill", "install", "--dir", other)
	if got.status != 1 || !strings.Contains(got.stderr, "isn't hecatron's skill") {
		t.Errorf("overwrote another skill: %+v", got)
	}
}

func TestSkillInstallProjectAndPath(t *testing.T) {
	home := repoHome(t)
	userHome := t.TempDir()
	env := []string{"HECATRON_HOME=" + home, "HOME=" + userHome, "USERPROFILE=" + userHome}
	repo := fixture(t, map[string]string{"README": ""})
	runIn(t, repo, nil, "git", "init", "-q")
	sub := filepath.Join(repo, "a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := runIn(t, sub, env, binary, "skill", "install", "--project"); got.status != 0 {
		t.Fatalf("--project: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "skills", "hecatron", "SKILL.md")); err != nil {
		t.Errorf("not installed at the repository's top: %v", err)
	}
	if got := runIn(t, t.TempDir(), env, binary, "skill", "install", "--project"); got.status != 2 {
		t.Errorf("--project outside git: %+v", got)
	}
	got := runIn(t, t.TempDir(), env, binary, "skill", "path")
	if got.status != 0 || strings.TrimSpace(got.stdout) != filepath.Join(home, skillSource) {
		t.Errorf("path: %+v", got)
	}
	if got := runIn(t, t.TempDir(), env, binary, "skill", "bogus"); got.status != 2 {
		t.Errorf("unknown subcommand: %+v", got)
	}
}

// Without the binary, bin/hecatron says skill needs it.
func TestSkillNeedsTheBinary(t *testing.T) {
	home := repoHome(t)
	got := runIn(t, t.TempDir(), []string{"HECATRON_HOME=" + home, "HECATRON_NO_GO=1"}, "bash", filepath.Join(home, "bin", "hecatron"), "skill", "install")
	if got.status != 1 || !strings.Contains(got.stderr, "needs hecatron's binary") {
		t.Errorf("bin/hecatron skill: %+v", got)
	}
}
