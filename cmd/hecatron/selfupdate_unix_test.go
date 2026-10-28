//go:build unix

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dnf/hecatron/internal/selfupdate"
)

// hct self-update and the update notice, against a fake GitHub (its
// releases API and downloads, through HECATRON_UPDATE_URL) and a fake
// installation as install.sh makes one: a clone of a repository with
// releases v0.5.0 and v0.6.0, at v0.5.0, with this binary as its
// bin/.hecatron-go.

type fakeGitHub struct {
	*httptest.Server
	mu       sync.Mutex
	latest   string
	binaries map[string]string // version: the binary's contents
	badSum   bool              // SHA256SUMS lists another checksum
	partial  bool              // the binary's download stops halfway
	checks   int               // requests for the latest release
	token    string            // the last Authorization header
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{latest: "0.6.0", binaries: map[string]string{
		"0.4.0": "#!/bin/sh\necho hct 0.4.0\n",
		"0.5.0": "#!/bin/sh\necho hct 0.5.0\n",
		"0.6.0": "#!/bin/sh\necho hct 0.6.0\n",
	}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	asset := selfupdate.AssetName()
	switch path := r.URL.Path; {
	case path == "/releases/latest" || strings.HasPrefix(path, "/releases/tags/v"):
		version := strings.TrimPrefix(path, "/releases/tags/v")
		if path == "/releases/latest" {
			version = f.latest
			f.checks++
			f.token = r.Header.Get("Authorization")
		}
		if _, ok := f.binaries[version]; !ok {
			http.NotFound(w, r)
			return
		}
		download := f.URL + "/download/v" + version + "/"
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v" + version, "assets": []map[string]string{
			{"name": asset, "browser_download_url": download + asset},
			{"name": "SHA256SUMS", "browser_download_url": download + "SHA256SUMS"},
		}})
	case strings.HasPrefix(path, "/download/v"):
		version, name, _ := strings.Cut(strings.TrimPrefix(path, "/download/v"), "/")
		binary := f.binaries[version]
		switch name {
		case "SHA256SUMS":
			sum := sha256.Sum256([]byte(binary))
			if f.badSum {
				sum = sha256.Sum256([]byte("something else"))
			}
			_, _ = fmt.Fprintf(w, "%s  %s\n%s  hecatron-plan9-mips\n", hex.EncodeToString(sum[:]), asset, strings.Repeat("0", 64))
		case asset:
			if f.partial {
				w.Header().Set("Content-Length", fmt.Sprint(len(binary)))
				_, _ = w.Write([]byte(binary[:len(binary)/2]))
				return
			}
			_, _ = w.Write([]byte(binary))
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeGitHub) latestChecks() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checks
}

func (f *fakeGitHub) lastToken() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.token
}

var gitIdentity = []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t",
	"GIT_COMMITTER_EMAIL=t@example.com", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	got := runIn(t, dir, gitIdentity, append([]string{"git"}, args...)...)
	if got.status != 0 {
		t.Fatalf("git %v: %+v", args, got)
	}
	return strings.TrimSpace(got.stdout)
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const fakeSkill = "---\nname: hecatron\ndescription: x\nmetadata:\n  hecatron-version: \"%s\"\n---\n"

