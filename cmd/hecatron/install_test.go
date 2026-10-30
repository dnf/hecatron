package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// install and uninstall print what manage.py prints, with the same status,
// and their scripts write the same files: on a project whose languages are
// fixture definitions with stub install.sh, uninstall.sh, installed.sh and
// deps hooks (which only write into the project).
var installDefinitions = map[string]string{
	"ok/config.json": `{"abbreviation": "o", "dependency": "sh"}`,
	"ok/install.sh": `echo "ok: ${HECATRON_PROJECT} [${HECATRON_COMPONENTS}] ${HECATRON_IMPL_PATH} from ${PWD#"${HECATRON_ROOT}"}"
[ -n "${HECATRON_DEFER_DEPS:-}" ] && echo ok >> "${HECATRON_DEFER_DEPS}" && echo "ok: deferring"
[ -z "${HECATRON_NO_DEPS:-}" ] || echo "ok: no deps"
printf '2.0.1\t/opt/v20\n' > "${HECATRON_ROOT}/.installed-ver"
echo "ok to stderr" >&2`,
	"ok/uninstall.sh":     `echo "uninstalling ok"; touch "${HECATRON_ROOT}/.uninstalled-ok"`,
	"ok/hooks/deps.sh":    `echo "deps for ok in ${PWD#"${HECATRON_ROOT}"}"`,
	"bad/config.json":     `{"dependency": "sh"}`,
	"bad/install.sh":      `[ -n "${HECATRON_DEFER_DEPS:-}" ] && echo bad >> "${HECATRON_DEFER_DEPS}"; echo "bad fails" >&2; exit 3`,
	"bad/uninstall.sh":    `echo "bad uninstall fails"; exit 4`,
	"bad/hooks/deps.sh":   `echo "never: bad failed to install"`,
	"none/config.json":    `{"dependency": "sh"}`,
	"defer/config.json":   `{"dependency": "sh"}`,
	"defer/install.sh":    `[ -n "${HECATRON_DEFER_DEPS:-}" ] && printf 'defer\nok defer\n' >> "${HECATRON_DEFER_DEPS}"; echo "defer installed"`,
	"defer/hooks/deps.sh": `echo "deps for defer fail"; exit 1`,
	"ver/config.json":     `{"dependency": "sh", "version": "=2.0", "path": ["{home}/bin"], "env": {"VER_HOME": "{home}"}}`,
	"ver/releases.json":   `{"releases": [["1.0", "2019-01-01"], ["2.0", "2026-01-01"]]}`,
	"ver/installed.sh":    `[ -f "${HECATRON_ROOT}/.installed-ver" ] && cat "${HECATRON_ROOT}/.installed-ver"; printf '1.9\t/opt/v19\n'`,
	"ver/install.sh":      `echo "ver: ${VER_HOME:-unset} ${PATH%%:*}"`,
	"ver/uninstall.sh":    `echo "ver: ${VER_HOME:-unset}"`,
	"sig/config.json":     `{"dependency": "sh"}`,
	"sig/install.sh":      `echo "sig install"; kill -TERM $$`,
	"sig/uninstall.sh":    `kill -KILL $$`,
	"spare/config.json":   `{"dependency": "sh"}`,
	"spare/install.sh":    `echo "never: not added"`,
}

var installArgs = [][]string{
	{"install", "ok"}, {"install", "o", "ver"}, {"install", "ver"}, {"install", "bad", "ok"}, {"install", "none"},
	{"install", "nope"}, {"install", "spare"}, {"install", "ok", "nope"}, {"install", "sig"}, {"install", "OK"},
	{"install", "help"}, {"install", "--x"}, {"install", "ok", "--", "ver"}, {"install", "--no-deps", "ok"},
	{"uninstall"}, {"uninstall", "ok"}, {"uninstall", "ok", "bad", "ver"}, {"uninstall", "none"}, {"uninstall", "sig"},
	{"uninstall", "ver"}, {"uninstall", "spare"}, {"uninstall", "--help"},
	{"install"}, {"install", "--no-deps"},
}

func TestInstallMatchesBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	home := repoHome(t)
	definitions := filepath.Join(t.TempDir(), "languages")
	for name, content := range installDefinitions {
		path := filepath.Join(definitions, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".hecatron/config.json": `{"schema": 1, "project": "inst", "components": ["a", "b"],
			"languages": ["ok", "bad", "none", "defer", "ver", "sig"]}`,
		"a/ok/x": "", "b/ok/x": "", "a/defer/x": "", "a/bad/x": "", "sub/x": "",
	}
	projects := map[string]map[string]string{"stubs": files,
		"passing": merge(files, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": ["ver", "ok"]}`}),
		"deps":    merge(files, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": ["defer", "ok", "none"]}`}),
		"nothing": {".hecatron/config.json": `{"schema": 1}`, "sub/x": ""},
	}
	compared := 0
	for name, files := range projects {
		t.Run(name, func(t *testing.T) {
			env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=" + definitions, "HECATRON_NO_GO=1",
				"HECATRON_AS_OF=off", "HECATRON_OUTPUT=human", "PATH=/usr/bin:/bin", "TMPDIR=" + t.TempDir()}
			roots := [2]string{repoFixture(t, files), repoFixture(t, files)}
			for _, args := range installArgs {
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
	t.Logf("%d install and uninstall invocations compared with bin/hecatron", compared)
}
