package sbom

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dnf/hecatron/internal/licenses"
	"github.com/dnf/hecatron/internal/spdx"
)

// The bill of materials of the license fixtures (tests/licenses/fixtures,
// with OSV-Scanner's captured reports in tests/output/osv-scanner/2.7.0/),
// each language its own component, in both formats. The golden files in
// testdata/ are also checked against the CycloneDX 1.6 and SPDX 2.3 JSON
// schemas, by tests/test_sbom.py.

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

var fixtureLangs = []string{"csharp", "dart", "elixir", "go", "java", "php", "python", "ruby", "rust", "typescript"}

const capturedAt = "/tmp/licenses-fixtures"

func fixtureInput(t *testing.T) Input {
	t.Helper()
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var targets []licenses.Target
	var components []Component
	for _, lang := range fixtureLangs {
		from := filepath.Join(home, "tests", "licenses", "fixtures", lang)
		entries, err := os.ReadDir(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, lang), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			data, err := os.ReadFile(filepath.Join(from, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, lang, entry.Name()), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		targets = append(targets, licenses.Target{Lang: lang, Component: lang + "-app", Dir: filepath.Join(root, lang)})
		components = append(components, Component{Name: lang + "-app", Languages: []string{lang}})
	}
	scan := func(dir string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(home, "tests", "output", "osv-scanner", "2.7.0", "licenses-"+filepath.Base(dir)+".json"))
		if err != nil {
			return nil, err
		}
		return []byte(strings.ReplaceAll(string(data), capturedAt, root)), nil
	}
	report, err := licenses.GatherAll(root, targets, scan)
	if err != nil {
		t.Fatal(err)
	}
	return Input{Project: "fixtures", Commit: "0123456789abcdef0123456789abcdef01234567", ToolVersion: "0.6.0",
		Time: time.Unix(1700000000, 0), Components: components, Packages: report.Packages}
}

func write(t *testing.T, in Input, format string) []byte {
	t.Helper()
	data, err := Write(in, format)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from what's written (go test ./internal/sbom -update rewrites it)", path)
	}
}

func decode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func list(v any) []map[string]any {
	var out []map[string]any
	items, _ := v.([]any)
	for _, item := range items {
		out = append(out, item.(map[string]any))
	}
	return out
}

func TestGolden(t *testing.T) {
	in := fixtureInput(t)
	golden(t, "fixtures.cdx.json", write(t, in, "cyclonedx"))
	golden(t, "fixtures.spdx.json", write(t, in, "spdx"))
	in.NoDev = true
	golden(t, "fixtures-no-dev.cdx.json", write(t, in, "cyclonedx"))
	golden(t, "fixtures-no-dev.spdx.json", write(t, in, "spdx"))
}