// fakeInstall is an installation of the fake hecatron at v0.5.0, as
// install.sh makes one, in a directory of its own; its origin has v0.4.0
// to v0.6.0.
func fakeInstall(t *testing.T) (home string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, origin, "init", "-q")
	for _, version := range []string{"0.4.0", "0.5.0", "0.6.0"} {
		writeFiles(t, origin, map[string]string{
			"VERSION": version + "\n",
			"plugins/hecatron/skills/hecatron/SKILL.md": fmt.Sprintf(fakeSkill, version),
			".gitignore": "/bin/.hecatron-go\n/bin/.hecatron-go.commit\n/.update-check.json\n/.skill-installs\n",
		})
		mustGit(t, origin, "add", "-A")
		mustGit(t, origin, "commit", "-qm", "Release "+version)
		mustGit(t, origin, "tag", "v"+version)
	}
	home = filepath.Join(t.TempDir(), ".hecatron")
	mustGit(t, "", "-c", "advice.detachedHead=false", "clone", "-q", "--branch", "v0.5.0", origin, home)
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, home, map[string]string{"env": "# hecatron's PATH\n", "bin/.hecatron-go.commit": mustGit(t, home, "rev-parse", "HEAD") + "\n"})
	if err := os.WriteFile(filepath.Join(home, "bin", ".hecatron-go"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

// updateEnv is an environment where nothing outside the test decides: no
// CI, no agent, no real home or token.
func updateEnv(t *testing.T, home string, server *fakeGitHub, extra ...string) []string {
	return append([]string{"HECATRON_HOME=" + home, "HECATRON_UPDATE_URL=" + server.URL, "HOME=" + t.TempDir(),
		"XDG_DATA_HOME=", "CI=", "GITHUB_ACTIONS=", "CLAUDECODE=", "HECATRON_OUTPUT=", "HECATRON_NO_UPDATE_CHECK=", "GITHUB_TOKEN="}, extra...)
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// leftovers are self-update's staging or old directories beside home.
func leftovers(t *testing.T, home string) []string {
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(home), ".hecatron.*"))
	return matches
}

func TestSelfUpdate(t *testing.T) {
	server := newFakeGitHub(t)
	home := fakeInstall(t)
	env := updateEnv(t, home, server)
	user := env[2][len("HOME="):]
	hct := filepath.Join(home, "bin", ".hecatron-go")

	// hct skill install's copies: recorded (--dir) and the default one.
	other := t.TempDir()
	for _, args := range [][]string{{"skill", "install"}, {"skill", "install", "--dir", other}} {
		if got := runIn(t, t.TempDir(), env, append([]string{hct}, args...)...); got.status != 0 {
			t.Fatalf("%v: %+v", args, got)
		}
	}
	writeFiles(t, home, map[string]string{selfupdate.CacheFile: `{"checked":"` + time.Now().UTC().Format(time.RFC3339) + `","latest":"0.6.0"}`})

	got := runIn(t, t.TempDir(), env, hct, "self-update", "--check")
	if got.status != 3 || !strings.Contains(got.stdout, "hecatron 0.6.0 is available (this is 0.5.0): run hct self-update") {
		t.Fatalf("--check: %+v", got)
	}

	got = runIn(t, t.TempDir(), env, hct, "self-update")
	if got.status != 0 || !strings.Contains(got.stdout, "Updated hecatron 0.5.0 → 0.6.0") {
		t.Fatalf("self-update: %+v", got)
	}
	if v := readText(t, filepath.Join(home, "VERSION")); v != "0.6.0\n" {
		t.Errorf("VERSION is %q", v)
	}
	if b := readText(t, hct); b != server.binaries["0.6.0"] {
		t.Errorf("the binary wasn't replaced: %q", b)
	}
	if commit := strings.TrimSpace(readText(t, hct+".commit")); commit != mustGit(t, home, "rev-parse", "HEAD") {
		t.Errorf(".commit %s isn't HEAD", commit)
	}
	if tag := mustGit(t, home, "describe", "--tags", "--exact-match"); tag != "v0.6.0" {
		t.Errorf("at %s", tag)
	}
	wantEnv, wantFish := selfupdate.EnvFiles(home, func(name string) string {
		return map[string]string{"HOME": user}[name]
	})
	if env := readText(t, filepath.Join(home, "env")); env != wantEnv {
		t.Errorf("env not rewritten as install.sh would: %q", env)
	}
	if fish := readText(t, filepath.Join(home, "env.fish")); fish != wantFish {
		t.Errorf("env.fish not written as install.sh would: %q", fish)
	}
	if !selfupdate.IsReleaseBinary(home, hct) {
		t.Errorf("not a release installation afterwards")
	}
	for _, skill := range []string{filepath.Join(user, ".claude", "skills", "hecatron"), filepath.Join(other, "hecatron")} {
		if !strings.Contains(readText(t, filepath.Join(skill, "SKILL.md")), `"0.6.0"`) {
			t.Errorf("skill in %s not refreshed", skill)
		}
		if !strings.Contains(got.stdout, "Refreshed hecatron's skill in "+skill) {
			t.Errorf("refresh of %s not reported:\n%s", skill, got.stdout)
		}
	}
	if l := leftovers(t, home); len(l) > 0 {
		t.Errorf("left behind: %v", l)
	}

	// The new binary is the fake one; the test binary checks again.
	if err := os.WriteFile(hct, []byte(readText(t, binary)), 0o755); err != nil {
		t.Fatal(err)
	}
	got = runIn(t, t.TempDir(), env, hct, "self-update")
	if got.status != 0 || got.stdout != "hecatron 0.6.0 is already up to date\n" {
		t.Errorf("up to date: %+v", got)
	}
	got = runIn(t, t.TempDir(), env, hct, "self-update", "--check")
	if got.status != 0 || !strings.Contains(got.stdout, "hecatron 0.6.0 is up to date") {
		t.Errorf("--check up to date: %+v", got)
	}

	// --version installs another release, an older one too.
	got = runIn(t, t.TempDir(), env, hct, "self-update", "--version", "0.4.0")
	if got.status != 0 || !strings.Contains(got.stdout, "Updated hecatron 0.6.0 → 0.4.0") || readText(t, filepath.Join(home, "VERSION")) != "0.4.0\n" {
		t.Errorf("--version 0.4.0: %+v", got)
	}
	if err := os.WriteFile(hct, []byte(readText(t, binary)), 0o755); err != nil {
		t.Fatal(err)
	}
	got = runIn(t, t.TempDir(), env, hct, "self-update", "--version", "9.9.9")
	if got.status != 1 || !strings.Contains(got.stderr, "there is no release 9.9.9") {
		t.Errorf("--version 9.9.9: %+v", got)
	}
}

// A failed download, a checksum that doesn't match: nothing changes.
func TestSelfUpdateFailuresChangeNothing(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(*fakeGitHub)
		want  string
	}{
		{"checksum mismatch", func(f *fakeGitHub) { f.badSum = true }, "checksum doesn't match"},
		{"partial download", func(f *fakeGitHub) { f.partial = true }, "downloading"},
	} {
		t.Run(c.name, func(t *testing.T) {
			server := newFakeGitHub(t)
			c.setup(server)
			home := fakeInstall(t)
			hct := filepath.Join(home, "bin", ".hecatron-go")
			before := readText(t, hct)
			got := runIn(t, t.TempDir(), updateEnv(t, home, server), hct, "self-update")
			if got.status != 1 || !strings.Contains(got.stderr, c.want) || !strings.Contains(got.stderr, "hecatron 0.5.0 is unchanged") {
				t.Errorf("%+v", got)
			}
			if readText(t, filepath.Join(home, "VERSION")) != "0.5.0\n" || readText(t, hct) != before {
				t.Errorf("the installation changed")
			}
			if l := leftovers(t, home); len(l) > 0 {
				t.Errorf("left behind: %v", l)
			}
		})
	}
}

