package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// doctor prints what doctor.sh prints, with the same status, and its
// fixes write the same files: on a project whose languages are fixture
// definitions with stub doctor.sh, install.sh and installed.sh scripts
// (the installers only write into the project).
var doctorDefinitions = map[string]string{
	"ok/config.json":     `{"abbreviation": "o", "dependency": "sh"}`,
	"ok/doctor.sh":       `exit 0`,
	"broken/config.json": `{"dependency": "sh"}`,
	"broken/doctor.sh": `[ -f .fixed-broken ] && exit 0
printf 'thing broken\n\n  indented\n\n\n'; echo "to stderr" >&2; exit 1`,
	"broken/install.sh":   `echo "installing broken from $(pwd)"; pwd > "${HECATRON_ROOT}/.fixed-broken"; echo "installer stderr" >&2; exit 3`,
	"nodep/config.json":   `{"dependency": "no-such-command-for-hct"}`,
	"nodep/install.sh":    `echo "would install"`,
	"nodep/doctor.sh":     `echo "never runs"`,
	"lint/config.json":    `{"dependency": "sh", "lintDependency": "no-such-linter-for-hct"}`,
	"builtin/config.json": `{"dependency": "printf", "lintDependency": "[["}`,
	"ver/config.json": `{"dependency": "sh", "version": "=2.0", "path": ["{home}/bin"], "env": {"VER_HOME": "{home}"},
		"pinFiles": [{"file": ".ver", "format": "python-version"}],
		"supportFiles": [{"file": "support", "format": "python-version"}]}`,
	"ver/releases.json":  `{"releases": [["1.0", "2019-01-01"], ["1.5", "2020-01-01"], ["2.0", "2026-01-01"]]}`,
	"ver/installed.sh":   `[ -f "${HECATRON_ROOT}/.installed-ver" ] && cat "${HECATRON_ROOT}/.installed-ver"; printf '1.9\t/opt/v19\n'`,
	"ver/install.sh":     `printf '2.0.1\t/opt/v20\n1.5\t/opt/v15\n' > "${HECATRON_ROOT}/.installed-ver"`,
	"ver/doctor.sh":      `echo "env: ${HECATRON_PROJECT} ${HECATRON_COMPONENTS} ${VER_HOME:-unset}" >&2`,
	"noinst/config.json": `{"dependency": "sh"}`,
	"noinst/doctor.sh":   `echo "needs a fix"`,
	"badver/config.json": `{"dependency": "sh", "version": "=x"}`,
	"noted/config.json":  `{"dependency": "sh"}`,
	"noted/doctor.sh":    `echo "note: a/noted on 27"; printf 'note: b/noted on 25\n\n'`,
	"mixed/config.json":  `{"dependency": "sh"}`,
	"mixed/doctor.sh":    `echo "note: paired"; echo "really broken"; echo "note:no space, so a problem"; echo "note: "; exit 1`,
	"off/config.json":    `{"dependency": "sh"}`,
	"off/doctor.sh":      `echo "disabled, so unchecked"`,
}

var doctorArgs = [][]string{
	{"doctor"}, {"doctor", "ok", "broken"}, {"doctor", "o"}, {"doctor", "ok", "ok"}, {"doctor", "nodep", "lint", "builtin"},
	{"doctor", "lint"}, {"doctor", "--fix", "lint"},
	{"doctor", "ver"}, {"doctor", "badver"}, {"doctor", "badver", "ok"}, {"doctor", "nope"}, {"doctor", "zig"},
	{"doctor", "help"}, {"doctor", "--help", "ok"}, {"doctor", "ok", "help"}, {"doctor", "-x"}, {"doctor", "off"},
	{"doctor", "--fix", "noinst"}, {"doctor", "--fix", "ok", "broken", "nodep"}, {"doctor", "broken"},
	{"doctor", "--fix", "ver"}, {"doctor", "ver"}, {"doctor", "--fix"}, {"doctor", "--fix", "--no-deps", "broken"},
	{"doctor", "noted"}, {"doctor", "noted", "mixed"}, {"doctor", "--fix", "noted"}, {"doctor", "--fix", "mixed"},
	{"doctor", "--output", "human", "ok"}, {"doctor", "--output=agent", "ok"}, {"doctor", "--output", "xml"}, {"doctor", "--output"},
}

