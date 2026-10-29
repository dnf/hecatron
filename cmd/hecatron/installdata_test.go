package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A fake apt: apt-get install records its packages in $FAKE/installed and
// puts a command named like each package in $FAKE/bin; dpkg-query reads
// them back; sudo runs its command. Nothing touches the machine's packages.
var fakeApt = map[string]string{
	"apt-get": `#!/bin/sh
echo "apt-get $*" >> "${FAKE}/log"
case " $* " in
  *" install "*) shift 4; for p in "$@"; do touch "${FAKE}/installed/${p}"; printf '#!/bin/sh\n' > "${FAKE}/bin/${p}"; chmod +x "${FAKE}/bin/${p}"; done ;;
  *" remove "*) shift 2; for p in "$@"; do rm -f "${FAKE}/installed/${p}" "${FAKE}/bin/${p}"; done ;;
esac
`,
	"dpkg-query": `#!/bin/sh
shift 2
for p in "$@"; do [ -e "${FAKE}/installed/${p}" ] && printf '%s\tinstall ok installed\n' "${p}"; done
exit 0
`,
	"sudo": `#!/bin/sh
exec "$@"
`,
}

// The definitions: pkg is installed from data (its install.sh hands over
// to the binary, as Erlang's does), caller's script runs pkg's (as
// Elixir's runs Erlang's), and nodata's install.json has nothing for apt,
// so its script runs, asks the binary, is declined and carries on.
var dataDefinitions = map[string]string{
	"pkg/config.json": `{"dependency": "pkg-a"}`,
	"pkg/install.json": `{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["pkg-a"]}},
		"tools": [{"name": "pkg-tool", "when": {"glob": "*.tool", "because": "a component has .tool files"},
		           "packages": {"apt": ["pkg-tool"]}, "check": {"command": "pkg-tool"}}]}`,
	"pkg/install.sh": `source "$(dirname "$0")/../_common.sh"
install_from_data pkg
echo "pkg's script" >> "${HECATRON_ROOT}/.scripts"
`,
	"pkg/doctor.sh":      `echo "never: pkg's checks come from its data"`,
	"caller/config.json": `{"dependency": "sh"}`,
	"caller/install.sh": `echo "caller's script" >> "${HECATRON_ROOT}/.scripts"
bash "$(dirname "$0")/../pkg/install.sh"
`,
	"zz/config.json": `{"dependency": "zz", "version": "=1.0.0", "path": ["{home}"],
		"pinFiles": [{"file": ".zz-version", "format": "python-version"}]}`,
	"zz/releases.json": `{"releases": [["0.9.1", "2025-01-01"], ["0.9.2", "2025-02-01"], ["1.0.0", "2026-01-01"]]}`,
	"zz/installed.sh":  `exit 0`,
	"zz/install.json": `{"schema": 1, "toolchain": {"versions": "managed", "archive": {
		"url": "https://example.com/zz-{version}-{os}-{arch}.tar.gz", "os": {"linux": "linux"},
		"arch": {"x86_64": "x86_64", "aarch64": "aarch64"}, "checksum": "{url}.sha256", "check": {"path": "{home}/zz"}}},
		"links": ["zz"]}`,
	"nodata/config.json":  `{"dependency": "sh"}`,
	"nodata/install.json": `{"schema": 1, "toolchain": {"versions": "system", "packages": {"brew": ["nodata"]}}}`,
	"nodata/install.sh": `source "$(dirname "$0")/../_common.sh"
install_from_data nodata
echo "nodata's script" >> "${HECATRON_ROOT}/.scripts"
`,
}

type dataFixture struct {
	t                *testing.T
	root, fake, data string
	env              []string
}

