"""hecatron outdated's report reading (libexec/lib/outdated.py): each tool's
report as captured from the fixtures (tests/outdated/reports/, from
tests/deps/fixtures/ in a container), the tools' other shapes, --fail, and
--json across languages."""

from __future__ import annotations

import contextlib
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))
sys.path.insert(0, os.path.join(HOME, "tests"))

import outdated  # noqa: E402
from test_hecatron import Fixture, write  # noqa: E402

REPORTS = os.path.join(HOME, "tests", "outdated", "reports")


def captured(fmt: str) -> list[tuple[str, str, str, str]]:
    with open(os.path.join(REPORTS, f"{fmt}.txt"), encoding="utf-8") as f:
        return [(r.package, r.current, r.wanted, r.latest) for r in outdated.parse(fmt, f.read())]


class CapturedReportsTest(unittest.TestCase):
    """The fixtures pin one old dependency each; every report names it."""

    def assert_one(self, fmt: str, package: str, current: str, wanted: str = ""):
        rows = captured(fmt)
        self.assertEqual(len(rows), 1, rows)
        self.assertEqual(rows[0][:3], (package, current, wanted))
        self.assertTrue(rows[0][3] and rows[0][3] != current)

    def test_npm(self):
        self.assert_one("npm", "left-pad", "1.2.0", "1.2.0")

    def test_uv_tree(self):
        self.assert_one("uv-tree", "six", "1.15.0")

    def test_go_lists_only_direct_requirements(self):
        self.assert_one("go", "github.com/google/uuid", "1.3.0")

    def test_cargo_without_a_plugin(self):
        self.assert_one("cargo", "itoa", "1.0.1")

    def test_bundler(self):
        self.assert_one("bundler", "rack", "2.2.3")

    def test_composer_from_the_lock(self):
        self.assert_one("composer", "psr/log", "1.1.3")

    def test_dotnet(self):
        self.assert_one("dotnet", "Newtonsoft.Json", "12.0.3")

    def test_mix(self):
        self.assert_one("mix", "jason", "1.2.2")

    def test_dart(self):
        self.assert_one("dart", "path", "1.8.0", "1.8.0")

    def test_maven(self):
        self.assert_one("maven", "org.apache.commons:commons-lang3", "3.12.0")


class ReportShapesTest(unittest.TestCase):
    def rows(self, fmt: str, text: str, direct: str | None = None):
        return [(r.package, r.current, r.wanted, r.latest) for r in outdated.parse(fmt, text, direct)]

    def test_nothing_outdated_is_an_empty_report(self):
        for fmt in ("npm", "pnpm", "composer", "dotnet", "dart", "gradle"):
            self.assertEqual(self.rows(fmt, ""), [], fmt)

    def test_pnpm(self):
        text = json.dumps({"left-pad": {"current": "1.2.0", "latest": "1.3.0", "wanted": "1.2.0"}})
        self.assertEqual(self.rows("pnpm", text), [("left-pad", "1.2.0", "1.2.0", "1.3.0")])

    def test_yarn_classic(self):
        text = "\n".join([
            json.dumps({"type": "info", "data": "Color legend"}),
            json.dumps({"type": "table", "data": {
                "head": ["Package", "Current", "Wanted", "Latest", "Package Type", "URL"],
                "body": [["left-pad", "1.2.0", "1.2.0", "1.3.0", "dependencies", "https://x"]]}})])
        self.assertEqual(self.rows("yarn-classic", text), [("left-pad", "1.2.0", "1.2.0", "1.3.0")])

    def test_uv_pip(self):
        text = json.dumps([{"name": "six", "version": "1.15.0", "latest_version": "1.17.0"}])
        self.assertEqual(self.rows("uv-pip", text), [("six", "1.15.0", "", "1.17.0")])

    def test_uv_tree_with_extras_and_groups(self):
        text = "fixture v0.1.0\n├── six v1.15.0 (latest: v1.17.0)\n└── pytest v7.0.0 (group: dev) (latest: v8.3.0)\n"
        self.assertEqual(self.rows("uv-tree", text),
                         [("pytest", "7.0.0", "", "8.3.0"), ("six", "1.15.0", "", "1.17.0")])

    def test_cargo_counts_direct_dependencies_and_a_lock_behind_its_requirements(self):
        text = ("   Unchanged itoa v1.0.1 (available: v1.0.18)\n"
                "    Updating serde v1.0.100 -> v1.0.200\n"
                "   Unchanged memchr v2.0.0 (available: v2.7.0)\n")
        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
            json.dump({"packages": [{"dependencies": [{"name": "itoa"}, {"name": "serde"}]}]}, f)
        try:
            self.assertEqual(self.rows("cargo", text, f.name),
                             [("itoa", "1.0.1", "", "1.0.18"), ("serde", "1.0.100", "1.0.200", "1.0.200")])
        finally:
            os.unlink(f.name)

    def test_mix_says_when_the_requirement_allows_the_latest(self):
        text = ("Dependency  Only  Current  Latest  Status\n"
                "ex_doc      dev   0.30.0   0.34.0  Update possible\n"
                "jason             1.4.0    1.4.0   Up-to-date\n")
        self.assertEqual(self.rows("mix", text), [("ex_doc", "0.30.0", "0.34.0", "0.34.0")])

    def test_maven_puts_a_long_name_and_its_versions_on_two_lines(self):
        text = ("The following dependencies in Dependencies have newer versions:\n"
                "  com.example.something.very.long:artifact-with-a-long-name ...\n"
                "                                                        1.0 -> 2.0\n"
                "  junit:junit ............................................ 4.12 -> 4.13.2\n")
        self.assertEqual(self.rows("maven", text), [
            ("com.example.something.very.long:artifact-with-a-long-name", "1.0", "", "2.0"),
            ("junit:junit", "4.12", "", "4.13.2")])

    def test_gradle_versions_plugin(self):
        text = json.dumps({"outdated": {"dependencies": [
            {"group": "org.slf4j", "name": "slf4j-api", "version": "1.7.36", "available": {"release": "2.0.16"}}]}})
        self.assertEqual(self.rows("gradle", text), [("org.slf4j:slf4j-api", "1.7.36", "", "2.0.16")])

    def test_cabal(self):
        text = "Outdated dependencies:\nbase >=4 && <4.15 (latest: 4.20.0.0)\ntext ==1.2.* (latest: 2.1.2)\n"
        self.assertEqual(self.rows("cabal", text), [("base", ">=4 && <4.15", "", "4.20.0.0"),
                                                    ("text", "==1.2.*", "", "2.1.2")])

    def test_a_report_that_isnt_one_is_an_error(self):
        with self.assertRaises(ValueError):
            outdated.parse("npm", "npm ERR! something")