func TestCycloneDX(t *testing.T) {
	in := fixtureInput(t)
	document := decode(t, write(t, in, "cyclonedx"))
	refs := map[string]map[string]any{"project": nil}
	for _, c := range list(document["components"]) {
		ref := c["bom-ref"].(string)
		if _, seen := refs[ref]; seen {
			t.Errorf("bom-ref %s twice", ref)
		}
		refs[ref] = c
		for _, l := range list(c["licenses"]) {
			if _, err := spdx.Parse(l["expression"].(string)); err != nil {
				t.Errorf("%s: %v", ref, err)
			}
		}
	}
	for _, d := range list(document["dependencies"]) {
		for _, ref := range append([]any{d["ref"]}, d["dependsOn"].([]any)...) {
			if _, ok := refs[ref.(string)]; !ok {
				t.Errorf("dependency on %v, no such component", ref)
			}
		}
	}
	cases := map[string]struct {
		license, scope string
	}{
		"pkg:npm/lodash@4.17.21":                            {"MIT", "required"},
		"pkg:npm/is-number@7.0.0":                           {"MIT", "optional"},
		"pkg:cargo/serde_json@1.0.108":                      {"Apache-2.0 OR MIT", "required"},
		"pkg:maven/junit/junit@4.13.2":                      {"", "optional"},
		"pkg:golang/github.com/google/uuid@1.6.0":           {"BSD-3-Clause", "required"},
		"pkg:composer/phpunit/php-timer@6.0.0":              {"BSD-3-Clause", "optional"},
		"pkg:pub/lints@3.0.0":                               {"", "optional"},
		"pkg:hex/jason@1.4.1":                               {"", "required"},
		"pkg:pypi/pycrypto@2.6.1":                           {"", "required"},
		"pkg:gem/racc@1.7.3":                                {"Ruby OR BSD-2-Clause", "required"},
		"pkg:nuget/Newtonsoft.Json@13.0.3":                  {"MIT", "required"},
		"pkg:maven/org.apache.commons/commons-lang3@3.14.0": {"Apache-2.0", "required"},
	}
	for purl, want := range cases {
		c := refs[purl]
		if c == nil {
			t.Errorf("%s: not listed", purl)
			continue
		}
		license := ""
		if l := list(c["licenses"]); len(l) == 1 {
			license = l[0]["expression"].(string)
		}
		if license != want.license || c["scope"] != want.scope {
			t.Errorf("%s: %q, scope %v; want %q, %s", purl, license, c["scope"], want.license, want.scope)
		}
	}
	direct := map[string][]any{}
	for _, d := range list(document["dependencies"]) {
		direct[d["ref"].(string)] = d["dependsOn"].([]any)
	}
	if !containsAny(direct["component:typescript-app"], "pkg:npm/lodash@4.17.21") ||
		containsAny(direct["component:typescript-app"], "pkg:npm/color-name@1.1.4") {
		t.Errorf("typescript-app's direct dependencies: %v", direct["component:typescript-app"])
	}
	if len(direct["project"]) != len(fixtureLangs) {
		t.Errorf("the project's components: %v", direct["project"])
	}
	metadata := document["metadata"].(map[string]any)
	if metadata["timestamp"] != "2023-11-14T22:13:20Z" || metadata["component"].(map[string]any)["name"] != "fixtures" {
		t.Errorf("metadata: %v", metadata)
	}
}

