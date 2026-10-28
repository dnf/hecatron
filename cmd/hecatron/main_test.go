package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The Go binary must print what bin/hecatron prints, byte for byte, with the
// same exit status: natively for the commands ported, by delegating for the
// rest. Each case runs both in a fixture project.

var binary string // the Go hecatron, built once for the tests

func TestMain(m *testing.M) {
	// bin/hecatron is the scripts these tests compare the binary with; a
	// binary from hecatron self build in the checkout mustn't stand in.
	if err := os.Setenv("HECATRON_NO_GO", "1"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Nor may a run under test write reports into the CI job running it.
	for _, name := range []string{"GITHUB_STEP_SUMMARY", "HECATRON_REPORT_DIR"} {
		_ = os.Unsetenv(name)
	}
	dir, err := os.MkdirTemp("", "hecatron-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "hct")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building hecatron:", err)
		os.Exit(1)
	}
	status := m.Run()
	_ = os.RemoveAll(dir) // a temporary directory
	os.Exit(status)
}

func repoHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return home
}

type result struct {
	stdout, stderr string
	status         int
}

func runIn(t *testing.T, dir string, env []string, argv ...string) result {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	status := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		status = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v", argv, err)
	}
	return result{stdout.String(), stderr.String(), status}
}

// Fixture projects, by name: files relative to the root.
var cliProjects = map[string]map[string]string{
	"components": {
		".hecatron/config.json": `{"schema": 1, "project": "multi", "components": ["age", "date"],
			"implPath": "{component}/impl/{lang}", "languages": ["typescript", "go", "rust", "python", "csharp"],
			"tools": {"shell": {"dependency": "shellcheck"}},
			"tasks": {"test": {"zeta": "true", "alpha": "true"}, "lint": {"style": "true"}}}`,
		".hecatron/local.json":                 `{"disabled": ["rust", "haskell"]}`,
		".hecatron/languages/go/test.sh":       "",
		".hecatron/languages/mine/config.json": `{"abbreviation": "mi"}`,
		".hecatron/languages/mine/build.sh":    "",
		"age/impl/go/go.mod":                   "",
		"date/impl/dotnet/x.csproj":            "",
	},
	"single": {
		".hecatron/config.json": `{"schema": 1, "languages": ["go"]}`,
	},
	"many languages": {
		".hecatron/config.json": `{"schema": 1, "languages": ["ada", "bash", "c", "cpp", "go", "java", "kotlin",
			"python", "ruby", "rust", "scala", "swift", "typescript", "zig"]}`,
	},
	"schema 99": {
		".hecatron/config.json": `{"schema": 99}`,
	},
	"languages a string": { // the Go model declines; project.py reads it
		".hecatron/config.json": `{"schema": 1, "languages": "go"}`,
	},
	"broken definition": {
		".hecatron/config.json":              `{"schema": 1, "languages": ["go"]}`,
		".hecatron/languages/go/config.json": `{"abbreviation": }`,
	},
	"too new": {
		".hecatron/config.json": `{"schema": 1, "hecatron": ">=99.0"}`,
	},
	"no project": {
		"README": "",
	},
}

// cliArgs are the invocations compared: every native form, and delegated
// ones that only read.
var cliArgs = [][]string{
	{}, {"help"}, {"-h"}, {"--help"}, {"help", "list"}, {"help", "agents"},
	{"--version"}, {"-V"}, {"version-of-hecatron"},
	{"list"}, {"list", "languages"}, {"list", "--all"}, {"list", "--all", "languages"}, {"list", "languages", "--all"},
	{"list", "--al"}, {"list", "help"}, {"list", "-h"}, {"list", "nope"}, {"list", "languages", "languages"},
	{"test", "help"}, {"test", "--help"}, {"lint", "help"}, {"build", "help"}, {"clean", "help"},
	{"rebuild", "help"}, {"retest", "help"}, {"audit", "help"}, {"format", "help"}, {"deps", "help"}, {"outdated", "help"}, {"outdated", "--help"}, {"format", "--help"}, {"test", "-c", "nope", "help"},
	{"doctor", "help"}, {"doctor", "--help", "go"},
	{"version"}, {"version", "help"}, {"version", "--help"}, {"version", ""},
	{"env"}, {"env", "go"}, {"env", "go", "-c", "age"},
	{"nope"}, {"plan-not"},
	{"check", "help"}, {"init", "help"}, {"add"}, {"enable"}, {"disable"},
	{"get-started", "--help"}, {"parallel", "-h"},
	{"self"}, {"self", "help"}, {"self", "--help"}, {"self", "bogus"}, {"self-update"},
}

func TestOutputMatchesBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	for _, tool := range []string{"bash", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home := repoHome(t)
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "COLUMNS=80", "NO_COLOR=1"}
	compared := 0
	for name, files := range cliProjects {
		root := fixture(t, files)
		for _, args := range cliArgs {
			t.Run(name+"/"+strings.Join(args, " "), func(t *testing.T) {
				want := runIn(t, root, env, append([]string{"bash", filepath.Join(home, "bin", "hecatron")}, args...)...)
				got := runIn(t, root, env, append([]string{binary}, args...)...)
				if got != want {
					t.Errorf("bin/hecatron %q:\n status %d, stdout:\n%s\n stderr:\n%s\nGo: status %d, stdout:\n%s\n stderr:\n%s",
						args, want.status, want.stdout, want.stderr, got.status, got.stdout, got.stderr)
				}
				compared++
			})
		}
	}
	t.Logf("%d invocations compared with bin/hecatron", compared)
}

