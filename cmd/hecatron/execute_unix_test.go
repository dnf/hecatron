//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// `hecatron exec LANG [-c COMPONENT] -- CMD...` runs CMD with what `hct env`
// gives the component: a stub language whose installed.sh lists two
// toolchains in the project, each a `zz` script saying which it is.
var execDefinitions = map[string]string{
	"zz/config.json": `{"version": "=1.2", "pinFiles": [{"file": ".zz-version", "format": "python-version"}],
		"path": ["{home}/bin"], "env": {"ZZ_HOME": "{home}"}}`,
	"zz/installed.sh": `printf '1.2.0\t%s/tc/zz12\n1.4.1\t%s/tc/zz14\n' "$PWD" "$PWD"`,
}

const execStub = "#!/bin/sh\necho \"zz $ZZ_HOME $(pwd -P) $*\"\nexit ${ZZ_EXIT:-0}\n"

func execFixture(t *testing.T, components string) (root string, env []string) {
	t.Helper()
	root = fixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ` + components + `, "languages": ["zz"]}`,
		"a/zz/sub/x":            "",
		"a/zz/.zz-version":      "1.2\n",
		"b/zz/.zz-version":      "1.4\n",
		"c/zz/.zz-version":      "1.9\n",
		"tc/zz12/bin/zz":        execStub,
		"tc/zz14/bin/zz":        execStub,
	})
	for _, stub := range []string{"tc/zz12/bin/zz", "tc/zz14/bin/zz"} {
		if err := os.Chmod(filepath.Join(root, stub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	definitions := fixture(t, execDefinitions)
	return root, []string{"HECATRON_HOME=" + repoHome(t), "HECATRON_BUILTIN_LANGUAGES=" + definitions,
		"HECATRON_AS_OF=off", "XDG_DATA_HOME=" + t.TempDir(), "HECATRON_NO_UPDATE_CHECK=1"}
}

func TestExec(t *testing.T) {
	root, env := execFixture(t, `["a", "b", "c"]`)
	cases := []struct {
		name, dir string
		env       []string
		args      []string
		status    int
		stdout    string // "" for any
		stderr    string // a part of it
	}{
		{"component named", "", nil, []string{"zz", "-c", "b", "--", "zz", "x", "y"}, 0,
			"zz " + root + "/tc/zz14 " + root + " x y\n", ""},
		{"component from the directory", "a/zz/sub", nil, []string{"zz", "--", "zz"}, 0,
			"zz " + root + "/tc/zz12 " + root + "/a/zz/sub \n", ""},
		{"component from its own directory", "b/zz", nil, []string{"zz", "--", "zz"}, 0,
			"zz " + root + "/tc/zz14 " + root + "/b/zz \n", ""},
		{"exit status", "", []string{"ZZ_EXIT=7"}, []string{"zz", "-c", "a", "--", "zz"}, 7, "", ""},
		{"not found", "", nil, []string{"zz", "-c", "a", "--", "no-such-command-zz"}, 127, "", "no-such-command-zz: command not found"},
		{"not executable", "", nil, []string{"zz", "-c", "a", "--", "./a/zz/sub/x"}, 126, "", "./a/zz/sub/x"},
		{"which component", "", nil, []string{"zz", "--", "zz"}, 2, "", "components in zz: a b c"},
		{"not installed", "", nil, []string{"zz", "-c", "c", "--", "zz"}, 3, "", "c: zz 1.9 not installed (from pin:c/zz/.zz-version); hct install zz installs it"},
		{"unknown component", "", nil, []string{"zz", "-c", "d", "--", "zz"}, 2, "", "unknown component 'd' (components: a b c)"},
		{"unknown language", "", nil, []string{"nope", "-c", "a", "--", "zz"}, 2, "", "unknown language: nope"},
		{"no command", "", nil, []string{"zz", "-c", "a", "--"}, 2, "", "Usage: hecatron exec"},
		{"no separator", "", nil, []string{"zz", "-c", "a", "zz"}, 2, "", "after --"},
		{"help", "", nil, []string{"help"}, 0, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{binary, "exec"}, tc.args...)
			got := runIn(t, filepath.Join(root, tc.dir), append(append([]string(nil), env...), tc.env...), args...)
			if got.status != tc.status || (tc.stdout != "" && got.stdout != tc.stdout) || !strings.Contains(got.stderr, tc.stderr) {
				t.Errorf("hct %q: status %d, stdout %q, stderr %q; want %d, %q, %q",
					args[1:], got.status, got.stdout, got.stderr, tc.status, tc.stdout, tc.stderr)
			}
		})
	}
}

// Standard input reaches the command, and a project with one component
// needs no -c from anywhere in it.
func TestExecOnlyComponentAndStdin(t *testing.T) {
	root, env := execFixture(t, `["a"]`)
	cmd := exec.Command(binary, "exec", "zz", "--", "sh", "-c", `read line; echo "$line $ZZ_HOME"`)
	cmd.Dir, cmd.Env, cmd.Stdin = filepath.Join(root, "a"), append(os.Environ(), env...), strings.NewReader("hello\n")
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "hello "+root+"/tc/zz12\n" {
		t.Errorf("%v: %q", err, out)
	}
}

func TestExecWithoutTheBinary(t *testing.T) {
	root, env := execFixture(t, `["a"]`)
	got := runIn(t, root, append(env, "HECATRON_NO_GO=1"), "bash", filepath.Join(repoHome(t), "bin", "hecatron"), "exec", "zz", "--", "zz")
	// A checkout (as here) is told to build the binary (#258).
	if got.status != 1 || !strings.Contains(got.stderr, `exec needs hecatron's binary, which install.sh downloads with a release; `+
		`in this checkout, run hct self build; without it, eval "$(hct env LANG -c COMPONENT)" first`) {
		t.Errorf("%+v", got)
	}
}