func newDataFixture(t *testing.T, extra map[string]string) *dataFixture {
	if runtime.GOOS != "linux" {
		t.Skip("installs from data use apt here only on Linux")
	}
	home := repoHome(t)
	definitions := filepath.Join(t.TempDir(), "languages")
	files := map[string]string{}
	for name, content := range dataDefinitions {
		files[name] = content
	}
	common, err := os.ReadFile(filepath.Join(home, "languages", "_common.sh"))
	if err != nil {
		t.Fatal(err)
	}
	files["_common.sh"] = string(common)
	for name, content := range files {
		writeModeFile(t, filepath.Join(definitions, filepath.FromSlash(name)), content, 0o644)
	}
	f := &dataFixture{t: t, fake: t.TempDir(), data: t.TempDir()}
	for _, dir := range []string{"bin", "installed"} {
		if err := os.MkdirAll(filepath.Join(f.fake, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tools := filepath.Join(t.TempDir(), "tools")
	for name, content := range fakeApt {
		writeModeFile(t, filepath.Join(tools, name), content, 0o755)
	}
	f.root = repoFixture(t, merge(map[string]string{
		".hecatron/config.json": `{"schema": 1, "languages": ["caller", "nodata", "pkg"]}`, "pkg/x": ""}, extra))
	f.env = []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=" + definitions, "HECATRON_AS_OF=off",
		"HECATRON_OUTPUT=human", "HECATRON_INSTALLER=", "FAKE=" + f.fake, "XDG_DATA_HOME=" + f.data,
		"PATH=" + tools + ":" + filepath.Join(f.fake, "bin") + ":/usr/bin:/bin", "TMPDIR=" + t.TempDir()}
	return f
}

func writeModeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func (f *dataFixture) run(env []string, args ...string) result {
	f.t.Helper()
	return runIn(f.t, f.root, append(append([]string{}, f.env...), env...), append([]string{binary}, args...)...)
}

// read is a file's content, "" when it's missing; reading resets it.
func (f *dataFixture) read(path string) string {
	data, _ := os.ReadFile(path)
	_ = os.Remove(path)
	return string(data)
}

func (f *dataFixture) aptLog() string { return f.read(filepath.Join(f.fake, "log")) }
func (f *dataFixture) scripts() string {
	return f.read(filepath.Join(f.root, ".scripts"))
}

func TestInstallFromData(t *testing.T) {
	f := newDataFixture(t, map[string]string{"pkg/a.tool": ""})
	r := f.run(nil, "install", "--plan", "pkg", "caller", "nodata")
	for _, line := range []string{"pkg: ", "/pkg/install.json", "install  toolchain (apt: pkg-a): missing pkg-a",
		"install  tool pkg-tool (apt: pkg-tool)", "caller: ", "nodata: ", "(install.json: it has no apt packages)",
		"Would run:", "apt-get -o Acquire::Retries=3 install -y pkg-a pkg-tool"} {
		if !strings.Contains(r.stdout, line) {
			t.Errorf("--plan lacks %q:\n%s", line, r.stdout)
		}
	}
	if log, scripts := f.aptLog(), f.scripts(); r.status != 0 || strings.Contains(log, "install ") || scripts != "" {
		t.Fatalf("--plan changed something: %+v %q %q", r, log, scripts)
	}

	r = f.run(nil, "install", "pkg")
	if log := f.aptLog(); r.status != 0 || !strings.Contains(log, "install -y pkg-a pkg-tool") || f.scripts() != "" {
		t.Fatalf("install pkg: %+v, apt %q", r, log)
	}
	receipt, err := os.ReadFile(filepath.Join(f.data, "hecatron", "receipts", "pkg.json"))
	if err != nil || !strings.Contains(string(receipt), `"pkg-a"`) || !strings.Contains(string(receipt), `"pkg-tool"`) {
		t.Errorf("the receipt: %s %v", receipt, err)
	}
	if r = f.run(nil, "install", "pkg"); r.status != 0 || strings.Contains(f.aptLog(), "install ") {
		t.Errorf("a rerun installed again: %+v", r)
	}

	if r = f.run(nil, "doctor", "pkg"); r.status != 0 || strings.Contains(r.stdout, "never") {
		t.Errorf("doctor pkg: %+v", r)
	}
	f.read(filepath.Join(f.fake, "bin", "pkg-tool"))
	r = f.run(nil, "doctor", "pkg")
	if r.status != 1 || !strings.Contains(r.stdout, "pkg-tool not installed (a component has .tool files)") {
		t.Errorf("doctor with pkg-tool gone: %+v", r)
	}
	r = f.run(nil, "doctor", "--fix", "pkg")
	if log := f.aptLog(); r.status != 0 || !strings.Contains(r.stdout, "pkg: repairing with ") ||
		!strings.Contains(log, "install -y pkg-tool") || strings.Contains(log, "pkg-a") {
		t.Errorf("doctor --fix pkg: %+v, apt %q", r, log)
	}

	r = f.run(nil, "uninstall", "pkg")
	if log := f.aptLog(); r.status != 0 || !strings.Contains(log, "remove -y pkg-a pkg-tool") {
		t.Errorf("uninstall pkg: %+v, apt %q", r, log)
	}
	if r = f.run(nil, "uninstall", "pkg"); r.status != 0 || !strings.Contains(r.stdout, "no receipt") || f.aptLog() != "" {
		t.Errorf("uninstall without a receipt: %+v", r)
	}
}

// Another installer running a converted language's install.sh gets the
// install from data through the binary; forced to the scripts, or
// declined, the script carries on itself, once.
func TestConvertedInstallShHandsOverToTheBinary(t *testing.T) {
	f := newDataFixture(t, nil)
	r := f.run(nil, "install", "caller")
	if scripts, log := f.scripts(), f.aptLog(); r.status != 0 || scripts != "caller's script\n" || !strings.Contains(log, "install -y pkg-a") {
		t.Errorf("install caller: %+v, scripts %q, apt %q", r, scripts, log)
	}
	r = f.run([]string{"HECATRON_INSTALLER=script"}, "install", "caller")
	if scripts, log := f.scripts(), f.aptLog(); r.status != 0 || scripts != "caller's script\npkg's script\n" || log != "" {
		t.Errorf("HECATRON_INSTALLER=script install caller: %+v, scripts %q, apt %q", r, scripts, log)
	}
	r = f.run(nil, "install", "nodata")
	if scripts := f.scripts(); r.status != 0 || scripts != "nodata's script\n" {
		t.Errorf("install nodata: %+v, scripts %q", r, scripts)
	}
	if r = f.run(nil, "_install", "nodata"); r.status != declinedStatus {
		t.Errorf("_install nodata: %+v", r)
	}
	if r = f.run(nil, "_install", "caller"); r.status != declinedStatus || f.scripts() != "" {
		t.Errorf("_install caller (no install.json) ran something: %+v", r)
	}
}

// A "managed" release download installs the default and each version the
// components resolve to, a prefix as its newest release in releases.json,
// side by side; the link is the default's.
func TestManagedDownloadPlansEachVersion(t *testing.T) {
	f := newDataFixture(t, map[string]string{
		".hecatron/config.json": `{"schema": 1, "languages": ["zz"]}`, "zz/.zz-version": "0.9\n"})
	r := f.run(nil, "install", "--plan", "zz")
	toolchains := filepath.Join(f.data, "hecatron", "toolchains", "zz")
	for _, line := range []string{"install  toolchain zz 1.0.0 (", "install  toolchain zz 0.9.2 (",
		"into " + filepath.Join(toolchains, "0.9.2"), "→ " + filepath.Join(toolchains, "1.0.0", "zz")} {
		if !strings.Contains(r.stdout, line) {
			t.Errorf("--plan lacks %q:\n%s", line, r.stdout)
		}
	}
	if r.status != 0 || strings.Contains(r.stdout, "0.9.1") {
		t.Errorf("--plan: %+v", r)
	}

	writeModeFile(t, filepath.Join(f.root, "zz", ".zz-version"), "2.1\n", 0o644)
	r = f.run(nil, "install", "--plan", "zz")
	if !strings.Contains(r.stderr, "no zz release matches 2.1 (from pin:zz/.zz-version) in its releases.json") {
		t.Errorf("a version with no release: %+v", r)
	}
}

// A project's own install.sh always wins over the data.
func TestProjectInstallShWins(t *testing.T) {
	f := newDataFixture(t, map[string]string{
		".hecatron/languages/pkg/install.sh": `echo "project's pkg" >> "${HECATRON_ROOT}/.scripts"`})
	r := f.run(nil, "install", "pkg")
	if scripts := f.scripts(); r.status != 0 || scripts != "project's pkg\n" || f.aptLog() != "" {
		t.Errorf("install pkg: %+v, scripts %q", r, scripts)
	}
	r = f.run(nil, "install", "--plan", "pkg")
	if !strings.Contains(r.stdout, "(the project's own install.sh)") {
		t.Errorf("--plan: %+v", r)
	}
}
