"""Swift's default test hook: swift test, unless the package declares no
test target. swift is a fake on PATH that prints how it was called."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

HOOK = os.path.join(HOME, "languages", "swift", "hooks", "test.sh")


class SwiftTestHookTest(Fixture):
    def run_hook(self) -> subprocess.CompletedProcess:
        fake = self.path(".fakebin")
        write(os.path.join(fake, "swift"), '#!/bin/sh\necho "swift $*"\n', 0o755)
        env = {**os.environ, "PATH": f"{fake}:{os.environ['PATH']}", "HECATRON_ROOT": self.root,
               "HECATRON_LANG": "swift"}
        return subprocess.run(["bash", HOOK], cwd=self.root, env=env, capture_output=True, text=True)

    def test_a_package_with_test_targets_runs_them(self):
        write(self.path("Package.swift"), 'let p = Package(targets: [.testTarget(name: "T")])\n')
        result = self.run_hook()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("swift test", result.stdout)

    def test_a_package_without_test_targets_is_skipped(self):
        write(self.path("Package.swift"), 'let p = Package(targets: [.target(name: "Lib")])\n')
        result = self.run_hook()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("the package declares no test targets", result.stdout)
        self.assertNotIn("swift test", result.stdout)


if __name__ == "__main__":
    unittest.main()
