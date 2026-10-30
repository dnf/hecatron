"""docs/real-projects.md and the real-project data files (tests/real-projects/)."""

import importlib.util
import json
import os
import sys
import unittest

SCRIPTS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "real-projects")


def load_script(name):
    """Load tests/real-projects/<name>.py under its own module name: a plain
    import would share `report` with tests/real-apps/report.py, and whichever
    test module ran first would win."""
    module_name = "real_projects_" + name
    if module_name in sys.modules:
        return sys.modules[module_name]
    spec = importlib.util.spec_from_file_location(module_name, os.path.join(SCRIPTS, name + ".py"))
    module = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = module
    spec.loader.exec_module(module)
    return module


report = load_script("report")

LOG = """\
### meta {"repo": "acme/widget", "date": "2026-10-28", "hecatron": {"version": "0.5.0", "commit": "abc123", "runner": "binary"}, "runner": {"os": "Ubuntu 24.04.3 LTS", "arch": "x86_64", "image": "ubuntu:24.04", "host": "local", "runUrl": ""}, "actions": "test"}
### step clone: git clone ...
### done clone rc=0 secs=2
### project {"commit": "0123456789abcdef", "root": "/home/dev/acme_widget"}
### step init: hct init
Initialised /home/dev/acme_widget
  languages:  go, python, rust
  not added: c (found only in examples, tests or vendored code)
### done init rc=0 secs=1
### json config {"languages": ["go", "python", "rust"]}
### json status {"languages": [{"name": "go", "components": [{"name": ".", "dir": "go"}], "versions": [{"spec": "1.24", "exact": false, "sources": ["pin:go/go.mod"]}], "toolchain": "installed"}]}
### step install: hct install
Failed to install: rust
### done install rc=1 secs=300
### step doctor: hct doctor
### done doctor rc=1 secs=3
### step test:go: bash -c ...
+ go test ./...
ok  \texample.com/widget\t0.01s
FAIL\texample.com/widget/sub\t0.02s
### done test:go rc=1 secs=12
### json test:go {"logs": "/home/dev/acme_widget/.logs/test", "jobs": [{"language": "go", "status": "failed", "exitCode": 1, "components": [{"name": ".", "dir": "/home/dev/acme_widget/go", "failed": true}], "errors": [{"text": "--- FAIL: TestAdd (0.00s)"}]}]}
### step test:python: bash -c ...
+ uv run --with pytest pytest
===== 10 passed, 2 skipped in 1.50s =====
### done test:python rc=0 secs=5
### json test:python null
### step test:rust: bash -c ...
bash: line 1: cargo: command not found
### done test:rust rc=3 secs=0
### json test:rust {"jobs": [{"language": "rust", "status": "failed", "exitCode": 3, "components": []}]}
### end {"wallSeconds": 330}
"""


class ParseTest(unittest.TestCase):
    def setUp(self):
        self.run = report.parse_log(LOG)
        self.langs = {lang["language"]: lang for lang in self.run["languages"]}

    def test_run_context(self):
        self.assertEqual(self.run["repo"], "acme/widget")
        self.assertEqual(self.run["commit"], "0123456789abcdef")
        self.assertTrue(self.run["complete"])
        self.assertEqual(self.run["install"], {"rc": 1, "seconds": 300})
        self.assertEqual(self.run["init"]["notAdded"], [{"language": "c", "reason": "found only in examples, tests or vendored code"}])
        self.assertEqual(self.run["init"]["components"], [{"name": ".", "dirs": {"go": "go"}}])

    def test_each_language(self):
        go = self.langs["go"]
        self.assertEqual(go["result"], "fail")
        self.assertEqual(go["buildSystems"], ["go"])
        self.assertEqual(go["components"], [{"name": ".", "dir": "go", "failed": True}])
        self.assertEqual(go["toolchain"]["versions"][0]["sources"], ["pin:go/go.mod"])
        test = go["steps"]["test"]
        self.assertEqual(test["counts"], {"tool": "go (packages)", "passed": 1, "failed": 1})
        self.assertEqual(test["failure"], {"category": "tests", "excerpt": ["--- FAIL: TestAdd (0.00s)"]})
        python = self.langs["python"]
        self.assertEqual(python["result"], "pass")
        self.assertEqual(python["steps"]["test"]["counts"], {"tool": "pytest", "passed": 10, "failed": 0, "skipped": 2})
        self.assertEqual(self.langs["rust"]["steps"]["test"]["failure"]["category"], "toolchain")

    def test_a_cut_off_log_is_incomplete_and_the_rest_not_run(self):
        run = report.parse_log(LOG.split("### step test:python")[0])
        self.assertFalse(run["complete"])
        self.assertEqual([lang["result"] for lang in run["languages"]], ["fail", "not-run", "not-run"])

    def test_the_run_validates(self):
        with open(report.SCHEMA, encoding="utf-8") as f:
            schema = json.load(f)
        data = {"schemaVersion": 1, "results": [self.run]}
        self.assertEqual(report.validate(data, schema["$defs"]["resultsFile"], schema), [])