func containsAny(list []any, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func TestSPDX(t *testing.T) {
	in := fixtureInput(t)
	document := decode(t, write(t, in, "spdx"))
	ids := map[string]map[string]any{"SPDXRef-DOCUMENT": nil}
	for _, p := range list(document["packages"]) {
		id := p["SPDXID"].(string)
		if _, seen := ids[id]; seen {
			t.Errorf("SPDXID %s twice", id)
		}
		ids[id] = p
		if declared, ok := p["licenseDeclared"].(string); ok && declared != "NOASSERTION" {
			if _, err := spdx.Parse(declared); err != nil {
				t.Errorf("%s: %v", id, err)
			}
		}
	}
	kinds := map[string]int{}
	for _, r := range list(document["relationships"]) {
		for _, key := range []string{"spdxElementId", "relatedSpdxElement"} {
			if _, ok := ids[r[key].(string)]; !ok {
				t.Errorf("relationship with %v, no such element", r[key])
			}
		}
		kinds[r["relationshipType"].(string)]++
	}
	if kinds["DESCRIBES"] != 1 || kinds["CONTAINS"] != len(fixtureLangs) || kinds["DEV_DEPENDENCY_OF"] == 0 || kinds["DEPENDS_ON"] == 0 {
		t.Errorf("relationships: %v", kinds)
	}
	if lints := ids["SPDXRef-Package-Pub-lints-3.0.0"]; lints == nil || lints["licenseDeclared"] != "NOASSERTION" {
		t.Errorf("pub's lints: %v", lints)
	}
	if !strings.HasPrefix(document["documentNamespace"].(string), "https://spdx.org/spdxdocs/fixtures-") {
		t.Errorf("namespace: %v", document["documentNamespace"])
	}
}

func TestNoDev(t *testing.T) {
	in := fixtureInput(t)
	in.NoDev = true
	for _, format := range Formats {
		out := string(write(t, in, format))
		if strings.Contains(out, "pkg:npm/is-number@") || strings.Contains(out, "pkg:pub/lints@") ||
			!strings.Contains(out, "pkg:npm/lodash@") {
			t.Errorf("%s with --no-dev lists a dev dependency, or lost another", format)
		}
	}
}

// The serial number and namespace follow the content: the same input
// gives the same document, another time another one.
func TestReproducible(t *testing.T) {
	in := fixtureInput(t)
	for _, format := range Formats {
		first, second := write(t, in, format), write(t, in, format)
		later := in
		later.Time = in.Time.Add(time.Second)
		if !bytes.Equal(first, second) || bytes.Equal(first, write(t, later, format)) {
			t.Errorf("%s: not reproducible, or the same at another time", format)
		}
	}
	if _, err := Write(in, "xml"); err == nil {
		t.Error("an unknown format is accepted")
	}
}

// A LicenseRef needs its text in an SPDX document; hecatron has none, so
// the license is said in a comment instead.
func TestLicenseRef(t *testing.T) {
	in := Input{Project: "p", ToolVersion: "1", Time: time.Unix(0, 0), Components: []Component{{"c", []string{"python"}}},
		Packages: []licenses.Package{{Ecosystem: "PyPI", Name: "x", Version: "1", License: "LicenseRef-Proprietary",
			Dependency: "direct", Language: "python", Component: "c", Source: "requirements.txt"}}}
	out := string(write(t, in, "spdx"))
	if !strings.Contains(out, `"licenseDeclared": "NOASSERTION"`) || !strings.Contains(out, "declared: LicenseRef-Proprietary") {
		t.Errorf("%s", out)
	}
	if out := string(write(t, in, "cyclonedx")); !strings.Contains(out, `"expression": "LicenseRef-Proprietary"`) {
		t.Errorf("%s", out)
	}
}

func TestPurl(t *testing.T) {
	cases := []struct{ ecosystem, name, version, want string }{
		{"npm", "@types/node", "20.1.0", "pkg:npm/%40types/node@20.1.0"},
		{"npm", "lodash", "4.17.21", "pkg:npm/lodash@4.17.21"},
		{"PyPI", "Typing_Extensions", "4.8.0", "pkg:pypi/typing-extensions@4.8.0"},
		{"Go", "github.com/google/uuid", "1.6.0", "pkg:golang/github.com/google/uuid@1.6.0"},
		{"Go", "stdlib", "1.22.1", "pkg:golang/stdlib@1.22.1"},
		{"Maven", "org.apache.commons:commons-lang3", "3.14.0", "pkg:maven/org.apache.commons/commons-lang3@3.14.0"},
		{"Packagist", "Monolog/Monolog", "3.5.0", "pkg:composer/monolog/monolog@3.5.0"},
		{"crates.io", "serde", "1.0.0+build", "pkg:cargo/serde@1.0.0%2Bbuild"},
		{"RubyGems", "rack", "2.2.8", "pkg:gem/rack@2.2.8"},
		{"NuGet", "Newtonsoft.Json", "13.0.3", "pkg:nuget/Newtonsoft.Json@13.0.3"},
		{"SwiftURL", "https://github.com/apple/swift-nio.git", "2.0.0", "pkg:swift/github.com/apple/swift-nio.git@2.0.0"},
		{"Hex", "Jason", "1.4.1", "pkg:hex/jason@1.4.1"},
		{"Other", "x y", "", "pkg:generic/x%20y"},
	}
	for _, c := range cases {
		if got := Purl(c.ecosystem, c.name, c.version); got != c.want {
			t.Errorf("Purl(%s, %s, %s) = %s; want %s", c.ecosystem, c.name, c.version, got, c.want)
		}
	}
}
