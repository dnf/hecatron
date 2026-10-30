package status

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dnf/hecatron/internal/project"
)

// Golden outputs, per format, of status on fixture repositories: a saved
// config with components, a toolchain installed and one missing, a history
// with failures and changes in one language; and a git repository read as
// detected. go test ./internal/status -update rewrites them.

var update = flag.Bool("update", false, "rewrite the golden files")

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

var definitions = map[string]string{
	"alpha/config.json": `{"dependency": "sh", "version": "=1.2.3", "path": ["{home}/bin"],
		"pinFiles": [{"file": ".alpha-version", "format": "python-version"}]}`,
	"alpha/installed.sh": `printf '1.2.3\t%s/tc/alpha\n1.1.0\t%s/tc/alpha11\n' "$PWD" "$PWD"`,
	"beta/config.json":   `{"dependency": "no-such-tool-for-hct", "version": "=2.0"}`,
	"gamma/config.json":  `{}`,
	"go/config.json":     `{"dependency": "sh", "detect": ["go.mod"], "version": "=1.27.3", "pinFiles": [{"file": "go.mod", "format": "go-mod-toolchain"}]}`,
}

type fixtureRepo struct {
	files  map[string]string
	ages   map[string]time.Duration // file: how long before now it was last written
	edit   map[string]string        // written after the commit: uncommitted changes
	config bool                     // false: read as detected
}

var repos = map[string]fixtureRepo{
	"saved": {config: true, files: map[string]string{
		".hecatron/config.json":   `{"schema": 1, "components": ["a", "b"], "languages": ["alpha", "beta", "gamma"]}`,
		".hecatron/local.json":    `{"disabled": ["gamma"]}`,
		".gitignore":              ".logs/\n.hecatron/local.json\n",
		"a/alpha/x":               "x\n",
		"b/alpha/.alpha-version":  "1.1\n",
		"a/beta/y":                "",
		"a/gamma/z":               "",
		"tc/alpha/bin/.keep":      "",
		"tc/alpha11/bin/.keep":    "",
		".logs/test/history.log":  "alpha=20:0 beta=31:0\nalpha=123:1 beta=40:0\n",
		".logs/test/alpha.log":    "=== test a (alpha) ===\nalpha: test failed: a\n",
		".logs/test/beta.log":     "beta: test passed for 1 component(s)\n",
		".logs/lint/alpha.log":    "alpha: lint passed for 2 component(s)\n",
		".logs/build/beta.log":    "no summary line\n",
		".logs/build/history.log": "beta=950:2\n",
		".logs/docs/beta.log":     "beta: docs skipped: no docs tool for beta\n",
		".logs/lint/beta.log":     "beta: lint skipped for 1 component(s): a (no linter)\n",
		".logs/audit/beta.log":    "beta: audit skipped for 1 component(s): b (nothing to audit)\nbeta: audit passed for 1 component(s)\n",
		"README.md":               "",
	}, ages: map[string]time.Duration{
		".logs/test/history.log": 2 * time.Hour, ".logs/test/alpha.log": 2*time.Hour + time.Second,
		".logs/test/beta.log": 2*time.Hour + time.Second, ".logs/lint/alpha.log": 30 * time.Minute,
		".logs/build/beta.log": 50 * time.Hour, ".logs/build/history.log": 50 * time.Hour,
		".logs/docs/beta.log": time.Hour, ".logs/lint/beta.log": time.Hour, ".logs/audit/beta.log": time.Hour,
	}, edit: map[string]string{"a/alpha/x": "changed\n", "a/alpha/new": "", "README.md": "changed\n"}},
	"detected": {files: map[string]string{
		"go/go.mod": "module x\n\ntoolchain go1.22.3\n", "go/main.go": "package main\n",
	}, edit: map[string]string{"go/main.go": "package main // changed\n"}},
}

