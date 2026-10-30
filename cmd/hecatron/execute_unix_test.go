//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
