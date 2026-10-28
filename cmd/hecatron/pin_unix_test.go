package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pinHome is a hecatron home at version: the checkout's files, linked,
// with VERSION of its own, and with dev a binary hct self build built.
func pinHome(t *testing.T, version string, dev bool) string {
	t.Helper()
	checkout := repoHome(t)
	home := t.TempDir()
	for _, name := range []string{"libexec", "languages", "plugins", "bin/hecatron", "bin/hct"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(home, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(checkout, name), filepath.Join(home, name)); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{"VERSION": version + "\n"}
	if dev {
		files["bin/.hecatron-go.commit"] = "dev\n"
	}
	writeFiles(t, home, files)
	return home
}

// A project's "hecatron": "0.5" pins it: another hecatron warns once on
// stderr (not for help or --version), hct check fails on it but for a dev
// build, and the generated GitLab pipeline installs the pin.
func TestPinnedHecatron(t *testing.T) {
	hct := func(home, root string, args ...string) result {
		env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_PIN_WARNED=", "CI=1"}
		return runIn(t, root, env, append([]string{binary}, args...)...)
	}
	const warning = "hecatron: warning: this project pins hecatron 0.5, but this is 0.6.0 " +
		"(hct self update --version 0.5.N installs one that matches)\n"
	root := fixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "hecatron": "0.5", "languages": [], "generated": {"ci": "gitlab"}}`,
	})
	release, dev := pinHome(t, "0.6.0", false), pinHome(t, "0.6.0", true)

	if got := hct(release, root, "list"); got.status != 0 || got.stderr != warning {
		t.Errorf("list: %+v, want the warning once", got)
	}
	for _, args := range [][]string{{"help"}, {"--version"}, {"self", "update", "--help"}} {
		if got := hct(release, root, args...); strings.Contains(got.stderr, "pins hecatron") {
			t.Errorf("%v warns: %+v", args, got)
		}
	}
	got := hct(release, root, "check")
	if got.status != 1 || !strings.Contains(got.stdout, ".hecatron/config.json: this project pins hecatron 0.5, but this is 0.6.0") ||
		strings.Contains(got.stderr, "warning") {
		t.Errorf("check with another hecatron: %+v", got)
	}
	if got := hct(dev, root, "check"); got.status != 1 || !strings.Contains(got.stderr, "; a dev build, so hct check passes") {
		// 1: the pipeline isn't generated yet; the pin itself doesn't fail
		t.Errorf("check from a dev build: %+v", got)
	}
	if got := hct(dev, root, "workflows"); got.status != 0 || got.stderr != warning {
		t.Fatalf("workflows: %+v", got)
	}
	if got := hct(dev, root, "check"); got.status != 0 || strings.Contains(got.stdout, "pins") {
		t.Errorf("check from a dev build: %+v, want it to pass", got)
	}
	pipeline, err := os.ReadFile(filepath.Join(root, ".gitlab-ci.yml"))
	if err != nil || !strings.Contains(string(pipeline), "HECATRON_VERSION: v0.5\n") ||
		!strings.Contains(string(pipeline), "/dnf/hecatron/main/install.sh") {
		t.Errorf("the pipeline doesn't install the pinned series (%v):\n%s", err, pipeline)
	}

	matching := pinHome(t, "0.5.3", false)
	if got := hct(matching, root, "list"); got.stderr != "" {
		t.Errorf("list with a hecatron the pin takes: %+v", got)
	}
	if got := hct(matching, root, "check"); got.status != 0 {
		t.Errorf("check with a hecatron the pin takes: %+v", got)
	}

	malformed := fixture(t, map[string]string{".hecatron/config.json": `{"schema": 1, "hecatron": "0.x", "languages": []}`})
	if got := hct(matching, malformed, "check"); got.status != 1 ||
		!strings.Contains(got.stdout, `.hecatron/config.json: "hecatron" must pin a version`) {
		t.Errorf("check with a malformed pin: %+v", got)
	}
}

// A command's hct commands (its tasks, hooks) don't warn again.
func TestPinWarnsOnce(t *testing.T) {
	home := pinHome(t, "0.6.0", false)
	root := fixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "hecatron": "0.5", "languages": [],
			"tasks": {"test": {"nested": "\"$HCT\" list > nested.out"}}}`,
	})
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_PIN_WARNED=", "CI=1", "HCT=" + binary}
	got := runIn(t, root, env, binary, "test")
	if _, err := os.Stat(filepath.Join(root, "nested.out")); err != nil || strings.Count(got.stderr, "pins hecatron") != 1 {
		t.Errorf("test with a nested hct: %+v, want one warning", got)
	}
}
