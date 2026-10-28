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
        for tool in ("cmake", "ctest", "make", "meson"):
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

    def test_tests_set_up_in_a_subdirectory_run_from_there(self):
        # enable_testing() in sub/, not at the top (Zstandard's build/cmake/):
        # CTest finds the tests only from that subdirectory's build tree.
        write(self.path("CMakeLists.txt"), "project(x)\nadd_subdirectory(sub)\n")
        write(self.path("sub", "CMakeLists.txt"), "enable_testing()\nadd_test(NAME t COMMAND t)\n")
        write(self.path("build", "CMakeCache.txt"), "")
        for below in ("sub", "sub/inner", "_deps/dep"):
            write(self.path("build", *below.split("/"), "CTestTestfile.cmake"), "")
        # ctest reports tests anywhere below the top.
        fake = FAKE.replace('echo "Total Tests: ${FAKE_TESTS:-0}"',
                            'case " $* " in *" build "*) echo "Total Tests: 0" ;; *) echo "Total Tests: 2" ;; esac')
        write(os.path.join(self.bin, "ctest"), fake, 0o755)
        result = self.run_hook("test")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("ctest --test-dir build/sub --output-on-failure", result.stdout)
        self.assertNotIn("inner", result.stdout)
        self.assertNotIn("_deps", result.stdout)
        self.assertNotIn("--no-tests=error", result.stdout)

    def test_a_makefile_builds_before_its_check(self):
        write(self.path("Makefile"), "all:\ncheck:\n")
        result = self.run_hook("test")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        lines = result.stdout.splitlines()
        self.assertLess(lines.index("+ make"), lines.index("+ make check"), result.stdout)

    def test_meson_starts_over_after_a_failed_setup(self):
        write(self.path("meson.build"), "project('x', 'c')\n")
        write(self.path("build", "meson-private", "x"), "")
        result = self.run_hook("test")
        self.assertIn("+ rm -rf build\n+ meson setup build", result.stdout)
        write(self.path("build", "build.ninja"), "")
        result = self.run_hook("test")
        self.assertNotIn("meson setup", result.stdout)
        self.assertIn("meson test -C build", result.stdout)


if __name__ == "__main__":
    unittest.main()
