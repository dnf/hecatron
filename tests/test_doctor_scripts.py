"""The built-in doctor.sh scripts, run as hecatron runs them: from the
project root, with HECATRON_HOME and HECATRON_ROOT, against fake tools."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402


class RubyDoctorTest(Fixture):
    def setUp(self):
        super().setUp()
        self.configure(languages=["ruby"])
        write(self.path("ruby", "Gemfile"), "source 'https://rubygems.org'\n")
        self.bin = self.path("bin")
        os.makedirs(self.bin)

    def doctor(self, path: str) -> subprocess.CompletedProcess:
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root, "PATH": path}
        return subprocess.run(["bash", os.path.join(HOME, "languages", "ruby", "doctor.sh")],
                              cwd=self.root, env=env, capture_output=True, text=True)

    def test_gems_not_installed(self):
        # `bundle check` fails until `hct deps ruby` installs the bundle.
        write(self.path("bin", "bundle"), "#!/bin/sh\nexit 1\n", 0o755)
        result = self.doctor(f"{self.bin}:{os.environ['PATH']}")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("ruby: the Gemfile's gems aren't installed (hecatron deps ruby installs them)", result.stdout)

    def test_bundler_not_installed(self):
        # The system's tools, but not Bundler (macOS has /usr/bin/bundle).
        for directory in ("/usr/bin", "/bin"):
            for name in os.listdir(directory):
                link = os.path.join(self.bin, name)
                if name != "bundle" and not os.path.exists(link):
                    os.symlink(os.path.join(directory, name), link)
        python = os.path.join(self.bin, "python3")
        if os.path.lexists(python):
            os.remove(python)
        os.symlink(sys.executable, python)
        result = self.doctor(self.bin)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Bundler isn't installed", result.stdout)
        self.assertNotIn("gems aren't installed", result.stdout)


class NodeInstallerTest(unittest.TestCase):
    def test_javascript_installs_node_through_typescript(self):
        # TypeScript's installer installs the Node.js versions JavaScript's
        # components resolve too (missing_versions javascript); JavaScript's
        # once installed only Homebrew's latest on macOS, and doctor went on
        # reporting the project's version missing.
        with open(os.path.join(HOME, "languages", "javascript", "install.sh"), encoding="utf-8") as f:
            javascript = f.read()
        with open(os.path.join(HOME, "languages", "typescript", "install.sh"), encoding="utf-8") as f:
            typescript = f.read()
        self.assertIn('bash "$(dirname "$0")/../typescript/install.sh"', javascript)
        self.assertNotIn("brew_install node", javascript)
        self.assertIn("missing_versions javascript", typescript)


if __name__ == "__main__":
    unittest.main()