func TestGolden(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture toolchains are shell scripts")
	}
	for _, tool := range []string{"git", "python3", "bash"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home, _ := filepath.Abs("../..")
	builtin := filepath.Join(t.TempDir(), "languages")
	write(t, builtin, definitions)
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", builtin)
	t.Setenv("HECATRON_AS_OF", "off")
	for name, repo := range repos {
		t.Run(name, func(t *testing.T) {
			root, _ := filepath.EvalSymlinks(t.TempDir())
			root = filepath.Join(root, "repo")
			write(t, root, repo.files)
			git(t, root, "init", "-q")
			git(t, root, "add", "-A")
			git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "x")
			write(t, root, repo.edit)
			for file, age := range repo.ages {
				when := now.Add(-age)
				if err := os.Chtimes(filepath.Join(root, file), when, when); err != nil {
					t.Fatal(err)
				}
			}
			p := load(t, home, root, repo.config)
			env := []string{"PATH=/usr/bin:/bin", "HOME=" + root}
			s, err := Gather(p, env, now)
			if err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"agent", "human", "json"} {
				var out bytes.Buffer
				if err := Write(&out, s, format); err != nil {
					t.Fatal(err)
				}
				got := strings.ReplaceAll(out.String(), root, "/repo")
				got = strings.ReplaceAll(got, p.Home.Version(), "VERSION")
				golden := filepath.Join("testdata", name+"."+format)
				if *update {
					if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatal(err)
				}
				if got != string(want) {
					t.Errorf("%s:\n got:\n%s\nwant:\n%s", golden, got, want)
				}
			}
		})
	}
}

// load reads the project as the CLI does; a detected one from the cache
// project.py writes when it finds the root.
func load(t *testing.T, home, root string, saved bool) *project.Project {
	t.Helper()
	if !saved {
		cmd := exec.Command("python3", filepath.Join(home, "libexec", "lib", "project.py"), "root")
		cmd.Dir, cmd.Env = root, append(os.Environ(), "HECATRON_HOME="+home)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("project.py root: %v\n%s", err, out)
		}
		p, err := project.LoadDetected(root, project.NewHome(home))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p, err := project.Load(root, project.NewHome(home))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// A workspace at the root (durabuilt's Cargo workspace, Rust at ".")
// claims only Rust's files: a crash dump or a README beside its crates is
// another change, and an agent's worktree or another clone below the root
// isn't the project's at all, whether git lists it as one directory (a
// repository it can read) or file by file (a worktree whose .git file
// points nowhere here).
func TestRootComponentClaimsItsLanguagesFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	builtin := filepath.Join(t.TempDir(), "languages")
	write(t, builtin, map[string]string{
		"go/config.json":   `{"detect": ["go.mod"]}`,
		"rust/config.json": `{"detect": ["Cargo.toml"], "sourceFiles": ["*.rs", "Cargo.lock"]}`,
	})
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", builtin)
	t.Setenv("HECATRON_AS_OF", "off")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(builtin, "no-gitconfig"))
	cases := []struct {
		name                string
		edit                map[string]string
		nested              []string // directories made git repositories
		rust, golang, other []string
	}{
		{"stray files and nested repositories", map[string]string{
			".claude/worktrees/x/.git":        "gitdir: /nowhere/.git/worktrees/x\n",
			".claude/worktrees/x/src/main.rs": "",
			".claude/worktrees/y/src/lib.rs":  "",
			"erl_crash.dump":                  "",
			"crates/a/README.md":              "x\n",
			"clients/go/notes.txt":            "",
		}, []string{".claude/worktrees/y"}, nil, []string{"clients/go/notes.txt"}, []string{"crates/a/README.md", "erl_crash.dump"}},
		{"a crate's source", map[string]string{"crates/a/src/lib.rs": "pub fn a() {}\n", "Cargo.lock": "x\n"}, nil,
			[]string{"Cargo.lock", "crates/a/src/lib.rs"}, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, _ := filepath.EvalSymlinks(t.TempDir())
			root = filepath.Join(root, "repo")
			write(t, root, map[string]string{
				".hecatron/config.json": `{"schema": 1, "components": ["."], "languages": ["go", "rust"],
					"implPath": "clients/{lang}", "implPaths": {"rust": "."}}`,
				"Cargo.toml":          "[workspace]\nmembers = [\"crates/*\"]\n",
				"Cargo.lock":          "",
				"crates/a/Cargo.toml": "[package]\n",
				"crates/a/src/lib.rs": "",
				"crates/a/README.md":  "",
				"clients/go/go.mod":   "module x\n",
			})
			git(t, root, "init", "-q")
			git(t, root, "add", "-A")
			git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "x")
			write(t, root, c.edit)
			for _, dir := range c.nested {
				git(t, filepath.Join(root, dir), "init", "-q")
			}
			p, err := project.Load(root, project.NewHome(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			s, err := Gather(p, []string{"PATH=/usr/bin:/bin", "HOME=" + root}, now)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string][]string{"other": s.OtherChanges}
			for _, l := range s.Languages {
				got[l.Name] = l.Changes
			}
			for name, want := range map[string][]string{"rust": c.rust, "go": c.golang, "other": c.other} {
				if strings.Join(got[name], " ") != strings.Join(want, " ") {
					t.Errorf("%s changes %q, want %q", name, got[name], want)
				}
			}
		})
	}
}