// Installations self-update can't replace are refused, saying how to
// update them instead.
func TestSelfUpdateRefuses(t *testing.T) {
	server := newFakeGitHub(t)
	cases := []struct {
		name  string
		setup func(t *testing.T, home string) (hct string)
		want  string
	}{
		{"dev build", func(t *testing.T, home string) string {
			writeFiles(t, home, map[string]string{"bin/.hecatron-go.commit": "dev\n"})
			return ""
		}, "dev build (hct self-build)"},
		{"go build in a checkout", func(t *testing.T, home string) string { return binary }, "a build from the checkout"},
		{"a branch", func(t *testing.T, home string) string {
			mustGit(t, home, "checkout", "-q", "-b", "work")
			return ""
		}, "a git checkout on a branch"},
		{"local changes", func(t *testing.T, home string) string {
			writeFiles(t, home, map[string]string{"VERSION": "0.5.0\n\n"})
			return ""
		}, "has changes to hecatron's own files"},
		{"not install.sh's", func(t *testing.T, home string) string {
			if err := os.Remove(filepath.Join(home, "env")); err != nil {
				t.Fatal(err)
			}
			return ""
		}, "wasn't installed by install.sh"},
		{"not writable", func(t *testing.T, home string) string {
			if os.Geteuid() == 0 {
				t.Skip("root writes anywhere")
			}
			if err := os.Chmod(home, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(home, 0o755) })
			return ""
		}, "isn't writable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := fakeInstall(t)
			hct := c.setup(t, home)
			if hct == "" {
				hct = filepath.Join(home, "bin", ".hecatron-go")
			}
			got := runIn(t, t.TempDir(), updateEnv(t, home, server), hct, "self-update")
			if got.status != 1 || !strings.Contains(got.stderr, c.want) {
				t.Errorf("%+v", got)
			}
			if readText(t, filepath.Join(home, "VERSION")) == "0.6.0\n" {
				t.Errorf("updated anyway")
			}
		})
	}
	t.Run("bin/hecatron without the binary", func(t *testing.T) {
		home := repoHome(t)
		got := runIn(t, t.TempDir(), []string{"HECATRON_HOME=" + home, "HECATRON_NO_GO=1"}, "bash", filepath.Join(home, "bin", "hecatron"), "self-update")
		if got.status != 1 || !strings.Contains(got.stderr, "self-update needs hecatron's binary") {
			t.Errorf("%+v", got)
		}
	})
	t.Run("usage", func(t *testing.T) {
		home := fakeInstall(t)
		hct := filepath.Join(home, "bin", ".hecatron-go")
		for _, args := range [][]string{{"--bogus"}, {"--version"}, {"--version", "1.2"}, {"--check", "--version", "0.6.0"}} {
			if got := runIn(t, t.TempDir(), updateEnv(t, home, server), append([]string{hct, "self-update"}, args...)...); got.status != 2 {
				t.Errorf("%v: %+v", args, got)
			}
		}
		if got := runIn(t, t.TempDir(), updateEnv(t, home, server), hct, "self-update", "help"); got.status != 0 || !strings.HasPrefix(got.stdout, "Usage: hecatron self-update") {
			t.Errorf("help: %+v", got)
		}
	})
}

