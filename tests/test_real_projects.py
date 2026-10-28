"""docs/real-projects.md and the real-project data files (tests/real-projects/)."""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "real-projects"))
import report  # noqa: E402

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
