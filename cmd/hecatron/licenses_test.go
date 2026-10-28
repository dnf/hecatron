package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hct licenses end to end: the fixture projects (tests/licenses/fixtures)
// as one project, a stand-in osv-scanner on PATH answering with the
// captured reports, and the exit statuses.

const fakeOSVScanner = `#!/usr/bin/env bash
for dir; do :; done
name="$(basename "${dir}")"
report="${CAPTURED}/licenses-${name}.json"
[ -f "${report}" ] || exit 128
sed "s|/tmp/licenses-fixtures/${name}|${dir}|g" "${report}"
exit 1
`

func licensesProject(t *testing.T, config string) (string, []string) {
	t.Helper()
	home := repoHome(t)
	files := map[string]string{".hecatron/config.json": config, "zig/build.zig": "", "web/package.json": "{}"}
	for _, lang := range []string{"typescript", "python", "dart"} {
		dir := filepath.Join(home, "tests", "licenses", "fixtures", lang)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files[lang+"/"+entry.Name()] = string(data)
		}
	}
	root := fixture(t, files)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "osv-scanner"), []byte(fakeOSVScanner), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=", "CLAUDECODE=", "GITHUB_ACTIONS=",
		"CAPTURED=" + filepath.Join(home, "tests", "output", "osv-scanner", "2.7.0"), "PATH=" + bin + ":" + os.Getenv("PATH")}
	return root, env
}

const licensesConfig = `{"schema": 1, "project": "lic", "languages": ["dart", "python", "typescript", "zig"],
	"implPaths": {"typescript": "typescript", "python": "python", "dart": "dart", "zig": "zig"}%s}`

func TestLicensesCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in osv-scanner is bash")
	}
	root, env := licensesProject(t, strings.Replace(licensesConfig, "%s", "", 1))
	got := runIn(t, root, env, binary, "licenses", "--output", "agent")
	if got.status != 0 || !strings.HasPrefix(got.stdout, "hct licenses: 20 packages, ") ||
		!strings.Contains(got.stdout, "NOT COVERED dart: ") || !strings.Contains(got.stdout, "NOT COVERED zig: ") {
		t.Errorf("no policy: %+v", got)
	}
	json := runIn(t, root, env, binary, "licenses", "--json", "ts")
	if json.status != 0 || !strings.Contains(json.stdout, `"name": "caniuse-lite"`) || strings.Contains(json.stdout, `"PyPI"`) {
		t.Errorf("--json ts: %+v", json)
	}
	for _, args := range [][]string{{"licenses", "nope"}, {"licenses", "--bogus"}, {"licenses", "-c", "zz"}, {"licenses", "--output", "xml"}} {
		if got := runIn(t, root, env, append([]string{binary}, args...)...); got.status != 2 || got.stderr == "" {
			t.Errorf("%v: %+v", args, got)
		}
	}
	if help := runIn(t, root, env, binary, "licenses", "help"); help.status != 0 || !strings.HasPrefix(help.stdout, "Usage: hecatron licenses") {
		t.Errorf("help: %+v", help)
	}
	// hct puts hecatron's bin directory on PATH, where an installed
	// osv-scanner would be: an empty one.
	noScanner := append(append([]string{}, env...), "PATH=/usr/bin:/bin", "XDG_DATA_HOME="+t.TempDir())
	if got := runIn(t, root, noScanner, binary, "licenses", "python"); got.status != 3 ||
		!strings.Contains(got.stderr, "OSV-Scanner isn't installed; hecatron install python installs it with the toolchain, or see ") {
		t.Errorf("without osv-scanner: %+v", got)
	}
	if got := runIn(t, root, noScanner, binary, "licenses", "ts", "dart", "python"); got.status != 3 ||
		!strings.Contains(got.stderr, "hecatron install typescript installs it with the toolchain (as does installing python), or see ") {
		t.Errorf("without osv-scanner, several languages: %+v", got)
	}
	if got := runIn(t, root, noScanner, binary, "licenses", "dart", "zig"); got.status != 0 {
		t.Errorf("nothing covered needs no osv-scanner: %+v", got)
	}
}

func TestLicensesPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in osv-scanner is bash")
	}
	policy := `, "licenses": {"deny": ["GPL-3.0"], "ignore": [{"package": "pypi:pyqt5", "reason": "r", "until": "2001-01-01"}]}`
	root, env := licensesProject(t, strings.Replace(licensesConfig, "%s", policy, 1))
	got := runIn(t, root, env, binary, "licenses", "--output", "agent")
	if got.status != 1 || !strings.Contains(got.stdout, "VIOLATION pypi:pyqt5 5.15.10 GPL-3.0-only: GPL-3.0-only denied") ||
		!strings.Contains(got.stdout, "NOTE the accepted package pypi:pyqt5 expired on 2001-01-01") {
		t.Errorf("deny: %+v", got)
	}
	if got := runIn(t, root, env, binary, "licenses", "typescript"); got.status != 0 {
		t.Errorf("typescript alone has nothing denied: %+v", got)
	}
	if check := runIn(t, root, env, binary, "check"); check.status != 0 || strings.Contains(check.stdout, "licenses") {
		t.Errorf("check of a well-formed policy: %+v", check)
	}
	bad := strings.Replace(licensesConfig, "%s", `, "licenses": {"allow": ["Nope"]}`, 1)
	root, env = licensesProject(t, bad)
	check := runIn(t, root, env, binary, "check")
	if check.status == 0 || !strings.Contains(check.stdout, `.hecatron/config.json: licenses.allow: "Nope" isn't an SPDX license identifier`) {
		t.Errorf("check: %+v", check)
	}
}
