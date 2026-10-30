"""The real-apps workflow's bookkeeping (tests/real-apps/report.py): whose
turn it is, a run's outcome, and the issue that reports it."""

from __future__ import annotations

import contextlib
import io
import json
import os
import re
import sys
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests", "real-apps"))

import report  # noqa: E402

APPS = os.path.join(HOME, "tests", "real-apps", "apps.txt")


class ListTest(unittest.TestCase):
    def test_each_entry_is_an_owner_and_repository_once(self):
        listed = report.apps(APPS)
        self.assertGreater(len(listed), 30)
        for app in listed:
            self.assertRegex(app, r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$")
        lowered = [app.lower() for app in listed]
        self.assertEqual(len(lowered), len(set(lowered)), "an app listed twice")

    def test_turns_go_round_the_list(self):
        listed = report.apps(APPS)
        self.assertEqual(report.pick(APPS, 0), listed[0])
        self.assertEqual(report.pick(APPS, 1), listed[1])
        self.assertEqual(report.pick(APPS, len(listed)), listed[0])

    def test_comments_and_blank_lines_are_ignored(self):
        with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as f:
            f.write("# heading\n\na/b   # why\n  c/d\n")
        try:
            self.assertEqual(report.apps(f.name), ["a/b", "c/d"])
        finally:
            os.remove(f.name)


class RunTest(unittest.TestCase):
    def setUp(self):
        self.work = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(self.work))
        self.meta(app="owner/app", app_sha="0123456789abcdef", hecatron_sha="fedcba9876543210",
                  macos="26.6", xcode="Xcode 27.1", run_url="https://github.com/dnf/hecatron/actions/runs/1")

    def meta(self, **facts):
        with open(os.path.join(self.work, "meta.tsv"), "a") as f:
            for key, value in facts.items():
                f.write(f"{key}\t{value}\n")

    def step(self, name, status, command="cmd"):
        with open(os.path.join(self.work, "steps.tsv"), "a") as f:
            f.write(f"{name}\t{status}\t3\t{command}\n")

    def build(self, status, jobs):
        with open(os.path.join(self.work, "build.json"), "w") as f:
            json.dump({"status": status, "jobs": jobs}, f)

    def outcome(self):
        return report.Run(self.work).outcome()

    def test_outcomes(self):
        self.step("clone", 0)
        self.step("init", 0)
        self.assertEqual(self.outcome()[:2], ("not-detected", True))
        self.meta(languages="swift objectivec")
        self.assertEqual(self.outcome()[:2], ("build-crashed", True))  # no JSON report
        self.build("passed", [{"label": "swift", "status": "passed"}])
        self.assertEqual(self.outcome()[:2], ("passed", False))
        self.build("skipped", [{"label": "swift", "status": "skipped", "reason": "the Pods aren't installed"}])
        kind, file, line = self.outcome()
        self.assertEqual((kind, file), ("skipped", False))
        self.assertIn("swift: the Pods aren't installed", line)
        self.build("failed", [{"label": "swift", "status": "failed"}])
        self.assertEqual(self.outcome()[:2], ("build-failed", True))

    def test_early_failures(self):
        self.step("clone", 128)
        self.assertEqual(self.outcome()[:2], ("clone-failed", True))
        self.meta(harness_error="hecatron's binary didn't build")
        self.assertEqual(self.outcome()[:2], ("harness-error", True))

    def test_the_issue_has_the_facts_errors_and_log(self):
        self.step("clone", 0)
        self.step("init", 0)
        self.step("deps", 1, "hct deps swift")
        self.step("build", 1, "hct build swift --output json")
        self.meta(languages="swift")
        self.build("failed", [{"label": "swift", "status": "failed", "log": ".logs/build/swift.log",
                               "errors": ["error: no such module 'Foo'"]},
                              {"label": "objectivec", "status": "skipped", "reason": "built with swift's hook"}])
        os.makedirs(os.path.join(self.work, "logs", "hct", "build"))
        with open(os.path.join(self.work, "logs", "hct", "build", "swift.log"), "w") as f:
            f.write("".join(f"line {i}\n" for i in range(200)))
        with open(os.path.join(self.work, "logs", "deps.log"), "w") as f:
            f.write("pod: command not found\n")
        with open(os.path.join(self.work, "plan.json"), "w") as f:
            json.dump({"kind": "-project", "scheme": "App"}, f)
        run = report.Run(self.work)
        self.assertEqual(report.title(run), "Real app: owner/app")
        body = report.body(run)
        for want in ("hct build failed on owner/app.", "[`0123456789ab`](https://github.com/owner/app/commit/0123456789abcdef)",
                     "| build | 1 | 3 | `hct build swift --output json` |", "### swift: failed",
                     "error: no such module 'Foo'", "line 199", "- objectivec: built with swift's hook",
                     "pod: command not found", '"scheme": "App"', "actions/runs/1"):
            self.assertIn(want, body)
        self.assertNotIn("line 119\n", body)  # only the log's last lines
        self.assertIn("line 120\n", body)

    def test_a_long_body_is_cut(self):
        self.meta(languages="swift")
        self.build("failed", [{"label": "swift", "status": "failed", "errors": ["x" * 100] * 1000}])
        body = report.body(report.Run(self.work))
        self.assertLess(len(body), 65536)
        self.assertTrue(body.endswith("(cut short: the full logs are in the run's artifact)"))

    def test_a_comment_on_an_open_issue_when_it_builds_again(self):
        self.meta(languages="swift")
        self.build("passed", [])
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertEqual(report.main(["comment", self.work]), 0)
        self.assertTrue(out.getvalue().startswith("owner/app built."))

    def test_the_script_is_listed_with_its_workflow(self):
        with open(os.path.join(HOME, ".github", "workflows", "real-apps.yml")) as f:
            workflow = f.read()
        self.assertIn("try-next.sh build", workflow)
        self.assertIn("try-next.sh report", workflow)
        # The build job runs the app's own scripts: no token that can write.
        build_job = re.search(r"\n  build:\n(.*?)\n  report:", workflow, re.S).group(1)
        self.assertNotIn("issues: write", build_job)
        self.assertNotIn("GH_TOKEN", build_job)
        self.assertIn("persist-credentials: false", build_job)


if __name__ == "__main__":
    unittest.main()
