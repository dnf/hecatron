package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each runner's result line, from the recorded samples (failing runs) and
// the forms a passing run prints.
func TestTally(t *testing.T) {
	samples := map[string]string{
		"cargo/1.99.1/rust-test-fail.log":        "test result: FAILED. 1 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s",
		"dotnet/10.0.113/dotnet-test-fail.log":   "Failed! - Failed: 1, Passed: 1, Skipped: 0, Total: 2, Duration: 29 ms - Calc.Tests.dll (net10.0)",
		"go/1.27.3/go-test-fail.log":             "go test: 1 failed: example.com/calc",
		"gradle/9.1.0/gradle-junit-fail.log":     "2 tests completed, 1 failed",
		"jest/30.1.3/jest-test-fail.log":         "Tests: 2 failed, 1 passed, 3 total",
		"maven/3.9.11/maven-junit-fail.log":      "Tests run: 2, Failures: 1, Errors: 0, Skipped: 0",
		"minitest/5.16.3/minitest-test-fail.log": "3 runs, 2 assertions, 1 failures, 1 errors, 0 skips",
		"mix/1.14.0/exunit-test-fail.log":        "3 tests, 2 failures",
		"node-test/24.22.1/node-test-fail.log":   "tests 3, pass 1, fail 2",
		"phpunit/11.5.57/phpunit-test-fail.log":  "Tests: 3, Assertions: 2, Errors: 1, Failures: 1.",
		"pytest/9.1.1/python-test-fail.log":      "2 failed, 1 passed in 0.05s",
		"rspec/3.13.6/rspec-test-fail.log":       "3 examples, 2 failures",
		"vitest/3.2.4/vitest-test-fail.log":      "Tests 2 failed | 1 passed (3)",
	}
	for path, want := range samples {
		log, err := os.ReadFile(filepath.Join("..", "..", "tests", "output", path))
		if err != nil {
			t.Fatal(err)
		}
		if got := Tally(log); len(got) != 1 || got[0] != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
	passing := map[string]string{
		"======================= 246 passed, 15 skipped in 0.14s ========================":           "246 passed, 15 skipped in 0.14s",
		"ok  \tgithub.com/cucumber/tag-expressions/go/v11\t(cached)":                                 "ok github.com/cucumber/tag-expressions/go/v11 (cached)",
		"ok  \texample.com/a\t0.1s\nok  \texample.com/b\t0.2s\n?   \texample.com/c\t[no test files]": "go test: 2 packages ok",
		"64 examples, 0 failures":                                                "64 examples, 0 failures",
		"1 doctest, 342 tests, 0 failures":                                       "1 doctest, 342 tests, 0 failures",
		"test result: ok. 12 passed; 0 failed; 0 ignored":                        "test result: ok. 12 passed; 0 failed; 0 ignored",
		"ℹ tests 5\nℹ suites 0\nℹ pass 5\nℹ fail 0\nℹ cancelled 0\nℹ todo 0":     "tests 5, pass 5, fail 0",
		"OK (12 tests, 30 assertions)":                                           "OK (12 tests, 30 assertions)",
		"Executed 12 tests, with 0 failures (0 unexpected) in 0.1 (0.1) seconds": "Executed 12 tests, with 0 failures (0 unexpected) in 0.1 (0.1) seconds",
		"✔ Test run with 12 tests passed after 0.1 seconds.":                     "Test run with 12 tests passed after 0.1 seconds.",
		"12 successes / 0 failures / 0 errors / 0 pending : 0.01 seconds":        "12 successes / 0 failures / 0 errors / 0 pending : 0.01 seconds",
		"100% tests passed, 0 tests failed out of 12":                            "100% tests passed, 0 tests failed out of 12",
		"Ran 12 tests in 0.010s\n\nOK":                                           "Ran 12 tests in 0.010s",
		"All 12 tests passed.":                                                   "All 12 tests passed.",
		// Surefire prints one per class, then the total: the last is kept.
		"[INFO] Tests run: 1, Failures: 0, Errors: 0, Skipped: 0, Time elapsed: 0.1 s -- in a.ATest\n[INFO] Tests run: 3, Failures: 0, Errors: 0, Skipped: 0": "Tests run: 3, Failures: 0, Errors: 0, Skipped: 0",
	}
	for log, want := range passing {
		if got := Tally([]byte(log)); len(got) != 1 || got[0] != want {
			t.Errorf("%q: %q, want %q", log, got, want)
		}
	}
	// Several components: each one's, named.
	log := "=== test web (ruby) ===\n+ bundle exec rspec\n3 examples, 0 failures\nruby: test passed for 1 component(s)\n" +
		"=== test api (ruby) ===\n+ bundle exec rspec\n5 examples, 1 failure\n"
	if got := strings.Join(Tally([]byte(log)), " | "); got != "web: 3 examples, 0 failures | api: 5 examples, 1 failure" {
		t.Errorf("components: %q", got)
	}
	if got := Tally([]byte("compiling\nall good\n")); len(got) != 0 {
		t.Errorf("no runner: %q", got)
	}
}