class ReportTest(unittest.TestCase):
    TEXT = json.dumps({"left-pad": {"current": "1.2.0", "wanted": "1.2.0", "latest": "1.3.0"}})

    def run_report(self, text: str, **kwargs):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            status = outdated.report("npm", text, "typescript", "comp", **kwargs)
        return status, out.getvalue()

    def test_reports_and_passes_by_default(self):
        status, out = self.run_report(self.TEXT)
        self.assertEqual(status, 0)
        self.assertIn("left-pad", out)
        self.assertIn("comp: typescript 1 outdated dependency", out)

    def test_fail_fails_only_when_something_is_outdated(self):
        self.assertEqual(self.run_report(self.TEXT, fail=True)[0], 1)
        self.assertEqual(self.run_report("", fail=True)[0], 0)

    def test_saves_rows_for_json(self):
        with tempfile.TemporaryDirectory() as directory:
            self.run_report(self.TEXT, json_dir=directory)
            self.assertEqual(outdated.collect(directory), [{
                "component": "comp", "language": "typescript", "package": "left-pad",
                "current": "1.2.0", "wanted": "1.2.0", "latest": "1.3.0"}])


class RunTest(unittest.TestCase):
    """outdated_run in languages/_outdated.sh: a tool's status and its report."""

    def run_tool(self, tool: str) -> subprocess.CompletedProcess:
        hooks = os.path.join(HOME, "languages", "_hooks.sh")
        helpers = os.path.join(HOME, "languages", "_outdated.sh")
        script = f"source {hooks}\nsource {helpers}\noutdated_run bash -c '{tool}'\necho ran"
        env = {**os.environ, "HECATRON_LANG": "fixture"}
        return subprocess.run(["bash", "-c", script], env=env, capture_output=True, text=True)

    def test_a_tool_failing_with_a_report_carries_on(self):
        # npm outdated, Bundler and Hex exit 1 when something is outdated.
        result = self.run_tool('echo "{}"; exit 1')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ran", result.stdout)

    def test_a_tool_failing_without_a_report_is_an_error(self):
        # uv tree, its pinned interpreter gone, says so on stderr only.
        result = self.run_tool("echo no interpreter >&2; exit 2")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("ran", result.stdout)
        self.assertIn("failed (exit 2) without a report", result.stderr)


class JsonCommandTest(Fixture):
    """--json runs each language and prints one array, logs on stderr."""

    def setUp(self):
        super().setUp()
        self.configure(languages=["fixture"])
        os.makedirs(self.path("fixture"))
        write(self.path(".hecatron", "languages", "fixture", "config.json"), json.dumps({"dependency": "bash"}))
        report = json.dumps({"left-pad": {"current": "1.2.0", "latest": "1.3.0"}})
        lib = os.path.join(HOME, "libexec", "lib", "outdated.py")
        write(self.path(".hecatron", "languages", "fixture", "outdated.sh"),
              f"#!/usr/bin/env bash\necho '{report}' >report.json\n"
              f'python3 {lib} report npm report.json "$HECATRON_LANG" "$HECATRON_COMPONENT"\n', 0o755)

    def test_json_prints_one_array(self):
        result = self.hecatron("outdated", "--json", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        rows = json.loads(result.stdout)
        self.assertEqual([(r["language"], r["package"], r["latest"]) for r in rows], [("fixture", "left-pad", "1.3.0")])
        self.assertIn("=== outdated", result.stderr)

    def test_json_without_languages_runs_every_one(self):
        result = self.hecatron("outdated", "--json")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(json.loads(result.stdout)), 1)

    def test_fail(self):
        self.assertEqual(self.hecatron("outdated", "fixture").returncode, 0)
        self.assertNotEqual(self.hecatron("outdated", "--fail", "fixture").returncode, 0)


if __name__ == "__main__":
    unittest.main()
