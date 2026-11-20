package runner

import (
	"fmt"
	"regexp"
	"strings"
)

// A test job's tally is what its test runners said they ran: each one's own
// result line ("246 passed, 15 skipped in 0.14s", "64 examples, 0
// failures", "test result: ok. 12 passed; ..."), as agent and JSON output
// report it, so an agent asked how many tests passed doesn't read the logs
// or run the tests again to count them.

// MaxTally caps a job's tally lines.
const MaxTally = 6

type tallyPattern struct {
	re *regexp.Regexp
	// counts: the runner prints one count a line (node:test's "ℹ tests
	// 3", "ℹ pass 1"), which make one tally line; else the last line a
	// runner prints is its total.
	counts bool
}

// tallyPatterns are test runners' result lines: the text is the first
// group when there is one (pytest's, without its rule of = signs), else
// the line.
var tallyPatterns = []tallyPattern{
	// pytest: "===== 246 passed, 15 skipped in 0.14s ====="
	{re: regexp.MustCompile(`^=+ (\d+ (?:passed|failed|errors?|skipped|deselected|xfailed|xpassed)\b.*?) =+$`)},
	// RSpec "3 examples, 2 failures", minitest "3 runs, 2 assertions, ...",
	// ExUnit "1 doctest, 3 tests, 2 failures", bats "3 tests, 0 failures"
	{re: regexp.MustCompile(`^(?:\d+ \w+, )*\d+ (?:examples?|runs?|tests?|doctests?|propert(?:y|ies)), \d+ (?:failures?|assertions?)\b.*$`)},
	// cargo: "test result: ok. 12 passed; 0 failed; ..."
	{re: regexp.MustCompile(`^test result: .*$`)},
	// Jest "Tests: 2 failed, 1 passed, 3 total", Vitest "Tests 2 failed | 1 passed (3)"
	{re: regexp.MustCompile(`^Tests:?\s+\d+ (?:passed|failed|skipped|todo)\b.*$`)},
	// node:test "ℹ tests 3", TAP "# pass 1"
	{re: regexp.MustCompile(`^(?:ℹ|#) ((?:tests|pass|fail|skipped|todo|cancelled) (\d+))$`), counts: true},
	// Maven/Surefire's total: "[ERROR] Tests run: 2, Failures: 1, Errors: 0, Skipped: 0"
	{re: regexp.MustCompile(`^(?:\[\w+\] )?(Tests run: \d+, Failures: \d+, Errors: \d+, Skipped: \d+)$`)},
	// Gradle: "2 tests completed, 1 failed"
	{re: regexp.MustCompile(`^\d+ tests? completed\b.*$`)},
	// dotnet test: "Failed!  - Failed: 1, Passed: 1, Skipped: 0, Total: 2, Duration: 29 ms - Calc.Tests.dll (net10.0)"
	{re: regexp.MustCompile(`^(?:Passed|Failed)!\s+-\s+Failed:.*$`)},
	// PHPUnit: "OK (3 tests, 5 assertions)", "Tests: 3, Assertions: 2, Errors: 1, Failures: 1."
	{re: regexp.MustCompile(`^(?:OK \(\d+ tests?\b.*\)|Tests: \d+, Assertions: .*)$`)},
	// XCTest "Executed 12 tests, with 0 failures (0 unexpected) in 0.1 (0.1) seconds",
	// Swift Testing "✔ Test run with 12 tests passed after 0.1 seconds."
	{re: regexp.MustCompile(`^(?:\S+ )?(Executed \d+ tests?, with .*|Test run with \d+ tests?\b.*)$`)},
	// busted: "12 successes / 0 failures / 0 errors / 0 pending : 0.01 seconds"
	{re: regexp.MustCompile(`^\d+ successe?s? / \d+ failures? / .*$`)},
	// CTest: "100% tests passed, 0 tests failed out of 12"
	{re: regexp.MustCompile(`^\d+% tests passed, .*$`)},
	// Python's unittest: "Ran 12 tests in 0.010s"
	{re: regexp.MustCompile(`^Ran \d+ tests? in .*$`)},
	// zig build test: "All 12 tests passed."
	{re: regexp.MustCompile(`^All \d+ tests passed\.?$`)},
}

// goPackage is go test's line for a package, its fields tab-separated
// ("ok  \texample.com/calc\t0.003s", "FAIL\texample.com/calc\t0.003s"),
// unlike Jest's "FAIL ./calc.test.js".
var goPackage = regexp.MustCompile(`^(ok|FAIL) *\t(\S+)(?:\t(.*))?$`)

var spaces = regexp.MustCompile(`\s+`)

// Tally is what a test job's log says its runners ran: one line per runner
// and component, prefixed with the component when the log has several.
func Tally(log []byte) []string {
	lines, components := logLines(log)
	type entry struct {
		component, key, text string
	}
	var order []entry
	at := map[string]int{}
	goOK := map[string]int{}
	var goOnly = map[string]string{}
	var goFailed = map[string][]string{}
	set := func(component, key, text string) {
		id := component + "\x00" + key
		if i, ok := at[id]; ok {
			order[i].text = text
			return
		}
		at[id] = len(order)
		order = append(order, entry{component, key, text})
	}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		component := components[i]
		if m := goPackage.FindStringSubmatch(line); m != nil {
			if m[1] == "ok" {
				goOK[component]++
				goOnly[component] = spaces.ReplaceAllString(line, " ")
			} else {
				goFailed[component] = append(goFailed[component], m[2])
			}
			set(component, "go", "")
			continue
		}
		for n, p := range tallyPatterns {
			m := p.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			text := m[0]
			if len(m) > 1 && m[1] != "" {
				text = m[1]
			}
			key := fmt.Sprint(n)
			if p.counts {
				// One line of counts, leaving out the noteworthy-only ones at 0.
				if m[2] == "0" && !strings.HasPrefix(text, "fail") && !strings.HasPrefix(text, "pass") {
					break
				}
				if i, ok := at[component+"\x00"+key]; ok {
					text = order[i].text + ", " + text
				}
			}
			set(component, key, spaces.ReplaceAllString(text, " "))
			break
		}
	}
	several := false
	for _, e := range order {
		if e.component != order[0].component {
			several = true
		}
	}
	var tally []string
	for _, e := range order {
		text := e.text
		if e.key == "go" {
			text = goTally(goOK[e.component], goOnly[e.component], goFailed[e.component])
		}
		if several && e.component != "" {
			text = e.component + ": " + text
		}
		tally = append(tally, truncate(text))
		if len(tally) == MaxTally {
			break
		}
	}
	return tally
}

// goTally sums go test's package lines: the one line for one package that
// passed, else how many passed and which failed.
func goTally(ok int, only string, failed []string) string {
	if ok == 1 && len(failed) == 0 {
		return only
	}
	var parts []string
	if ok > 0 {
		parts = append(parts, fmt.Sprintf("%d packages ok", ok))
	}
	if len(failed) > 0 {
		parts = append(parts, fmt.Sprintf("%d failed: %s", len(failed), strings.Join(failed, ", ")))
	}
	return "go test: " + strings.Join(parts, ", ")
}