class CountsTest(unittest.TestCase):
    def test_runners(self):
        self.assertEqual(report.counts("test result: ok. 5 passed; 1 failed; 2 ignored; 0 measured\n"
                                       "test result: ok. 3 passed; 0 failed; 0 ignored; 0 measured\n"),
                         {"tool": "cargo", "passed": 8, "failed": 1, "skipped": 2})
        self.assertEqual(report.counts("Passed!  - Failed:     0, Passed:    12, Skipped:     1, Total:    13, Duration: 1 s"),
                         {"tool": "dotnet", "passed": 12, "failed": 0, "skipped": 1, "total": 13})
        self.assertEqual(report.counts("[INFO] Tests run: 40, Failures: 1, Errors: 1, Skipped: 3\n"),
                         {"tool": "maven", "passed": 35, "failed": 2, "skipped": 3, "total": 40})
        self.assertEqual(report.counts("12 examples, 0 failures\n"), {"tool": "rspec", "passed": 12, "failed": 0, "total": 12})
        self.assertEqual(report.counts("00:02 +41 ~1: All tests passed!"), {"tool": "dart", "passed": 41, "failed": 0, "skipped": 1})
        self.assertEqual(report.counts("Test run summary: Passed!\n  total: 222\n  failed: 0\n  succeeded: 222\n  skipped: 0\n"),
                         {"tool": "dotnet", "passed": 222, "failed": 0, "skipped": 0, "total": 222})
        self.assertIsNone(report.counts("nothing to see"))

    # Summaries as the first run's logs show them (tests/real-projects, 2026-10-28).
    def test_julia(self):
        passing = ("Test Summary:  |   Pass   Total     Time\n"
                   "DataStructures | 290238  290238  1m37.0s\n"
                   "     Testing DataStructures tests passed \n")
        self.assertEqual(report.counts(passing), {"tool": "julia", "passed": 290238, "failed": 0, "total": 290238})
        # Columns show only when used; numbers are right-aligned to them.
        failing = ("Test Summary: | Pass  Fail  Error  Broken  Total  Time\n"
                   "Widgets       |   40     2      1       3     46  2.1s\n"
                   "  shapes      |   10     2              1     13  0.3s\n")
        self.assertEqual(report.counts(failing), {"tool": "julia", "passed": 40, "failed": 3, "skipped": 3, "total": 46})

    def test_gradle(self):
        # Gradle prints counts only when a test fails...
        self.assertEqual(report.counts("> Task :lib:test FAILED\n\n42 tests completed, 2 failed, 1 skipped\n\nFAILURE: Build failed with an exception.\n"),
                         {"tool": "gradle", "passed": 39, "failed": 2, "skipped": 1, "total": 42})
        # ...else run.sh totals the JUnit XML it wrote.
        self.assertIsNone(report.counts("> Task :test\nBUILD SUCCESSFUL in 1m 6s\n10 actionable tasks: 10 executed\n"))
        self.assertEqual(report.junit_counts({"files": 3, "tests": 120, "failures": 1, "errors": 0, "skipped": 4}),
                         {"tool": "junit-xml", "passed": 115, "failed": 1, "skipped": 4, "total": 120})
        self.assertIsNone(report.junit_counts(None))
        self.assertIsNone(report.junit_counts({"files": 0}))

    def test_more_runners(self):
        cases = [
            # surefire 2 prints no [INFO]; a class's own line ends in its time
            ("Tests run: 1, Failures: 1, Errors: 0, Skipped: 0, Time elapsed: 0.083 sec <<< FAILURE!\n"
             "Results :\n\nTests run: 2, Failures: 2, Errors: 0, Skipped: 0\n",
             {"tool": "maven", "passed": 0, "failed": 2, "skipped": 0, "total": 2}),
            ("Build Summary: 39/39 steps succeeded; 583/594 tests passed (11 skipped)\n",
             {"tool": "zig", "passed": 583, "failed": 0, "skipped": 11, "total": 594}),
            ("Build Summary: 3/5 steps succeeded; 1 failed; 10/12 tests passed; 2 failed\n",
             {"tool": "zig", "passed": 10, "failed": 2, "total": 12}),
            ("[==========] 327 tests from 16 test suites ran. (1630 ms total)\n[  PASSED  ] 327 tests.\n",
             {"tool": "gtest", "passed": 327, "failed": 0}),
            ("[info] Passed: Total 121, Failed 0, Errors 0, Passed 121\n", {"tool": "sbt", "passed": 121, "failed": 0, "total": 121}),
            ("[info] Tests: succeeded 92, failed 0, canceled 0, ignored 0, pending 0\n",
             {"tool": "scalatest", "passed": 92, "failed": 0, "skipped": 0}),
            ("Tests: 130, Assertions: 227, PHPUnit Notices: 7.\n", {"tool": "phpunit", "passed": 130, "failed": 0, "total": 130}),
            ("193 tests, 1508 assertions, 0 failures, 0 errors, 0 pendings, 0 omissions, 0 notifications\n",
             {"tool": "test-unit", "passed": 193, "failed": 0, "skipped": 0, "total": 193}),
            ("26 doctests, 9 properties, 410 tests, 0 failures\n", {"tool": "exunit", "passed": 410, "failed": 0, "total": 410}),
            ("Ran 55 tests containing 293 assertions.\n0 failures, 0 errors.\n", {"tool": "clojure.test", "passed": 55, "failed": 0, "total": 55}),
            ("ℹ tests 22\nℹ suites 0\nℹ pass 0\nℹ fail 22\n", {"tool": "node:test", "passed": 0, "failed": 22}),
            ("  ✔ type › subtype alias\n  ─\n\n  44 tests passed\n", {"tool": "ava", "passed": 44, "failed": 0}),
            ("TEST RUN PASSED\nDuration: 89 ms\nPassed:   3\nFailed:   0\n", {"tool": "elm-test", "passed": 3, "failed": 0}),
            ("Tests completed in 33.09s\nTests Passed: 261, Failed: 7, Skipped: 17, Inconclusive: 0, NotRun: 3\n",
             {"tool": "pester", "passed": 261, "failed": 7, "skipped": 17}),
            ("116 run, 116 passes, 0 failures, 0 errors.\n", {"tool": "pharo", "passed": 116, "failed": 0, "total": 116}),
            ("35 test(s) run, 0 failed, 2 hidden, 4/4 failing as expected.\n", {"tool": "unit-threaded", "passed": 35, "failed": 0, "total": 35}),
            ("Test Successful in 0.005s. 56 tests run.\n", {"tool": "alcotest", "passed": 56, "failed": 0}),
            ("Ran: 16 tests in: 0.13 seconds.\nOK\n", {"tool": "ounit", "passed": 16, "failed": 0, "total": 16}),
            ("% 6 tests failed\n% 25 tests passed\n", {"tool": "plunit", "passed": 25, "failed": 6}),
            ("1/276 test failures\n", {"tool": "raco", "passed": 275, "failed": 1, "total": 276}),
            ("ok 1 a\nok 2 b\nnot ok 3 c\n", {"tool": "tap", "passed": 2, "failed": 1}),
        ]
        for text, want in cases:
            with self.subTest(text=text):
                self.assertEqual(report.counts(text), want)