// doctorWants are what some invocations print, beyond matching
// bin/hecatron: a doctor.sh's "note: " lines are shown, but only its other
// lines are problems.
var doctorWants = map[string]result{
	"doctor noted": {status: 0, stdout: "  ok    noted\n          a/noted on 27\n          b/noted on 25\n\nAll 1 toolchains look healthy.\n"},
	"doctor noted mixed": {status: 1, stdout: "  ok    noted\n          a/noted on 27\n          b/noted on 25\n" +
		"  FAIL  mixed\n          really broken\n          note:no space, so a problem\n          paired\n\n" +
		"1 of 2 need attention: mixed\n"},
	"doctor --fix noted": {status: 0, stdout: "  ok    noted\n"},
	// A missing lint tool is a note (#254), so --fix runs no installer.
	"doctor lint": {status: 0, stdout: "  ok    lint\n          lint tool no-such-linter-for-hct not found: " +
		"hct lint lint needs it (hct install lint installs it)\n\nAll 1 toolchains look healthy.\n"},
	"doctor --fix lint": {status: 0, stdout: "  ok    lint\n"},
}

func TestDoctorMatchesBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	home := repoHome(t)
	definitions := filepath.Join(t.TempDir(), "languages")
	for name, content := range doctorDefinitions {
		path := filepath.Join(definitions, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".hecatron/config.json": `{"schema": 1, "project": "doc", "components": ["a", "b"],
			"languages": ["ok", "broken", "nodep", "lint", "builtin", "ver", "noinst", "off", "noted", "mixed"]}`,
		".hecatron/local.json": `{"disabled": ["off"]}`,
		"a/ok/x":               "", "a/broken/x": "", "a/ver/.ver": "1.5\n", "b/ver/support": "1.0\n", "b/ver/x": "",
		"sub/x": "",
	}
	projects := map[string]map[string]string{"stubs": files,
		"bad version": merge(files, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": ["ok", "badver"]}`}),
		"nothing":     {".hecatron/config.json": `{"schema": 1}`, "sub/x": ""},
	}
	compared := 0
	for name, files := range projects {
		for _, asOf := range []string{"off", "2021-06-01"} {
			t.Run(name+"/"+asOf, func(t *testing.T) {
				env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=" + definitions, "HECATRON_NO_GO=1",
					"HECATRON_AS_OF=" + asOf, "PATH=/usr/bin:/bin"}
				roots := [2]string{repoFixture(t, files), repoFixture(t, files)}
				for _, args := range doctorArgs {
					var results [2]result
					for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
						results[i] = runIn(t, filepath.Join(roots[i], "sub"), env, append(entry, args...)...)
						results[i].stdout = strings.ReplaceAll(results[i].stdout, roots[i], "ROOT")
						results[i].stderr = strings.ReplaceAll(results[i].stderr, roots[i], "ROOT")
					}
					if results[0] != results[1] {
						t.Errorf("%q:\nbin/hecatron: %+v\nGo:           %+v", args, results[0], results[1])
					}
					if want, ok := doctorWants[strings.Join(args, " ")]; ok && name == "stubs" &&
						(results[1].status != want.status || !strings.Contains(results[1].stdout, want.stdout)) {
						t.Errorf("%q: want status %d and output containing\n%s\ngot %+v", args, want.status, want.stdout, results[1])
					}
					if diff := compareTrees(t, roots[0], roots[1]); diff != "" {
						t.Errorf("%q: the files differ: %s", args, diff)
					}
					compared++
				}
			})
		}
	}
	t.Logf("%d doctor invocations compared with bin/hecatron", compared)
}

// --output json is the binary's alone: the report as JSON on stdout.
func TestDoctorJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("doctor.sh needs bash")
	}
	home := repoHome(t)
	definitions := filepath.Join(t.TempDir(), "languages")
	for name, content := range doctorDefinitions {
		path := filepath.Join(definitions, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root := repoFixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "languages": ["ok", "mixed"]}`, "a/ok/x": "",
	})
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=" + definitions, "HECATRON_AS_OF=off", "PATH=/usr/bin:/bin"}
	r := runIn(t, root, env, binary, "doctor", "--output", "json", "ok", "mixed")
	var report struct {
		Healthy   bool
		Languages []doctorResult
	}
	if err := json.Unmarshal([]byte(r.stdout), &report); err != nil {
		t.Fatalf("stdout isn't JSON: %v\n%+v", err, r)
	}
	want := []doctorResult{
		{Language: "ok", Healthy: true, Problems: []string{}, Notes: []string{}},
		{Language: "mixed", Problems: []string{"really broken", "note:no space, so a problem"}, Notes: []string{"paired"}},
	}
	if r.status != 1 || report.Healthy || !reflect.DeepEqual(report.Languages, want) {
		t.Errorf("got status %d, %+v; want 1, %+v", r.status, report, want)
	}
	if !strings.Contains(r.stderr, "1 of 2 need attention") {
		t.Errorf("the human report should go to stderr: %q", r.stderr)
	}
}
