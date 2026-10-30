package project

import "testing"

// The cases tests/test_versions.py's RangeTest checks versions.py against.
func TestSatisfies(t *testing.T) {
	cases := []struct {
		allowed, version string
		want             bool
	}{
		{">=22", "26.13.0", true}, {">=22", "20.1.0", false}, {">= 22", "22.0.0", true},
		{">22", "22.5.0", false}, {">22", "23.0.0", true}, {">22.4", "22.5.0", true},
		{"<=22", "22.9.9", true}, {"<22", "22.0.0", false}, {"22.x", "22.2.0", true}, {"22", "23.0.0", false},
		{"=22.4.0", "22.4.0", true}, {"=22.4.0", "22.4.1", false}, {"*", "1.0.0", true}, {"", "1.0.0", true},
		{"^18", "18.9.1", true}, {"^18", "19.0.0", false}, {"^0.2.3", "0.2.9", true}, {"^0.2.3", "0.3.0", false},
		{"^0.0.3", "0.0.4", false}, {"~18.17", "18.17.9", true}, {"~18.17", "18.18.0", false},
		{">=16 <21", "20.9.0", true}, {">=16 <21", "21.0.0", false}, {"14 || >=18", "16.0.0", false},
		{"14 || >=18", "14.2.0", true}, {"20 - 22", "22.9.0", true}, {"20 - 22", "23.0.0", false},
		{"20 - 22.4.1", "22.4.2", false}, {">=22", "26.0.0-rc.1", false}, {"lts", "1.0.0", false},
	}
	for _, c := range cases {
		if got := satisfies(c.allowed, c.version); got != c.want {
			t.Errorf("satisfies(%q, %q) = %v, want %v", c.allowed, c.version, got, c.want)
		}
	}
}
