"""The native installers' Meson (meson_missing, meson_install and
meson_required in languages/_common.sh): a Meson older than a meson.build's
project(meson_version: '>=X') counts as missing, and is then installed from
PyPI with uv rather than from apt. Package managers are faked."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

COMMON = os.path.join(HOME, "languages", "_common.sh")
FAKES = ('OS=linux\napt_install() { echo "apt_install $*"; }\nuv_install() { echo uv_install; }\n'
         'uv() { echo "uv $*"; }\n')


class MesonVersionTest(Fixture):
    def shell(self, script: str, meson: str | None = None) -> str:
        self.configure(languages=["c"], implPath=".")
        path = os.environ["PATH"]
        if meson:
            fake = self.path(".fakebin", "meson")
            write(fake, f"#!/bin/sh\necho {meson}\n", 0o755)
            path = f"{os.path.dirname(fake)}:{path}"
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root, "PATH": path}
        result = subprocess.run(["bash", "-c", f"source {COMMON}\n{FAKES}{script}"],
                                cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout.strip()

    def test_the_newest_requirement_counts(self):
        write(self.path("meson.build"), "project('x', 'c',\n  meson_version: '>= 1.8.3',\n)\n")
        write(self.path("sub", "meson.build"), "project('y', meson_version : \">=1.10\")\n")
        write(self.path("other", "meson.build"), "project('z')\n")
        self.assertEqual(self.shell("meson_required c"), "1.10")

    def test_an_older_meson_is_missing_and_comes_from_pypi(self):
        write(self.path("meson.build"), "project('x', 'c', meson_version: '>=1.8.3')\n")
        self.assertEqual(self.shell("meson_missing c && echo missing", meson="1.3.2"), "missing")
        self.assertEqual(self.shell("meson_missing c || echo fine", meson="1.10.0"), "fine")
        self.assertEqual(self.shell("meson_install c", meson="1.3.2").splitlines()[1:],
                         ["apt_install ninja-build", "uv_install", "uv tool install --force meson"])

    def test_without_a_requirement_apt_meson_will_do(self):
        write(self.path("meson.build"), "project('x', 'c')\n")
        self.assertEqual(self.shell("meson_missing c || echo fine", meson="0.60.0"), "fine")
        self.assertEqual(self.shell("meson_install c", meson="0.60.0").splitlines()[1:],
                         ["apt_install meson ninja-build"])

    def test_version_order(self):
        self.assertEqual(self.shell("version_older 1.3.2 1.10 && echo older; version_older 1.10.0 1.8.3 || echo newer;"
                                    " version_older 1.8.3 1.8.3 || echo same"), "older\nnewer\nsame")


if __name__ == "__main__":
    unittest.main()
