package spdx

import "testing"

func TestCanonical(t *testing.T) {
	for id, want := range map[string]string{
		"mit": "MIT", "apache-2.0": "Apache-2.0", "GPL-3.0": "GPL-3.0-only", "LGPL-2.1+": "LGPL-2.1-or-later",
		"GPL-2.0-or-later": "GPL-2.0-or-later", "Apache-2.0+": "Apache-2.0+", "LicenseRef-Mine": "LicenseRef-Mine",
	} {
		if got, ok := Canonical(id); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", id, got, ok, want)
		}
	}
	for _, id := range []string{"Foo", "MIT2", "", "non-standard"} {
		if _, ok := Canonical(id); ok {
			t.Errorf("%q is canonical", id)
		}
	}
}

func TestParse(t *testing.T) {
	for text, want := range map[string]string{
		"MIT":                            "MIT",
		"mit or apache-2.0":              "MIT OR Apache-2.0",
		"(MIT OR Apache-2.0) AND ISC":    "(MIT OR Apache-2.0) AND ISC",
		"MIT AND ISC OR BSD-3-Clause":    "MIT AND ISC OR BSD-3-Clause",
		"((MIT))":                        "MIT",
		"MIT OR (ISC OR 0BSD)":           "MIT OR ISC OR 0BSD",
		"Apache-2.0 WITH LLVM-exception": "Apache-2.0 WITH LLVM-exception",
		"The MIT License":                "MIT",
	} {
		e, err := Parse(text)
		if err != nil || e.String() != want {
			t.Errorf("%q: %v %v, want %q", text, e, err, want)
		}
	}
	for _, text := range []string{"", "UNKNOWN", "MIT OR", "(MIT", "MIT)", "MIT WITH Nope", "AND", "Foo-1.0", "MIT ISC"} {
		if e, err := Parse(text); err == nil {
			t.Errorf("%q parses as %v", text, e)
		}
	}
}

func TestPermits(t *testing.T) {
	policy := Policy{Allow: []string{"MIT", "Apache-2.0"}, Deny: []string{"GPL-3.0-only"}}
	for text, want := range map[string]bool{
		"MIT": true, "ISC": false, "GPL-3.0-only": false, "MIT OR GPL-3.0-only": true, "MIT AND GPL-3.0-only": false,
		"MIT AND Apache-2.0": true, "ISC OR (MIT AND Apache-2.0)": true, "Apache-2.0 WITH LLVM-exception": true,
	} {
		e, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := policy.Permits(e); got != want {
			t.Errorf("%s: %v, want %v", text, got, want)
		}
	}
	denyOnly := Policy{Deny: []string{"GPL-3.0-only"}}
	if e, _ := Parse("ISC"); !denyOnly.Permits(e) {
		t.Error("deny only refuses ISC")
	}
}
