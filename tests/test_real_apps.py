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

LISTS = {track: os.path.join(HOME, "tests", "real-apps", f"{track}.txt") for track in ("xcode", "android")}
APPS = LISTS["xcode"]


class ListTest(unittest.TestCase):
    def test_each_entry_is_an_owner_and_repository_once(self):
        for track, path in LISTS.items():
            listed = report.apps(path)
            self.assertGreater(len(listed), 30, track)
            for app in listed:
                self.assertRegex(app, r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$")
            lowered = [app.lower() for app in listed]
            self.assertEqual(len(lowered), len(set(lowered)), f"an app listed twice on {track}")

    def test_turns_go_round_the_list(self):
        listed = report.apps(APPS)
        self.assertEqual(report.pick(APPS, 0), listed[0])
        self.assertEqual(report.pick(APPS, 1), listed[1])
        self.assertEqual(report.pick(APPS, len(listed)), listed[0])

    def test_fixups(self):
        with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as f:
            f.write("# heading\n\nowner/app: cp a.template a   # its README\nother/app: true\n"
                    "Owner/App: ./bootstrap.sh --url https://x.example/#frag\nowner/app:\n"
                    "android owner/app: echo sdk.dir=x > local.properties\nxcode owner/app: pod install\n")
        try:
            self.assertEqual(report.fixups(f.name, "owner/app", "xcode"),
                             ["cp a.template a", "./bootstrap.sh --url https://x.example/#frag", "pod install"])
            self.assertEqual(report.fixups(f.name, "owner/app", "android"),
                             ["cp a.template a", "./bootstrap.sh --url https://x.example/#frag",
                              "echo sdk.dir=x > local.properties"])
            self.assertEqual(report.fixups(f.name, "none/here", "xcode"), [])
        finally:
            os.remove(f.name)
        with open(os.path.join(HOME, "tests", "real-apps", "fixups.txt")) as f:
            for line in f:
                words = report.COMMENT.sub("", line).partition(":")[0].split()
                if not words:
                    continue
                tracks = [words[0]] if len(words) == 2 else list(LISTS)
                listed = {app.lower() for track in tracks for app in report.apps(LISTS[track])}
                self.assertIn(words[-1].lower(), listed, f"a fixup for an app on no list: {line.strip()}")

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
        self.meta(app="owner/app", track="xcode", wanted="Swift or Objective-C", app_sha="0123456789abcdef",
                  hecatron_sha="fedcba9876543210", os="macOS 26.6", xcode="Xcode 27.1",
                  run_url="https://github.com/dnf/hecatron/actions/runs/1")

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
        # Nothing built is filed: a getting-started step for fixups.txt, most often.
        self.assertEqual((kind, file), ("nothing-built", True))
        self.assertIn("swift: the Pods aren't installed", line)
        self.build("failed", [{"label": "swift", "status": "failed"}])
        self.assertEqual(self.outcome()[:2], ("build-failed", True))

    def test_early_failures(self):
        self.step("clone", 0)
        self.step("setup", 1, "cp App.xcconfig.template App.xcconfig")
        self.assertEqual(self.outcome()[:2], ("setup-failed", True))
        os.remove(os.path.join(self.work, "steps.tsv"))
        self.step("clone", 128)
        self.assertEqual(self.outcome()[:2], ("clone-failed", True))
        os.remove(os.path.join(self.work, "steps.tsv"))
        self.step("clone", 0)
        self.step("init", 0)
        self.meta(languages="kotlin")
        self.step("install", 1, "hct install kotlin --no-deps")
        self.assertEqual(self.outcome()[:2], ("install-failed", True))
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
        self.assertEqual(report.title(run), "Real app (xcode): owner/app")
        body = report.body(run)
        for want in ("hct build failed on owner/app.", "[`0123456789ab`](https://github.com/owner/app/commit/0123456789abcdef)",
                     "| build | 1 | 3 | `hct build swift --output json` |", "### swift: failed",
                     "error: no such module 'Foo'", "line 199", "- objectivec: built with swift's hook",
                     "pod: command not found", '"scheme": "App"', "actions/runs/1"):
            self.assertIn(want, body)
        self.assertIn("real-app-retry", body)  # how to put it back in rotation
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
        self.assertNotIn("issues:", build_job)
        self.assertNotIn("GH_TOKEN", build_job)
        self.assertIn("persist-credentials: false", build_job)
        # Picking a retry reads issues; only the report job writes them.
        # Each track's schedule is the one TRACK maps to it, and each has a list.
        schedules = re.findall(r'- cron: "([^"]+)" # (\w+)', workflow)
        self.assertEqual({track for _, track in schedules}, set(LISTS))
        for cron, track in schedules:
            if track != "xcode":
                self.assertIn(f"github.event.schedule == '{cron}' && '{track}'", workflow)
        pick_job = re.search(r"\n  pick:\n(.*?)\n  # Runs", workflow, re.S).group(1)
        self.assertIn("issues: read", pick_job)
        self.assertNotIn("write", pick_job)


if __name__ == "__main__":
    unittest.main()