class FailureCategoryTest(unittest.TestCase):
    """Failures the first run left as "other", with the lines that place them."""

    def category(self, text, action="test", found=None):
        return report.failure_category(1, text, None, False, found if found is not None else report.counts(text), action)

    def test_causes_outside_the_project(self):
        cases = [
            ("disk", "error: could not write: No space left on device (os error 28)"),
            ("network", "fatal: unable to access 'https://x/': Could not resolve host: x"),
            ("toolchain", "/hecatron/languages/typescript/hooks/../../_hooks.sh: line 44: nub: command not found"),
            ("toolchain-version", "===> OTP release 26 or later is required. Version in use: 25.3.2.8"),
            ("toolchain-version", "error: the configured Python version (3.15) is newer than PyO3's maximum supported version (3.14)"),
            ("toolchain-version", "build.zig:87:27: error: no field named 'ReleaseSafe' in enum 'lang.Optimize'"),
            ("system", "./generate.sh: line 12: rsync: command not found"),
            ("system", "/bin/sh: 1: wasm-pack: not found"),
            ("system", "meson.build:1:0: ERROR: Unknown compiler(s): [['cython'], ['cython3']]"),
            ("system", "Gem::Ext::BuildError: ERROR: Failed to build gem native extension."),
            ("system", "System.IO.FileNotFoundException: Could not find 'mono' host. Make sure that 'mono' is installed on the machine"),
            ("system", "  Could not find a package configuration file provided by \"absl\" with any of"),
            ("system", "bindings.c:6:10: fatal error: tree_sitter/api.h: No such file or directory"),
        ]
        for want, text in cases:
            with self.subTest(text=text):
                self.assertEqual(self.category(text), want)

    def test_causes_in_the_project(self):
        cases = [
            ("dependencies", "error: No solution found when resolving dependencies for split (markers: sys_platform != 'emscripten')"),
            ("dependencies", "Error: ERR_PNPM_IGNORED_BUILDS"),
            ("setup", "[ERROR] Child module /w/java/lite/pom.xml of /w/java/pom.xml does not exist @"),
            ("setup", "go: updates to go.mod needed; to update it:"),
            ("setup", "test/test_const.erl:25:10: can't find include file \"gen-erl/constants_demo_types.hrl\""),
            ("build", "error: SwiftCompile normal x86_64 failed with a nonzero exit code."),
            ("build", "Fatal: Compilation aborted"),
            ("no-tests", "collected 0 items\n============================ no tests ran in 0.02s ============================="),
            ("no-tests", "Test project /w/build\nNo tests were found!!!"),
            ("tests", "Tests Passed: 5393, Failed: 87, Skipped: 0, Inconclusive: 0, NotRun: 0"),
            ("tests", "not ok 3 -- stops option parsing before test paths\n# /usr/lib/bats-core/test_functions.bash: line 161: f: command not found"),
        ]
        for want, text in cases:
            with self.subTest(text=text):
                self.assertEqual(self.category(text), want)

    def test_lint_and_build_steps(self):
        self.assertEqual(self.category("src/a.py:1:1: E501 line too long", action="lint"), "lint")
        self.assertEqual(self.category("1 failed", action="build"), "other")
        self.assertEqual(self.category("error[E0425]: cannot find value", action="build"), "build")
        self.assertEqual(self.category("1 failed", action="test"), "tests")


