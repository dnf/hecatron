"""The skipped outcome through the scripts (bin/hecatron without the binary):
a hook's `skip`, each-component.sh's lines, run.sh's and run-parallel.sh's
counts, history and --fail-on-skip. cmd/hecatron/run_test.go checks that the
binary does the same."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

SOURCE = 'source "${HECATRON_HOME}/languages/_hooks.sh"\n'


class SkippedTest(Fixture):
    def setUp(self):
        super().setUp()
        self.configure(components=["a", "b"], languages=["bash"])
        for component in ("a", "b"):
            write(self.path(component, "bash", "x.sh"), "true\n")
        self.env = {**os.environ, "HECATRON_NO_GO": "1", "HECATRON_OUTPUT": "", "CLAUDECODE": "",
                    "HECATRON_FAIL_ON_SKIP": ""}

    def hook(self, action: str, body: str) -> None:
        write(self.path(".hecatron", "languages", "bash", action + ".sh"), "#!/usr/bin/env bash\n" + SOURCE + body, 0o755)

    def run_hct(self, *args: str, **env: str) -> subprocess.CompletedProcess:
        return subprocess.run(["bash", os.path.join(HOME, "bin", "hecatron"), *args], cwd=self.root,
                              env={**self.env, **env}, capture_output=True, text=True, stdin=subprocess.DEVNULL)

    def log(self, action: str) -> str:
        with open(self.path(".logs", action, "bash.log"), encoding="utf-8") as f:
            return f.read()

    def test_a_component_that_skips_is_named_and_the_rest_pass(self):
        self.hook("test", '[ "$1" = b ] && skip "no tests in b"\necho ran "$1"\n')
        result = self.run_hct("test", "bash")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("bash: test: no tests in b (skipping)", result.stdout)
        self.assertTrue(self.log("test").endswith(
            "bash: test skipped for 1 component(s): b (no tests in b)\nbash: test passed for 1 component(s)\n"),
            self.log("test"))
        self.assertEqual(self.run_hct("test", "bash", "--fail-on-skip").returncode, 0)

    def test_a_job_whose_components_all_skip_is_skipped(self):
        self.hook("test", 'skip "no tests"\n')
        result = self.run_hct("test", "bash")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(result.stdout.endswith("bash: test skipped for 2 component(s): a (no tests), b (no tests)\n"),
                        result.stdout)
        for strict in (self.run_hct("test", "bash", "--fail-on-skip"), self.run_hct("test", "bash", HECATRON_FAIL_ON_SKIP="1")):
            self.assertEqual(strict.returncode, 1)
            self.assertEqual(strict.stderr, "hecatron: nothing ran: every job skipped (--fail-on-skip)\n")

    def test_the_parallel_runner_counts_a_skipped_job_apart(self):
        self.hook("test", 'skip "no tests"\n')
        self.configure(components=["a", "b"], languages=["bash"], tasks={"test": {"check": "true"}})
        result = self.run_hct("test")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertRegex(result.stdout, r"\n  - bash +\d+\.\ds █+ skipped\n")
        self.assertIn("\n1/1 passed, 1 skipped (", result.stdout)
        self.assertIn("bash: test skipped for 2 component(s): a (no tests), b (no tests)\n", result.stdout)
        with open(self.path(".logs", "test", "history.log"), encoding="utf-8") as f:
            self.assertRegex(f.read(), r"\bbash=\d+:77\b")
        self.assertEqual(self.run_hct("test", "--fail-on-skip").returncode, 0)  # the task passed
        self.configure(components=["a", "b"], languages=["bash"])
        strict = self.run_hct("test", "--fail-on-skip")
        self.assertEqual(strict.returncode, 1, strict.stdout)
        self.assertIn("nothing ran", strict.stderr)

    def test_lint_skips_only_when_its_format_check_skips_too(self):
        self.hook("lint", 'skip "no linter"\n')
        self.hook("format", '[ "$1" = a ] && skip "no formatter"\necho checked\n')
        result = self.run_hct("lint", "bash")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue(result.stdout.endswith(
            "bash: lint skipped for 1 component(s): a (no linter)\nbash: lint passed for 1 component(s)\n"), result.stdout)

    def test_run_hook_in_skips_only_when_each_directory_skips(self):
        for component in ("a", "b"):
            write(self.path(component, "bash", "one", "build.marker"), "")
            write(self.path(component, "bash", "two", "build.marker"), "")
        write(self.path("b", "bash", "two", "tests"), "")
        self.hook("test", '[ -f build.marker ] || run_below_with build.marker\n'
                          '[ -f tests ] || skip "no tests here"\necho tested\n')
        result = self.run_hct("test", "bash")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("bash: test skipped for 1 component(s): a (one: no tests here; two: no tests here)\n"
                      "bash: test passed for 1 component(s)\n", result.stdout)

    def test_a_hook_that_writes_a_reason_and_fails_has_failed(self):
        self.hook("test", 'echo "nothing" >"${HECATRON_SKIPPED}"\nexit 3\n')
        result = self.run_hct("test", "bash")
        self.assertEqual(result.returncode, 1)
        self.assertIn("bash: test failed: a b", result.stdout)

    def test_a_language_without_a_hook_is_skipped(self):
        result = self.run_hct("deps", "bash")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "bash: deps skipped: no dependency manager for bash\n")
        self.assertEqual(self.run_hct("deps", "bash", "--fail-on-skip").returncode, 1)

    def test_skip_run_by_hand_just_succeeds(self):
        self.hook("test", 'skip "no tests"\n')
        result = subprocess.run(["bash", self.path(".hecatron", "languages", "bash", "test.sh")], cwd=self.path("a", "bash"),
                                env={**self.env, "HECATRON_HOME": HOME, "HECATRON_LANG": "bash"},
                                capture_output=True, text=True)
        self.assertEqual((result.returncode, result.stdout), (0, "bash: test: no tests (skipping)\n"))


if __name__ == "__main__":
    unittest.main()