// fixture creates a project from files and returns its root, links
// resolved as the commands report it.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The ported commands need neither bash nor Python: run with an empty PATH
// (as on a Windows machine without them).
func TestNativeCommandsNeedNoBashOrPython(t *testing.T) {
	home := repoHome(t)
	root := fixture(t, cliProjects["components"])
	if err := os.MkdirAll(filepath.Join(root, "age", "impl", "go", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "PATH=" + t.TempDir()}
	version, err := os.ReadFile(filepath.Join(home, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args   []string
		status int
		want   []string // lines stdout must contain
	}{
		{[]string{"--version"}, 0, []string{strings.TrimSpace(string(version))}},
		{[]string{"help"}, 0, []string{"Usage: hecatron <command> [arguments]", "hecatron " + strings.TrimSpace(string(version))}},
		{[]string{"list"}, 0, []string{"  go", "  rust  (disabled)", "  typescript", "5 languages"}},
		{[]string{"list", "--all"}, 0, []string{"  go           project", "  mine         project  (not added)", "  zig          built-in  (not added)"}},
		{[]string{"test", "help"}, 0, []string{"Usage: hecatron test [flags] [target ...]", "Project tasks: zeta alpha ", "components: age date", "  --timeout DURATION", "  --changed [REF]"}},
		{[]string{"audit", "help"}, 0, []string{"  --output FORMAT", "  --timeout DURATION"}},
		{[]string{"deps", "help"}, 0, []string{"Fetch each language's dependencies for every component, held to its lock file."}},
		{[]string{"build", "help"}, 0, []string{"Languages (5):", "csharp (cs)     go "}},
		{[]string{"version"}, 2, []string{"Usage: hecatron version <version>"}},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			got := runIn(t, filepath.Join(root, "age", "impl", "go", "sub"), env, append([]string{binary}, c.args...)...)
			if got.status != c.status || got.stderr != "" {
				t.Fatalf("status %d, stderr %q", got.status, got.stderr)
			}
			for _, line := range c.want {
				if !strings.Contains(got.stdout, line) {
					t.Errorf("no %q in:\n%s", line, got.stdout)
				}
			}
		})
	}
}

// Without HECATRON_HOME, the binary's home is the directory above its own
// (an install's bin/hct), or its own (a checkout it was built at the top
// of).
func TestHomeFromBinaryLocation(t *testing.T) {
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range []string{"bin", "."} {
		t.Run(layout, func(t *testing.T) {
			home := t.TempDir()
			files := map[string]string{"VERSION": "9.8.7\n", "libexec/lib/project.py": ""}
			for name, content := range files {
				path := filepath.Join(home, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			copied := filepath.Join(home, layout, filepath.Base(binary))
			if err := os.MkdirAll(filepath.Dir(copied), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(copied, data, 0o755); err != nil {
				t.Fatal(err)
			}
			got := runIn(t, t.TempDir(), []string{"HECATRON_HOME="}, copied, "--version")
			if got.stdout != "9.8.7\n" || got.status != 0 {
				t.Errorf("--version: %q (status %d, stderr %q)", got.stdout, got.status, got.stderr)
			}
		})
	}
}

// Reached through a link, the root is the linked-to directory, as Python's
// os.getcwd() gives it, not the $PWD the shell kept.
func TestRootThroughALinkMatchesBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	home := repoHome(t)
	root := fixture(t, cliProjects["single"])
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "PWD=" + link}
	want := runIn(t, link, env, "bash", filepath.Join(home, "bin", "hecatron"), "env")
	got := runIn(t, link, env, binary, "env")
	if got != want || !strings.Contains(got.stdout, "HECATRON_ROOT="+root+"\n") {
		t.Errorf("bin/hecatron: %+v\nGo:          %+v", want, got)
	}
}

// In a git repository hecatron hasn't set up, a project command reads it as
// detected (project.py's detected_config, reporting on stderr the first
// time), writing nothing in the working tree.
func TestAGitRepositoryWithoutConfigIsReadAsBinHecatronReadsIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	for _, tool := range []string{"bash", "python3", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home := repoHome(t)
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "COLUMNS=80", "NO_COLOR=1"}
	repositories := map[string]map[string]string{
		"new":       {"go/go.mod": "module x\n", "go/main.go": "package main\n", "rust/Cargo.toml": ""},
		"empty":     {"README": ""},
		"schema 99": {".hecatron/config.json": `{"schema": 99}`, "go/go.mod": ""},
	}
	for name, files := range repositories {
		for _, args := range [][]string{{"list"}, {"test", "help"}, {"nope"}, {"init", "help"}} {
			t.Run(name+"/"+strings.Join(args, " "), func(t *testing.T) {
				var results [2]result
				var roots [2]string
				for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
					roots[i] = fixture(t, files)
					sub := filepath.Join(roots[i], "go")
					if err := os.MkdirAll(sub, 0o755); err != nil {
						t.Fatal(err)
					}
					runIn(t, roots[i], nil, "git", "init", "-q")
					runIn(t, roots[i], nil, "git", "add", "-A")
					results[i] = runIn(t, sub, env, append(entry, args...)...)
					results[i].stdout = strings.ReplaceAll(results[i].stdout, roots[i], "ROOT")
					results[i].stderr = strings.ReplaceAll(results[i].stderr, roots[i], "ROOT")
				}
				if results[0] != results[1] {
					t.Errorf("bin/hecatron: %+v\nGo:          %+v", results[0], results[1])
				}
				for _, root := range roots {
					status := runIn(t, root, nil, "git", "status", "--porcelain", "--untracked-files=all")
					if strings.Contains(status.stdout, "??") {
						t.Errorf("wrote into the working tree:\n%s", status.stdout)
					}
				}
			})
		}
	}
}
