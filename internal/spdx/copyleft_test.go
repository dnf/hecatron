package spdx

import (
	"strings"
	"testing"
)

func TestCopyleft(t *testing.T) {
	for text, want := range map[string]Class{
		"MIT":                              None,
		"Apache-2.0+":                      None,
		"GPL-3.0":                          Strong,
		"LGPL-2.1+":                        Weak,
		"MPL-2.0":                          Weak,
		"AGPL-3.0-or-later":                Network,
		"SSPL-1.0":                         Network,
		"RPL-1.5":                          Maximal,
		"CC-BY-4.0":                        Unclassified,
		"LicenseRef-Mine":                  Unclassified,
		"BSD-3-Clause OR GPL-2.0-only":     None, // a choice: the permissive one will do
		"GPL-2.0-only OR LGPL-3.0":         Weak, // the less restrictive choice
		"Apache-2.0 AND LGPL-3.0-or-later": Weak, // both apply
		"MIT AND (GPL-2.0-only OR AGPL-3.0-only)":   Strong,
		"GPL-3.0-only OR CC-BY-4.0":                 Unclassified, // the unclassified choice may be less restrictive
		"MIT OR CC-BY-4.0":                          None,
		"MIT AND CC-BY-4.0":                         Unclassified, // CC-BY-4.0 may add obligations
		"RPL-1.5 AND CC-BY-4.0":                     Maximal,
		"GPL-2.0-only WITH Classpath-exception-2.0": Strong, // judged by the license, as a policy judges it
	} {
		e, err := Parse(text)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if got := e.Copyleft(); got != want {
			t.Errorf("%s: %s, want %s", text, got, want)
		}
	}
}

// Every identifier in copyleft.txt is on the SPDX list as SPDX spells it,
// once, in a known class.
func TestClassList(t *testing.T) {
	seen := map[string]bool{}
	for _, line := range strings.Split(classList, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		class, id, _ := strings.Cut(line, " ")
		if _, ok := classRank[Class(class)]; !ok {
			t.Errorf("%q: class %q", line, class)
		}
		if canonical, ok := Canonical(id); !ok || canonical != id {
			t.Errorf("%q isn't an SPDX identifier as SPDX spells it (%q)", id, canonical)
		}
		if seen[id] {
			t.Errorf("%s twice", id)
		}
		seen[id] = true
	}
}
