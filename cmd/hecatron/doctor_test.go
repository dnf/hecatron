package main

import (
	"os"
	"path/filepath"
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
	"off/config.json":    `{"dependency": "sh"}`,
	"off/doctor.sh":      `echo "disabled, so unchecked"`,
}

var doctorArgs = [][]string{
	{"doctor"}, {"doctor", "ok", "broken"}, {"doctor", "o"}, {"doctor", "ok", "ok"}, {"doctor", "nodep", "lint", "builtin"},
	{"doctor", "ver"}, {"doctor", "badver"}, {"doctor", "badver", "ok"}, {"doctor", "nope"}, {"doctor", "zig"},
	{"doctor", "help"}, {"doctor", "--help", "ok"}, {"doctor", "ok", "help"}, {"doctor", "-x"}, {"doctor", "off"},
	{"doctor", "--fix", "noinst"}, {"doctor", "--fix", "ok", "broken", "nodep"}, {"doctor", "broken"},
	{"doctor", "--fix", "ver"}, {"doctor", "ver"}, {"doctor", "--fix"},
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
			"languages": ["ok", "broken", "nodep", "lint", "builtin", "ver", "noinst", "off"]}`,
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