LINT_BUILD_LOG = """\
### meta {"repo": "acme/gadget", "date": "2026-11-03", "hecatron": {"version": "0.6.0", "commit": "def456", "runner": "binary"}, "runner": {"os": "Ubuntu 24.04.3 LTS", "arch": "x86_64", "image": "ubuntu:24.04", "host": "local", "runUrl": ""}, "actions": "test,lint,build"}
### disk start {"freeKB": 20971520, "sizeKB": 76546048}
### step clone: git clone ...
### done clone rc=0 secs=2
### disk clone {"freeKB": 20000000, "sizeKB": 76546048}
### project {"commit": "fedcba9876543210", "root": "/home/dev/acme_gadget"}
### step init: hct init
### done init rc=0 secs=1
### disk init {"freeKB": 20000000, "sizeKB": 76546048}
### json config {"languages": ["java"]}
### step install: hct install
Failed to fetch dependencies: java (retry with: hct deps java)
### done install rc=1 secs=200
### disk install {"freeKB": 3000000, "sizeKB": 76546048}
### step test:java: bash -c ...
+ ./gradlew test
BUILD SUCCESSFUL in 1m 6s
### done test:java rc=0 secs=70
### disk test:java {"freeKB": 1500000, "sizeKB": 76546048}
### json test:java null
### junit test:java {"files": 4, "tests": 30, "failures": 0, "errors": 0, "skipped": 2}
### step lint:java: bash -c ...
+ ./gradlew check -x test
src/main/java/A.java:3: [UnusedImports] Unused import
### done lint:java rc=1 secs=20
### disk lint:java {"freeKB": 1400000, "sizeKB": 76546048}
### json lint:java null
### step build:java: bash -c ...
+ ./gradlew assemble
BUILD SUCCESSFUL in 10s
### done build:java rc=0 secs=10
### disk build:java {"freeKB": 1450000, "sizeKB": 76546048}
### json build:java null
### disk end {"freeKB": 1450000, "sizeKB": 76546048}
### end {"wallSeconds": 310}
"""


