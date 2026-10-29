package licenses

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnf/hecatron/internal/project"
)

// The fixture projects (tests/licenses/fixtures/<lang>/) with OSV-Scanner's
// report on each as captured by tests/licenses/capture.sh
// (tests/output/osv-scanner/2.7.0/licenses-<lang>.json), read offline.

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

const capturedAt = "/tmp/licenses-fixtures"

func repoHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

var fixtureLangs = []string{"csharp", "dart", "elixir", "go", "java", "php", "python", "ruby", "rust", "typescript"}

// fixtureProject copies the fixtures into a project root, each language in
// its own component (<lang>-app), and gives a scanner answering with the
// captured reports.
func fixtureProject(t *testing.T) (string, []Target, Scanner) {
	t.Helper()
	home := repoHome(t)
	root := t.TempDir()
	var targets []Target
	for _, lang := range fixtureLangs {
		copyTree(t, filepath.Join(home, "tests", "licenses", "fixtures", lang), filepath.Join(root, lang))
		targets = append(targets, Target{Lang: lang, Component: lang + "-app", Dir: filepath.Join(root, lang)})
	}
	targets = append(targets, Target{Lang: "zig", Component: "tools", Dir: filepath.Join(root, "zig")})
	scan := func(dir string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(home, "tests", "output", "osv-scanner", "2.7.0", "licenses-"+filepath.Base(dir)+".json"))
		if err != nil {
			return nil, err
		}
		return []byte(strings.ReplaceAll(string(data), capturedAt, root)), nil
	}
	return root, targets, scan
}

func gather(t *testing.T) *Report {
	t.Helper()
	root, targets, scan := fixtureProject(t)
	report, err := Gather(root, targets, scan)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func find(r *Report, key, version string) *Package {
	for i, p := range r.Packages {
		if p.Key() == key && (version == "" || p.Version == version) {
			return &r.Packages[i]
		}
	}
	return nil
}

func TestEachEcosystem(t *testing.T) {
	r := gather(t)
	cases := []struct {
		key, version, license, dependency string
		dev                               bool
	}{
		{"npm:lodash", "4.17.21", "MIT", "direct", false},
		{"npm:caniuse-lite", "1.0.30001570", "CC-BY-4.0", "direct", false},
		{"npm:color-name", "1.1.4", "MIT", "transitive", false},
		{"npm:is-number", "7.0.0", "MIT", "direct", true},
		{"npm:left-pad", "1.3.0", "WTFPL", "direct", false},
		{"pypi:requests", "2.31.0", "Apache-2.0", "direct", false},
		{"pypi:pyqt5", "5.15.10", "GPL-3.0-only", "direct", false},
		{"pypi:pyqt5-qt5", "5.15.2", "LGPL-3.0-only", "transitive", false},
		{"pypi:pycrypto", "2.6.1", "", "direct", false},
		{"go:github.com/google/uuid", "1.6.0", "BSD-3-Clause", "direct", false},
		{"go:github.com/spf13/pflag", "1.0.5", "BSD-3-Clause", "transitive", false},
		{"crates.io:serde_json", "1.0.108", "Apache-2.0 OR MIT", "direct", false},
		{"crates.io:ryu", "1.0.15", "Apache-2.0 OR BSL-1.0", "transitive", false},
		{"maven:org.apache.commons:commons-lang3", "3.14.0", "Apache-2.0", "direct", false},
		{"maven:junit:junit", "4.13.2", "", "direct", true},
		{"maven:com.google.protobuf:protobuf-java", "3.21.9", "BSD-3-Clause", "transitive", false},
		{"nuget:Newtonsoft.Json", "13.0.3", "MIT", "direct", false},
		{"nuget:System.Memory", "4.5.5", "", "transitive", false},
		{"rubygems:rack", "2.2.8", "MIT", "direct", false},
		{"rubygems:racc", "1.7.3", "Ruby OR BSD-2-Clause", "transitive", false},
		{"packagist:monolog/monolog", "3.5.0", "MIT", "direct", false},
		{"packagist:psr/log", "3.0.0", "MIT", "transitive", false},
		{"packagist:phpunit/php-timer", "6.0.0", "BSD-3-Clause", "direct", true},
	}
	for _, c := range cases {
		p := find(r, c.key, c.version)
		if p == nil {
			t.Errorf("%s %s: not reported", c.key, c.version)
			continue
		}
		if p.License != c.license || p.Dependency != c.dependency || p.Dev != c.dev {
			t.Errorf("%s: license %q, %q, dev %v; want %q, %q, dev %v", c.key, p.License, p.Dependency, p.Dev,
				c.license, c.dependency, c.dev)
		}
	}
	if p := find(r, "crates.io:licenses-fixture", ""); p != nil {
		t.Error("the crate itself is reported as a dependency")
	}
	if p := find(r, "pypi:pyqt5", ""); p == nil || p.Component != "python-app" || p.Source != "python/requirements.txt" {
		t.Errorf("pyqt5's component and source: %+v", p)
	}
	for _, p := range r.Packages {
		if p.Ecosystem == "Pub" || p.Ecosystem == "Hex" {
			t.Errorf("%s reported; pub.dev and Hex aren't covered", p.Key())
		}
	}
	notCovered := map[string]string{}
	for _, n := range r.NotCovered {
		notCovered[n.Language] = n.Reason
	}
	for _, lang := range []string{"dart", "elixir", "zig"} {
		if notCovered[lang] == "" {
			t.Errorf("%s isn't listed as not covered: %+v", lang, r.NotCovered)
		}
	}
	if !strings.Contains(strings.Join(r.Warnings, "\n"), "python-app: python: python/requirements.txt isn't a lock file") {
		t.Errorf("no approximate warning for requirements.txt: %q", r.Warnings)
	}
}

// GatherAll (hct sbom) lists the registries deps.dev lacks too, their
// licenses unknown.
func TestGatherAll(t *testing.T) {
	root, targets, scan := fixtureProject(t)
	all, err := GatherAll(root, targets, scan)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pub:lints", "hex:jason"} {
		if p := find(all, key, ""); p == nil || !p.Unknown() || len(p.Declared) != 0 {
			t.Errorf("%s: %+v", key, p)
		}
	}
	if p := find(all, "pub:lints", ""); p == nil || !p.Dev {
		t.Errorf("pub:lints is dev: %+v", p)
	}
	if len(all.Packages) != len(gather(t).Packages)+5 {
		t.Errorf("%d packages; want those of Gather and dart's and elixir's 5", len(all.Packages))
	}
	reasons := map[string]bool{}
	for _, n := range all.NotCovered {
		reasons[n.Language] = true
	}
	if !reasons["zig"] || !reasons["dart"] {
		t.Errorf("not covered: %+v", all.NotCovered)
	}
}