// --output agent keeps the command's output in <logs>/exec/<CMD>.log: one
// line when it passes; its log, error lines and rerun command when it fails,
// with the command's own exit status.
func TestExecForAgent(t *testing.T) {
	root, env := execFixture(t, `["a"]`)
	times := regexp.MustCompile(`\d+(?:\.\d)?s\b`)
	pass := runIn(t, root, env, binary, "exec", "zz", "--output", "agent", "--", "zz", "x")
	if out := times.ReplaceAllString(pass.stdout, "T"); out != "hct exec zz: 1 passed (T)\n" || pass.status != 0 || pass.stderr != "" {
		t.Errorf("a pass: %+v", pass)
	}
	if log, err := os.ReadFile(filepath.Join(root, ".logs/exec/zz.log")); err != nil || string(log) != "zz "+root+"/tc/zz12 "+root+" x\n" {
		t.Errorf("its log: %q, %v", log, err)
	}
	fail := runIn(t, root, append(env, "ZZ_EXIT=7"), binary, "exec", "zz", "--output=agent", "--", "zz", "it's")
	want := "hct exec zz: 0 passed, 1 failed (T)\n" +
		"FAIL zz exit 7, T, log .logs/exec/zz.log\n" +
		"  zz " + root + "/tc/zz12 " + root + " it's\n" +
		"Rerun: hct exec zz --output agent -- zz 'it'\\''s'\n"
	if out := times.ReplaceAllString(fail.stdout, "T"); out != want || fail.status != 7 {
		t.Errorf("a failure: status %d, stdout:\n%s\nwant:\n%s", fail.status, out, want)
	}
	if bad := runIn(t, root, env, binary, "exec", "zz", "--output", "json", "--", "zz"); bad.status != 2 ||
		!strings.Contains(bad.stderr, "--output takes agent or human, not 'json'") {
		t.Errorf("--output json: %+v", bad)
	}
}

// config.json "wrapper" runs every job under a command of the project's:
// hct exec's command, in either output, and a run's hooks and tasks.
func TestWrapper(t *testing.T) {
	root, env := execFixture(t, `["a"]`)
	config := `{"schema": 1, "components": ["a"], "languages": ["zz"], "wrapper": ["tools/wrap.sh", "--tag"],
		"tasks": {"test": {"task": "echo task ran"}}}`
	wrap := "#!/bin/sh\necho \"wrapped $1 in $(pwd -P): $2 $3\"\nshift\nexec \"$@\"\n"
	for name, text := range map[string]string{".hecatron/config.json": config, "tools/wrap.sh": wrap,
		".hecatron/languages/zz/test.sh": "echo hook ran\n"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sub := filepath.Join(root, "a/zz/sub")
	if got := runIn(t, sub, env, binary, "exec", "zz", "--", "zz", "y"); got.status != 0 ||
		got.stdout != "wrapped --tag in "+sub+": zz y\nzz "+root+"/tc/zz12 "+sub+" y\n" {
		t.Errorf("exec: %+v", got)
	}
	if got := runIn(t, sub, env, binary, "exec", "zz", "--output", "agent", "--", "zz"); got.status != 0 {
		t.Errorf("exec --output agent: %+v", got)
	} else if log, err := os.ReadFile(filepath.Join(root, ".logs/exec/zz.log")); err != nil ||
		!strings.HasPrefix(string(log), "wrapped --tag in "+sub+": zz \n") {
		t.Errorf("exec --output agent's log: %q, %v", log, err)
	}
	if got := runIn(t, root, append(env, "HECATRON_OUTPUT=human"), binary, "test", "zz", "task"); got.status != 0 ||
		!strings.Contains(got.stdout, "wrapped --tag in "+root+"/a/zz: bash "+root+"/.hecatron/languages/zz/test.sh\nhook ran\n") ||
		!strings.Contains(got.stdout, "wrapped --tag in "+root+": bash -c\ntask ran\n") {
		t.Errorf("test: %+v", got)
	}
	if got := runIn(t, root, append(env, "HECATRON_NO_GO=1"), "bash", filepath.Join(repoHome(t), "bin", "hecatron"), "test", "zz"); got.status != 2 ||
		!strings.Contains(got.stderr, `config.json's "wrapper" needs hecatron's binary`) {
		t.Errorf("without the binary: %+v", got)
	}
	bad := `{"schema": 1, "languages": ["zz"], "wrapper": [3]}`
	if err := os.WriteFile(filepath.Join(root, ".hecatron/config.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runIn(t, root, env, binary, "exec", "zz", "--", "zz"); got.status == 0 || !strings.Contains(got.stderr, `"wrapper" must be a command`) {
		t.Errorf("a wrapper that isn't a command: %+v", got)
	}
}
