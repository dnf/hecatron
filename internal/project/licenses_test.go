package project

import (
	"reflect"
	"strings"
	"testing"
)

func TestLicensesProblems(t *testing.T) {
	home := testHome(t)
	root := t.TempDir()
	write(t, root, map[string]string{".hecatron/config.json": `{"schema": 1, "licenses": {
		"allow": ["mit", "Apache-2.0", "Nope", 5], "deny": "GPL-3.0", "includeDev": "yes", "other": 1,
		"ignore": [{"package": "npm:caniuse-lite", "reason": "data", "until": "2027-06-01"}, {"package": "npm:x"}, {"reason": "r"}]}}`})
	p, err := Load(root, home)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"licenses.other isn't a setting (allow, deny, ignore, includeDev)",
		`licenses.allow: "Nope" isn't an SPDX license identifier (https://spdx.org/licenses/)`,
		"licenses.allow: 5 isn't an SPDX license identifier (https://spdx.org/licenses/)",
		"licenses.deny must be a list of SPDX license identifiers",
		"licenses.includeDev must be true or false",
		`licenses.ignore npm:x needs a "reason"`,
		`licenses.ignore npm:x needs an "until" date (YYYY-MM-DD)`,
		`licenses.ignore[2] needs a "package" (ecosystem:name, e.g. npm:caniuse-lite)`,
	}
	if got := p.LicensesProblems(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems:\n%s", strings.Join(got, "\n"))
	}
	policy := p.LicensePolicy()
	if !policy.Set || !reflect.DeepEqual(policy.Allow, []string{"MIT", "Apache-2.0"}) || len(policy.Ignore) != 2 {
		t.Errorf("policy: %+v", policy)
	}

	write(t, root, map[string]string{".hecatron/config.json": `{"schema": 1, "licenses": {"deny": ["GPL-3.0"], "includeDev": true}}`})
	if p, err = Load(root, home); err != nil {
		t.Fatal(err)
	}
	if problems := p.LicensesProblems(); len(problems) != 0 {
		t.Errorf("problems: %q", problems)
	}
	if policy := p.LicensePolicy(); !reflect.DeepEqual(policy.Deny, []string{"GPL-3.0-only"}) || !policy.IncludeDev {
		t.Errorf("policy: %+v", policy)
	}
	write(t, root, map[string]string{".hecatron/config.json": `{"schema": 1}`})
	if p, err = Load(root, home); err != nil {
		t.Fatal(err)
	}
	if p.LicensePolicy().Set || len(p.LicensesProblems()) != 0 {
		t.Error("a policy without a licenses section")
	}
}
