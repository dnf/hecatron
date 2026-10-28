"""Rust's installer: the system packages a component's crates build with
(cargo_dependencies in languages/_common.sh), from the Cargo.lock files it
reads. The package manager is faked: functions that print what they'd do."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

COMMON = os.path.join(HOME, "languages", "_common.sh")
FAKES = 'OS=linux\napt_has() { true; }\napt_install() { echo "apt_install $*"; }\ndpkg() { return 1; }\n'


def lock(*crates: str) -> str:
    return "version = 4\n\n" + "".join(f'[[package]]\nname = "{c}"\nversion = "1.0.0"\n\n' for c in crates)


class CargoPackagesTest(Fixture):
    def install(self, impl_path: str = "rust") -> str:
        self.configure(languages=["rust"], implPath=impl_path)
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root}
        result = subprocess.run(["bash", "-c", f"source {COMMON}\n{FAKES}cargo_dependencies rust"],
                                cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return "".join(line for line in result.stdout.splitlines() if line.startswith("apt_install"))

    def test_sys_crates_and_build_tools_install_their_packages(self):
        write(self.path("rust", "Cargo.toml"), "[package]\nname = \"x\"\n")
        write(self.path("rust", "Cargo.lock"), lock("x", "openssl-sys", "cmake", "pyo3-build-config", "serde"))
        self.assertEqual(self.install(), "apt_install cmake libssl-dev pkg-config python3-dev")

    def test_the_workspace_lock_above_counts(self):
        write(self.path("Cargo.lock"), lock("prost-build"))
        write(self.path("crates", "rust", "Cargo.toml"), "[package]\nname = \"x\"\n")
        self.assertEqual(self.install("crates/{lang}"), "apt_install protobuf-compiler")

    def test_locks_below_count(self):
        write(self.path("rust", "a", "Cargo.lock"), lock("capnpc"))
        write(self.path("rust", "b", "Cargo.lock"), lock("glib-sys"))
        self.assertEqual(self.install(), "apt_install capnproto libglib2.0-dev pkg-config")

    def test_a_vendored_protoc_needs_none(self):
        write(self.path("rust", "Cargo.lock"), lock("prost-build", "protoc-bin-vendored"))
        self.assertEqual(self.install(), "")

    def test_nothing_to_install(self):
        write(self.path("rust", "Cargo.lock"), lock("serde", "tokio"))
        self.assertEqual(self.install(), "")


if __name__ == "__main__":
    unittest.main()