func TestNoLockFile(t *testing.T) {
	root := t.TempDir()
	r, err := Gather(root, []Target{{Lang: "typescript", Component: "web", Dir: filepath.Join(root, "web")}},
		func(string) ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(r.NotCovered) != 1 || r.NotCovered[0].Component != "web" || !strings.Contains(r.NotCovered[0].Reason, "no lock file") {
		t.Errorf("%+v", r.NotCovered)
	}
}

// Nested components: a lock file is reported once, by the deepest
// component directory holding it.
func TestNestedComponents(t *testing.T) {
	root, _, scan := fixtureProject(t)
	targets := []Target{{Lang: "rust", Component: "all", Dir: root}, {Lang: "rust", Component: "core", Dir: filepath.Join(root, "rust")}}
	r, err := Gather(root, targets, func(dir string) ([]byte, error) { return scan(filepath.Join(root, "rust")) })
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range r.Packages {
		if p.Component != "core" {
			t.Errorf("%s reported for %s", p.Key(), p.Component)
		}
	}
	if len(r.NotCovered) != 1 || r.NotCovered[0].Component != "all" {
		t.Errorf("%+v", r.NotCovered)
	}
}

func testPolicy() project.LicensePolicy {
	return project.LicensePolicy{
		Set:   true,
		Allow: []string{"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0"},
		Deny:  []string{"GPL-3.0-only"},
		Ignore: []project.LicenseIgnore{
			{Package: "npm:caniuse-lite", Reason: "CC-BY-4.0 data, reviewed", Until: "2999-01-01"},
			{Package: "PyPI:pycrypto@2.6.1", Reason: "public domain", Until: "2001-01-01"},
		},
	}
}

func TestPolicy(t *testing.T) {
	r := gather(t)
	r.Apply(testPolicy(), "2026-10-27")
	reasons := map[string]string{}
	for _, v := range r.Policy.Violations {
		reasons[v.Package] = v.Reason
	}
	want := map[string]string{
		"pypi:pyqt5":     "GPL-3.0-only denied",
		"pypi:pyqt5-qt5": "LGPL-3.0-only not allowed",
		"pypi:pycrypto":  "unknown license; licenses.allow lists what's allowed",
		"npm:left-pad":   "WTFPL not allowed",
	}
	for key, reason := range want {
		if reasons[key] != reason {
			t.Errorf("%s: %q, want %q", key, reasons[key], reason)
		}
	}
	for _, key := range []string{"npm:caniuse-lite", "crates.io:ryu", "rubygems:racc", "npm:is-number"} {
		if _, failed := reasons[key]; failed {
			t.Errorf("%s fails: %s", key, reasons[key])
		}
	}
	if len(r.Policy.Accepted) != 1 || r.Policy.Accepted[0].Package != "npm:caniuse-lite" {
		t.Errorf("accepted: %+v", r.Policy.Accepted)
	}
	if len(r.Policy.Expired) != 1 || !strings.Contains(r.Policy.Expired[0], "PyPI:pycrypto@2.6.1 expired on 2001-01-01") {
		t.Errorf("expired: %q", r.Policy.Expired)
	}

	// Dev and test dependencies count with includeDev: junit's license is
	// non-standard, so unknown under an allow list.
	withDev := gather(t)
	policy := testPolicy()
	policy.IncludeDev = true
	withDev.Apply(policy, "2026-10-27")
	if len(withDev.Policy.Violations) <= len(r.Policy.Violations) {
		t.Errorf("includeDev adds no violation: %d and %d", len(withDev.Policy.Violations), len(r.Policy.Violations))
	}

	// Deny alone: unknown licenses don't fail, only what's denied.
	denyOnly := gather(t)
	denyOnly.Apply(project.LicensePolicy{Set: true, Deny: []string{"GPL-3.0-only", "CC-BY-4.0"}}, "2026-10-27")
	var failed []string
	for _, v := range denyOnly.Policy.Violations {
		failed = append(failed, v.Package)
	}
	if strings.Join(failed, " ") != "npm:caniuse-lite pypi:pyqt5" {
		t.Errorf("deny only: %v", failed)
	}

	none := gather(t)
	none.Apply(project.LicensePolicy{}, "2026-10-27")
	if none.Policy != nil {
		t.Error("a verdict without a policy")
	}
}

// The renderers' output for the fixtures with the test policy, against
// testdata/golden.* (go test ./internal/licenses -update rewrites them).
func TestRenderers(t *testing.T) {
	r := gather(t)
	r.Apply(testPolicy(), "2026-10-27")
	for _, format := range []string{"human", "agent", "json"} {
		var out bytes.Buffer
		if err := Write(&out, r, format); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "golden."+format)
		if *update {
			if err := os.MkdirAll("testdata", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if out.String() != string(want) {
			t.Errorf("%s output differs from %s:\n%s", format, path, out.String())
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"MIT":                           "MIT",
		"mit":                           "MIT",
		"GPL-3.0":                       "GPL-3.0-only",
		"GPL-2.0+":                      "GPL-2.0-or-later",
		"Apache 2.0":                    "Apache-2.0",
		"(MIT OR Apache-2.0)":           "MIT OR Apache-2.0",
		"MIT AND (BSD-2-Clause OR ISC)": "MIT AND (BSD-2-Clause OR ISC)",
		"GPL-2.0-only WITH Classpath-exception-2.0": "GPL-2.0-only WITH Classpath-exception-2.0",
		"LicenseRef-Proprietary":                    "LicenseRef-Proprietary",
		"non-standard":                              "",
		"UNKNOWN":                                   "",
		"Some License":                              "",
		"MIT OR":                                    "",
	}
	for text, want := range cases {
		got := ""
		if e := Normalize([]string{text}); e != nil {
			got = e.String()
		}
		if got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
	if e := Normalize([]string{"Ruby", "BSD-2-Clause"}); e == nil || e.String() != "Ruby OR BSD-2-Clause" {
		t.Errorf("a list: %v", e)
	}
	if e := Normalize([]string{"MIT", "UNKNOWN"}); e != nil {
		t.Errorf("a list with an unknown: %v", e)
	}
	if e := Normalize(nil); e != nil {
		t.Error("nothing declared")
	}
}
