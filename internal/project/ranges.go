package project

import (
	"math/big"
	"regexp"
	"strings"
)

// Version ranges, as libexec/lib/versions.py reads them: npm's
// (node-semver's) syntax, which package.json's engines use. Comparators
// (>=22, <23.1, =22.4.0), partial versions (22, 22.x, *), ~ and ^, hyphens
// (20 - 22), all of a space-separated set, any set of ||. A rangeFiles
// entry gives one (docs/guide/versions.md).

// bound is one comparator: an operator and (major, minor, patch).
type bound struct {
	operator string
	numbers  [3]string
}

var (
	rangePartial  = regexp.MustCompile(`^v?([0-9]+|[xX*])?(?:\.([0-9]+|[xX*]))?(?:\.([0-9]+|[xX*]))?(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	rangeOperator = regexp.MustCompile(`^(<=|>=|<|>|=|~>|~|\^)?(.*)$`)
	rangeSpaced   = pyRegexp(`(<=|>=|<|>|=|~>|~|\^)\s+`)
	rangeHyphen   = pyRegexp(`^(\S+)\s+-\s+(\S+)$`)
)

// partialNumbers is versions.py's _partial: the numbers a partial version
// gives, up to the first wildcard; ok is false for no version.
func partialNumbers(text string) ([]string, bool) {
	match := rangePartial.FindStringSubmatch(text)
	if match == nil {
		return nil, false
	}
	var numbers []string
	for _, part := range match[1:] {
		if part == "" || part == "x" || part == "X" || part == "*" {
			break
		}
		numbers = append(numbers, pyInt(part))
	}
	return numbers, true
}

func padded(numbers []string) [3]string {
	result := [3]string{"0", "0", "0"}
	copy(result[:], numbers)
	return result
}

// following is versions.py's _next: the first version past a partial one
// (22 is followed by 23.0.0, 22.4 by 22.5.0).
func following(numbers []string) [3]string {
	bumped := append([]string{}, numbers...)
	last, _ := new(big.Int).SetString(bumped[len(bumped)-1], 10)
	bumped[len(bumped)-1] = last.Add(last, big.NewInt(1)).String()
	return padded(bumped)
}

// comparators is versions.py's _comparators for one token.
func comparators(token string) ([]bound, bool) {
	match := rangeOperator.FindStringSubmatch(token)
	operator := match[1]
	numbers, ok := partialNumbers(match[2])
	if !ok {
		return nil, false
	}
	n := len(numbers)
	if n == 0 {
		if operator == "<" || operator == ">" {
			return []bound{{"<", padded(nil)}}, true
		}
		return []bound{}, true
	}
	low := padded(numbers)
	switch operator {
	case "", "=":
		if n == 3 {
			return []bound{{">=", low}, {"<=", low}}, true
		}
		return []bound{{">=", low}, {"<", following(numbers)}}, true
	case ">=":
		return []bound{{">=", low}}, true
	case ">":
		if n == 3 {
			return []bound{{">", low}}, true
		}
		return []bound{{">=", following(numbers)}}, true
	case "<":
		return []bound{{"<", low}}, true
	case "<=":
		if n == 3 {
			return []bound{{"<=", low}}, true
		}
		return []bound{{"<", following(numbers)}}, true
	case "~", "~>":
		return []bound{{">=", low}, {"<", following(numbers[:min(n, 2)])}}, true
	}
	// ^: the leftmost non-zero part stays fixed (^0.2.3 is below 0.3.0).
	fixed := n - 1
	for i, part := range numbers {
		if part != "0" {
			fixed = i
			break
		}
	}
	return []bound{{">=", low}, {"<", following(numbers[:fixed+1])}}, true
}

func hyphenRange(low, high string) ([]bound, bool) {
	first, ok := partialNumbers(low)
	if !ok {
		return nil, false
	}
	last, ok := partialNumbers(high)
	if !ok {
		return nil, false
	}
	bounds := []bound{}
	if len(first) > 0 {
		bounds = append(bounds, bound{">=", padded(first)})
	}
	switch {
	case len(last) == 3:
		bounds = append(bounds, bound{"<=", padded(last)})
	case len(last) > 0:
		bounds = append(bounds, bound{"<", following(last)})
	}
	return bounds, true
}

// parseRange is versions.py's parse_range: a range's sets of comparators;
// ok is false when it isn't one.
func parseRange(text string) ([][]bound, bool) {
	var sets [][]bound
	for _, alternative := range strings.Split(text, "||") {
		alternative = rangeSpaced.ReplaceAllString(pyStrip(alternative), "$1")
		var bounds []bound
		if hyphen := rangeHyphen.FindStringSubmatch(alternative); hyphen != nil {
			found, ok := hyphenRange(hyphen[1], hyphen[2])
			if !ok {
				return nil, false
			}
			bounds = found
		} else {
			bounds = []bound{}
			for _, token := range pyFields(alternative) {
				found, ok := comparators(token)
				if !ok {
					return nil, false
				}
				bounds = append(bounds, found...)
			}
		}
		sets = append(sets, bounds)
	}
	return sets, true
}

// satisfies is versions.py's satisfies: whether a release is in a range; a
// pre-release never is.
func satisfies(text, versionText string) bool {
	sets, ok := parseRange(text)
	v, parsed := parseVersion(versionText)
	if !ok || !parsed || v.pre != "" {
		return false
	}
	numbers := padded(prefixStrings(v.numbers, 3))
	holds := func(b bound) bool {
		c := compareNumbers(numbers[:], b.numbers[:])
		switch b.operator {
		case "<":
			return c < 0
		case "<=":
			return c <= 0
		case ">":
			return c > 0
		}
		return c >= 0
	}
	for _, bounds := range sets {
		all := true
		for _, b := range bounds {
			if !holds(b) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}