class ActionsAndDiskTest(unittest.TestCase):
    def setUp(self):
        self.run = report.parse_log(LINT_BUILD_LOG)
        self.java = self.run["languages"][0]

    def test_each_action_is_a_step_and_the_tests_decide(self):
        self.assertEqual(self.run["actions"], ["test", "lint", "build"])
        self.assertEqual(sorted(self.java["steps"]), ["build", "lint", "test"])
        self.assertEqual(self.java["steps"]["lint"]["failure"]["category"], "lint")
        self.assertIsNone(self.java["steps"]["build"]["failure"])
        self.assertEqual(self.java["result"], "pass")
        self.assertEqual(self.java["install"], {"toolchainFailed": False, "depsFailed": True})

    def test_gradle_counts_come_from_its_junit_reports(self):
        self.assertEqual(self.java["steps"]["test"]["counts"], {"tool": "junit-xml", "passed": 28, "failed": 0, "skipped": 2, "total": 30})

    def test_disk_space(self):
        self.assertEqual(self.run["disk"], {"sizeKB": 76546048, "freeBeforeKB": 20971520, "freeAfterKB": 1450000,
                                            "minFreeKB": 1400000, "minFreeAfter": "lint:java", "low": True})
        self.assertIsNone(report.parse_log(LOG)["disk"])

    def test_the_run_validates(self):
        with open(report.SCHEMA, encoding="utf-8") as f:
            schema = json.load(f)
        self.assertEqual(report.validate({"schemaVersion": 1, "results": [self.run]}, schema["$defs"]["resultsFile"], schema), [])

    def test_the_page_shows_lint_build_and_disk(self):
        projects = {"verified": "2026-11-03", "projects": [
            {"repo": "acme/gadget", "category": "single", "languages": ["java"], "why": "", "github": {}}]}
        page = report.render_text(projects, {"acme/gadget": self.run})
        self.assertIn("| lint | 0/1 |", page)
        self.assertIn("| build | 1/1 |", page)
        self.assertIn("| [acme/gadget](https://github.com/acme/gadget) | 1.3 GB | lint:java | 20.0 GB | 73.0 GB |", page)
        row = report.rows(projects, {"acme/gadget": self.run})[0]
        self.assertEqual(row["actions"], {"lint": "fail", "build": "pass"})


class SchemaTest(unittest.TestCase):
    def test_the_data_files_match_the_schema(self):
        self.assertEqual(report.validate_files(), [])

    def test_the_validator_catches_mistakes(self):
        with open(report.SCHEMA, encoding="utf-8") as f:
            schema = json.load(f)
        bad = {"schemaVersion": 1, "rows": [{"repo": "not a repo"}]}
        self.assertTrue(report.validate(bad, schema["$defs"]["byLanguageFile"], schema))


class BacklogTest(unittest.TestCase):
    def test_every_project_is_listed_once_with_known_languages(self):
        projects, _ = report.load_all()
        repos = [p["repo"].lower() for p in projects["projects"]]
        self.assertEqual(len(repos), len(set(repos)))
        known = set(report.supported_languages())
        for project in projects["projects"]:
            self.assertLessEqual(set(project["languages"]), known, project["repo"])

    def test_every_supported_language_has_a_project(self):
        projects, _ = report.load_all()
        covered = {lang for p in projects["projects"] for lang in p["languages"]}
        self.assertEqual(set(report.supported_languages()) - covered, set())


class DocTest(unittest.TestCase):
    def test_the_page_and_the_flat_file_are_generated_from_the_data(self):
        projects, results = report.load_all()
        with open(report.DOC, encoding="utf-8") as f:
            self.assertEqual(f.read(), report.render_text(projects, results),
                             "docs/real-projects.md is out of date: python3 tests/real-projects/report.py render")
        with open(report.BY_LANGUAGE, encoding="utf-8") as f:
            self.assertEqual(json.load(f), report.by_language_data(projects, results),
                             "results-by-language.json is out of date: python3 tests/real-projects/report.py render")

    def test_each_tested_project_has_its_badge(self):
        projects, results = report.load_all()
        wanted = report.badges(projects, results)
        on_disk = {}
        for name in os.listdir(report.BADGES):
            with open(os.path.join(report.BADGES, name), encoding="utf-8") as f:
                on_disk[name] = json.load(f)
        self.assertEqual(on_disk, wanted, "badges/ is out of date: python3 tests/real-projects/report.py render")

    def test_a_badge_says_how_many_languages_pass(self):
        def row(result):
            return {"detected": True, "result": result}
        self.assertEqual(report.badge([row("pass"), row("pass")])["message"], "passing")
        self.assertEqual(report.badge([row("pass"), row("fail")])["message"], "1/2 languages")
        self.assertEqual(report.badge([row("fail")])["color"], "red")
        self.assertIsNone(report.badge([{"detected": None, "result": "untested"}]))


if __name__ == "__main__":
    unittest.main()
