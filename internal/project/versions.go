package project

import (
	"fmt"
	"regexp"
	"strings"
)

// Hecatron's version syntax (docs/design.md §18), as libexec/lib/versions.py
// has it. A spec is a prefix, "3.12", meaning the newest release whose
// version begins with those whole components, or exact, "=4.5". A leading
// "v" and build metadata ("+11") are ignored; a pre-release ("-rc1") is its
// own version and sorts before its release.
//
// Every string reaching these functions has passed checkDigits, so the
// digits \d matches in Python are ASCII ones here.

var versionPattern = regexp.MustCompile(`^v?([0-9]+(?:\.[0-9]+)*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

type version struct {
	numbers []string // decimal, without leading zeros: Python's ints have no size limit
	pre     string
}

// parseVersion is versions.py's _parse.
func parseVersion(text string) (version, bool) {
	match := versionPattern.FindStringSubmatch(pyStrip(text))
	if match == nil {
		return version{}, false
	}
	var v version
	for _, part := range strings.Split(match[1], ".") {
		v.numbers = append(v.numbers, pyInt(part))
	}
	v.pre = match[2]
	return v, true
}

func isVersion(text string) bool {
	_, ok := parseVersion(text)
	return ok
}

// pyInt is Python's int() of ASCII digits, kept as text.
func pyInt(digits string) string {
	if trimmed := strings.TrimLeft(digits, "0"); trimmed != "" {
		return trimmed
	}
	return "0"
}

func compareInts(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

// compareNumbers is Python's comparison of two tuples of ints.
func compareNumbers(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := compareInts(a[i], b[i]); c != 0 {
			return c
		}
	}
	return len(a) - len(b)
}

// compareVersions orders versions as versions.py's sort_key: numerically, a
// pre-release just before its release, and text that isn't a version
// before any version, by its characters.
func compareVersions(a, b string) int {
	va, okA := parseVersion(a)
	vb, okB := parseVersion(b)
	switch {
	case !okA && !okB:
		return strings.Compare(a, b)
	case !okA:
		return -1
	case !okB:
		return 1
	}
	if c := compareNumbers(va.numbers, vb.numbers); c != 0 {
		return c
	}
	releaseA, releaseB := va.pre == "", vb.pre == ""
	if releaseA != releaseB {
		if releaseA {
			return 1
		}
		return -1
	}
	return strings.Compare(va.pre, vb.pre)
}

// newest is versions.py's newest: the greatest version, the first of equals
// (as Python's max), and false for none.
func newest(candidates []string) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if compareVersions(candidate, best) > 0 {
			best = candidate
		}
	}
	return best, true
}

// spec is a version as a project or definition writes it.
type spec struct {
	text  string
	exact bool
	version
}

// parseSpec is versions.py's Spec, failing with its ValueError's message.
func parseSpec(text string) (spec, error) {
	s := spec{text: text, exact: strings.HasPrefix(text, "=")}
	parsed, ok := parseVersion(strings.TrimPrefix(text, "="))
	if !ok {
		return spec{}, &Error{fmt.Sprintf("not a version: %s", pyStringRepr(text))}
	}
	s.version = parsed
	return s, nil
}

// mustSpec is parseSpec for text already known to be a version.
func mustSpec(text string) spec {
	s, err := parseSpec(text)
	if err != nil {
		panic(err)
	}
	return s
}

func (s spec) matches(text string) bool {
	v, ok := parseVersion(text)
	if !ok {
		return false
	}
	if s.exact {
		return equalNumbers(v.numbers, s.numbers) && v.pre == s.pre
	}
	return len(v.numbers) >= len(s.numbers) && equalNumbers(v.numbers[:len(s.numbers)], s.numbers) &&
		(s.pre == "" || v.pre == s.pre)
}

func equalNumbers(a, b []string) bool { return compareNumbers(a, b) == 0 }

// prefixNumbers is Python's numbers[:n].
func prefixNumbers(numbers []string, n int) []string {
	if n < len(numbers) {
		return numbers[:n]
	}
	return numbers
}
