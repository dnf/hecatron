package deps

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The fixed versions each captured report gives, for one finding of each.
func TestFixedVersionsFromCapturedReports(t *testing.T) {
	home := repoHome(t)
	output := filepath.Join(home, "tests", "output")
	reports := filepath.Join(home, "tests", "audit", "reports")
	cases := []struct {
		format, path, id, pkg string
		want                  []string
	}{
		{"npm", filepath.Join(reports, "npm.json"), "GHSA-35jh-r3h4-6jhm", "lodash", []string{"4.17.21"}},
		{"npm", filepath.Join(reports, "npm.json"), "GHSA-p6mc-m468-83gw", "lodash", []string{"4.17.19"}},
		{"pnpm", filepath.Join(output, "pnpm", "12.15.0", "audit-pnpm-lodash.json"), "GHSA-p6mc-m468-83gw", "lodash", []string{"4.17.19"}},
		{"yarn-classic", filepath.Join(output, "yarn", "1.22.22", "audit-yarn-classic-lodash.json"), "GHSA-35jh-r3h4-6jhm", "lodash", []string{"4.17.21"}},
		{"yarn-berry", filepath.Join(output, "yarn", "4.18.1", "audit-yarn-berry-lodash.json"), "GHSA-29mw-wpgm-hmr9", "lodash", []string{"4.17.21"}},
		{"bun", filepath.Join(output, "bun", "1.4.4", "audit-bun-lodash.json"), "GHSA-p6mc-m468-83gw", "lodash", []string{"4.17.19"}},
		{"pip-audit", filepath.Join(reports, "pip-audit.json"), "PYSEC-2018-28", "requests", []string{"2.20.0"}},
		{"composer", filepath.Join(reports, "composer.json"), "GHSA-c2w2-prh8-qm98", "guzzlehttp/psr7", []string{"2.12.3"}},
		{"bundle-audit", filepath.Join(reports, "bundle-audit.txt"), "", "rack", nil}, // checked below
		{"osv", filepath.Join(reports, "osv-rust.json"), "GHSA-wcg3-cvx6-7396", "time", []string{"0.2.23"}},
	}
	for _, c := range cases {
		text := readFile(t, c.path)
		findings, err := ParseAudit(c.format, text)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		fixes := FixedVersions(c.format, text)
		found := false
		for _, f := range findings {
			if Text(f.Package) != c.pkg || (c.id != "" && Text(f.ID) != c.id) {
				continue
			}
			found = true
			got := fixes.For(f)
			if c.want == nil {
				if len(got) == 0 {
					t.Errorf("%s %s %s: no fixed versions", c.format, c.pkg, Text(f.ID))
				}
				continue
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("%s %s %s: fixed %q, want %q", c.format, c.pkg, Text(f.ID), got, c.want)
			}
		}
		if !found {
			t.Errorf("%s: no finding for %s %s", c.path, c.pkg, c.id)
		}
	}
}

func TestGovulncheckFixedVersion(t *testing.T) {
	text := readFile(t, filepath.Join(repoHome(t), "tests", "audit", "reports", "govulncheck.json"))
	findings, err := ParseAudit("govulncheck", text)
	if err != nil || len(findings) == 0 {
		t.Fatalf("findings %v, %v", findings, err)
	}
	fixes := FixedVersions("govulncheck", text)
	for _, f := range findings {
		got := fixes.For(f)
		if len(got) != 1 || !strings.HasPrefix(got[0], "v") {
			t.Errorf("%s %s: fixed %q, want govulncheck's fixed_version", Text(f.Package), Text(f.ID), got)
		}
	}
}

func TestBundleAuditSolutions(t *testing.T) {
	got := fixedByRequirements("update to '~> 2.0.9, >= 2.0.9.1', '~> 2.1.4, >= 2.1.4.1', '>= 2.2.3.1'")
	if want := []string{"2.0.9.1", "2.1.4.1", "2.2.3.1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("fixed %q, want %q", got, want)
	}
}

func TestRanges(t *testing.T) {
	for _, c := range []struct {
		vulnerable string
		want       []string
	}{
		{"<4.17.21", []string{"4.17.21"}},
		{">=3.7.0 <4.17.19", []string{"4.17.19"}},
		{">=1.0.0 <1.2.3 || >=2.0.0 <2.0.5", []string{"1.2.3", "2.0.5"}},
		{"<=1.2.3", nil}, // no version outside it is named
		{">=1.0,<1.4.2|>=2.0,<2.1", []string{"1.4.2", "2.1"}}, // Composer's
		{"*", nil},
	} {
		if got := fixedByRange(c.vulnerable); !reflect.DeepEqual(got, c.want) {
			t.Errorf("fixedByRange(%q) = %q, want %q", c.vulnerable, got, c.want)
		}
	}
	if got := fixedByPatched("<0.0.0"); got != nil {
		t.Errorf("npm's no-patch range gave %q", got)
	}
}

func TestOSVFixForTheInstalledVersion(t *testing.T) {
	events := []osvEvent{{Introduced: "0"}, {Fixed: "1.2.0"}, {Introduced: "2.0.0"}, {Fixed: "2.3.1"}, {Introduced: "3.0.0-rc1"}, {Fixed: "3.0.0"}}
	for installed, want := range map[string][]string{
		"1.1.9":     {"1.2.0"},
		"2.1.0":     {"2.3.1"},
		"2.10.0":    {"1.2.0", "2.3.1", "3.0.0"}, // in none: every fix
		"3.0.0-rc2": {"3.0.0"},
		"":          {"1.2.0", "2.3.1", "3.0.0"},
	} {
		if got := fixedAfter(events, installed); !reflect.DeepEqual(got, want) {
			t.Errorf("fixedAfter(%q) = %q, want %q", installed, got, want)
		}
	}
	for _, c := range [][2]string{{"1.9", "1.10"}, {"1.0.0-rc1", "1.0.0"}, {"0.0.0-0", "0.1.43"}, {"1.0", "1.0.1"}, {"v1.2.3", "1.2.4"}} {
		if compareVersions(c[0], c[1]) >= 0 || compareVersions(c[1], c[0]) <= 0 {
			t.Errorf("%s should come before %s", c[0], c[1])
		}
	}
}

// npm's report names the vulnerable range, not the installed version: the
// lock file in the directory it ran in gives it, as audit.py reads it.
func TestNPMInstalledVersionFromTheLockFile(t *testing.T) {
	report := readFile(t, filepath.Join(repoHome(t), "tests", "audit", "reports", "npm.json"))
	dir := t.TempDir()
	lock := `{"lockfileVersion": 3, "packages": {"": {"name": "x"}, "node_modules/lodash": {"version": "4.17.15"},
		"node_modules/a/node_modules/lodash": {"version": "4.17.10"}, "node_modules/b": "odd"}}`
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	findings, err := ParseAudit("npm", report)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if Text(f.Package) == "lodash" && Text(f.Version) != "4.17.10, 4.17.15" {
			t.Errorf("lodash %s: version %q", Text(f.ID), Text(f.Version))
		}
	}
	line, _ := Line(findings[0])
	if !strings.Contains(line, "lodash 4.17.10, 4.17.15  GHSA-") {
		t.Errorf("line %q", line)
	}
}