// waitFor polls until ok, for the background check.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The background check runs at most once a day, never in CI or when
// turned off, and its result reaches hct status in every format.
func TestUpdateCheck(t *testing.T) {
	server := newFakeGitHub(t)
	home := fakeInstall(t)
	hct := filepath.Join(home, "bin", ".hecatron-go")
	env := updateEnv(t, home, server, "GITHUB_TOKEN=secret-for-test")
	cache := func() selfupdate.Cache { return selfupdate.ReadCache(home) }

	got := runIn(t, t.TempDir(), env, hct, "--version")
	if got.status != 0 || got.stdout != "0.5.0\n" || got.stderr != "" {
		t.Fatalf("--version: %+v", got)
	}
	waitFor(t, "the background check", func() bool { return cache().Latest == "0.6.0" })
	if server.latestChecks() != 1 || server.lastToken() != "Bearer secret-for-test" {
		t.Errorf("checks %d, Authorization %q", server.latestChecks(), server.lastToken())
	}

	// Within a day: no check. Stderr isn't a terminal: no notice.
	for i := 0; i < 3; i++ {
		got = runIn(t, t.TempDir(), env, hct, "--version")
		if got.stderr != "" {
			t.Errorf("a notice without a terminal: %q", got.stderr)
		}
	}
	time.Sleep(300 * time.Millisecond)
	if n := server.latestChecks(); n != 1 {
		t.Errorf("checked %d times within a day", n)
	}

	// A day later it checks again, unless in CI or turned off.
	stale := func() {
		if err := selfupdate.WriteCache(home, selfupdate.Cache{Checked: time.Now().Add(-25 * time.Hour), Latest: "0.6.0"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, off := range []string{"CI=true", "HECATRON_NO_UPDATE_CHECK=1"} {
		stale()
		runIn(t, t.TempDir(), append(append([]string(nil), env...), off), hct, "--version")
		time.Sleep(300 * time.Millisecond)
		if n := server.latestChecks(); n != 1 || !cache().Due(time.Now()) {
			t.Errorf("%s: checked (%d)", off, n)
		}
	}
	stale()
	runIn(t, t.TempDir(), env, hct, "--version")
	waitFor(t, "the next day's check", func() bool { return server.latestChecks() == 2 && !cache().Due(time.Now()) })

	// A dev build never checks.
	writeFiles(t, home, map[string]string{"bin/.hecatron-go.commit": "dev\n"})
	stale()
	runIn(t, t.TempDir(), env, hct, "--version")
	time.Sleep(300 * time.Millisecond)
	if n := server.latestChecks(); n != 2 {
		t.Errorf("a dev build checked")
	}
	writeFiles(t, home, map[string]string{"bin/.hecatron-go.commit": mustGit(t, home, "rev-parse", "HEAD") + "\n"})

	// hct status says what the cache knows, in every format.
	if err := selfupdate.WriteCache(home, selfupdate.Cache{Checked: time.Now(), Latest: "0.6.0"}); err != nil {
		t.Fatal(err)
	}
	root := fixture(t, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": []}`})
	got = runIn(t, root, env, hct, "status", "--output", "json")
	var s struct {
		Version string `json:"version"`
		Update  *struct{ Latest, Command string }
		Next    []struct{ Command string }
	}
	if err := json.Unmarshal([]byte(got.stdout), &s); err != nil || s.Update == nil || s.Update.Latest != "0.6.0" || s.Version != "0.5.0" {
		t.Fatalf("status json: %+v (%v)", got, err)
	}
	if last := s.Next[len(s.Next)-1]; last.Command != "hct self-update" {
		t.Errorf("next: %+v", s.Next)
	}
	for _, format := range []string{"agent", "human"} {
		got = runIn(t, root, env, hct, "status", "--output", format)
		if !strings.Contains(got.stdout, "0.6.0 is available: hct self-update") {
			t.Errorf("status %s: %+v", format, got)
		}
	}
	if err := selfupdate.WriteCache(home, selfupdate.Cache{Checked: time.Now(), Latest: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	got = runIn(t, root, env, hct, "status", "--output", "json")
	if !strings.Contains(got.stdout, `"update": null`) {
		t.Errorf("status json, up to date: %s", got.stdout)
	}
}

// With stderr a terminal, the notice follows a command in human output
// only. script(1) gives the command a terminal.
func TestUpdateNoticeOnATerminal(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil || runtime.GOOS != "linux" {
		t.Skip("needs util-linux's script")
	}
	server := newFakeGitHub(t)
	home := fakeInstall(t)
	hct := filepath.Join(home, "bin", ".hecatron-go")
	if err := selfupdate.WriteCache(home, selfupdate.Cache{Checked: time.Now(), Latest: "0.6.0"}); err != nil {
		t.Fatal(err)
	}
	notice := "A new release of hecatron is available: 0.5.0 → 0.6.0. Run `hct self-update`."
	for _, c := range []struct {
		command string
		env     []string
		shown   bool
	}{
		{"--version", nil, true},
		{"help", nil, true},
		{"--version", []string{"CI=true"}, false},
		{"--version", []string{"HECATRON_NO_UPDATE_CHECK=1"}, false},
		{"--version", []string{"HECATRON_OUTPUT=agent"}, false},
		{"--version", []string{"HECATRON_OUTPUT=json"}, false},
		{"--version", []string{"GITHUB_ACTIONS=true"}, false},
		{"status --output agent", nil, false},
		{"status --output=junit", nil, false},
		{"self-update --check", nil, false},
	} {
		t.Run(c.command+" "+strings.Join(c.env, " "), func(t *testing.T) {
			got := runIn(t, t.TempDir(), updateEnv(t, home, server, c.env...), "script", "-qec", hct+" "+c.command, "/dev/null")
			if shown := strings.Contains(got.stdout, notice); shown != c.shown {
				t.Errorf("notice shown %v: %+v", shown, got)
			}
		})
	}
}
