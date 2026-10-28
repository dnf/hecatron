"""The native build hooks (languages/_hooks.sh's native_hook): where CMake
builds, and a CMake project that registers no tests. cmake and ctest are
fakes on PATH that print how they were called; the real tools run in
containers (tests/hooks/check.sh)."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

HOOKS = os.path.join(HOME, "languages", "_hooks.sh")

# ctest -N reports FAKE_TESTS tests; everything else is printed.
FAKE = """#!/bin/sh
echo "$(basename "$0") $*"
case " $* " in *" -N "*) echo "Total Tests: ${FAKE_TESTS:-0}" ;; esac
"""


class NativeHookTest(Fixture):
    def setUp(self):
        super().setUp()
        self.bin = self.path(".fakebin")
        for tool in ("cmake", "ctest", "make"):
            write(os.path.join(self.bin, tool), FAKE, 0o755)

    def run_hook(self, action: str, **env) -> subprocess.CompletedProcess:
        # The hook's action is its script's name, as each-component.sh runs it.
        script = self.path(".hook", f"{action}.sh")
        write(script, f"source {HOOKS}\nnative_hook {action}\n")
        environ = {**os.environ, "PATH": f"{self.bin}:{os.environ['PATH']}", "HECATRON_ROOT": self.root,
                   "HECATRON_LANG": "c", **env}
        return subprocess.run(["bash", script], cwd=self.root, env=environ, capture_output=True, text=True)

    def test_builds_in_build(self):
        write(self.path("CMakeLists.txt"), "project(x)\nenable_testing()\nadd_test(NAME t COMMAND t)\n")
        result = self.run_hook("test", FAKE_TESTS="1")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn(f"cmake -S {self.root} -B build", result.stdout)
        self.assertIn("ctest --test-dir build --output-on-failure", result.stdout)

    def test_a_build_directory_the_project_keeps_is_left_alone(self):
        write(self.path("CMakeLists.txt"), "project(x)\nadd_subdirectory(build/cmake)\n")
        write(self.path("build", "cmake", "CMakeLists.txt"), "enable_testing()\nadd_test(NAME t COMMAND t)\n")
        self.commit()
        result = self.run_hook("test", FAKE_TESTS="1")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn(f"cmake -S {self.root} -B _build", result.stdout)
        self.assertIn("ctest --test-dir _build --output-on-failure", result.stdout)
        result = self.run_hook("clean")
        self.assertIn("rm -rf _build", result.stdout)
        self.assertTrue(os.path.isfile(self.path("build", "cmake", "CMakeLists.txt")))

    def test_an_untracked_build_directory_is_build_output(self):
        write(self.path("CMakeLists.txt"), "project(x)\n")
        self.commit()
        write(self.path("build", "CMakeCache.txt"), "")
        result = self.run_hook("build")
        self.assertIn(f"cmake -S {self.root} -B build", result.stdout)

    def test_a_project_without_tests_is_skipped(self):
        write(self.path("CMakeLists.txt"), "project(x)\nadd_library(x x.c)\n")
        result = self.run_hook("test")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("the CMake project defines no tests", result.stdout)
        self.assertNotIn("--no-tests=error", result.stdout)

    def test_a_project_whose_tests_were_left_out_fails(self):
        write(self.path("CMakeLists.txt"), "project(x)\nif(X_TESTS)\n  add_subdirectory(tests)\nendif()\n")
        write(self.path("tests", "CMakeLists.txt"), "add_test(NAME t COMMAND t)\n")
        result = self.run_hook("test")
        self.assertIn("ctest --test-dir build --output-on-failure --no-tests=error", result.stdout)


if __name__ == "__main__":
    unittest.main()
