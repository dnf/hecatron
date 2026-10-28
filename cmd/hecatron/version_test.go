package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// version runs each language's version.sh as version.sh does, with the
// same output, status and files written: on projects whose languages are
// fixture definitions with stub hooks (which only write into the project).
var versionDefinitions = map[string]string{
	"alpha/config.json": `{"dependency": "sh"}`,
	"alpha/version.sh": `echo "alpha: [$*] from ${PWD#"${HECATRON_ROOT}"} lists:${_ADDED_LANGS:-none}"
env | grep '^HECATRON_' | sort
echo "$1" > "${HECATRON_ROOT}/alpha.version"; echo "alpha to stderr" >&2`,
	"beta/config.json":  `{"dependency": "sh"}`,
	"beta/version.sh":   `echo "never: the project's own version.sh comes first"`,
	"plain/config.json": `{"dependency": "sh"}`,
	"fail/config.json":  `{"dependency": "sh"}`,
	"fail/version.sh":   `echo "fail sets $1"; exit 3`,
	"sig/config.json":   `{"dependency": "sh"}`,
	"sig/version.sh":    `echo "sig"; kill -TERM $$`,
	"spare/config.json": `{"dependency": "sh"}`,
	"spare/version.sh":  `echo "never: not added"`,
}

var versionArgs = [][]string{
	{"version", "1.2.3"}, {"version", "2.0.0", "extra"}, {"version", "with space"}, {"version", "-x"},
	{"version"}, {"version", ""}, {"version", "help"}, {"version", "--help", "1.0"},
}

func TestVersionMatchesBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	home := repoHome(t)
	definitions := filepath.Join(t.TempDir(), "languages")
	for name, content := range versionDefinitions {
		path := filepath.Join(definitions, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".hecatron/config.json": `{"schema": 1, "project": "ver", "components": ["a", "b"],
			"languages": ["alpha", "beta", "plain"]}`,
		".hecatron/languages/beta/version.sh": `echo "beta's own: $1"`,
		"a/alpha/x":                           "", "sub/x": "",
	}
	projects := map[string]map[string]string{"stubs": files,
		"failing":  merge(files, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": ["alpha", "fail", "plain"]}`}),
		"signal":   merge(files, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": ["sig", "alpha"]}`}),
		"disabled": merge(files, map[string]string{".hecatron/local.json": `{"disabled": ["alpha"]}`}),
		"nothing":  {".hecatron/config.json": `{"schema": 1}`, "sub/x": ""},
	}
	compared := 0
	for name, files := range projects {
		t.Run(name, func(t *testing.T) {
			env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=" + definitions, "HECATRON_NO_GO=1",
				"HECATRON_AS_OF=off", "PATH=/usr/bin:/bin"}
			roots := [2]string{repoFixture(t, files), repoFixture(t, files)}
			for _, args := range versionArgs {
				var results [2]result
				for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
					results[i] = runIn(t, filepath.Join(roots[i], "sub"), env, append(entry, args...)...)
					results[i].stdout = strings.ReplaceAll(results[i].stdout, roots[i], "ROOT")
					results[i].stderr = strings.ReplaceAll(results[i].stderr, roots[i], "ROOT")
				}
				results[0].stderr = withoutJobReport(results[0].stderr)
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
	t.Logf("%d version invocations compared with bin/hecatron", compared)
}

// withoutJobReport drops what bash says on stderr when version.sh's hook is
// killed by a signal ("Terminated   bash "${_hook}" ..."), which the Go
// port doesn't say (its wording follows bash's version and the C library).
func withoutJobReport(stderr string) string {
	var kept []string
	for _, line := range strings.SplitAfter(stderr, "\n") {
		if !strings.HasSuffix(line, `bash "${_hook}" "${_version}"`+"\n") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "")
}
