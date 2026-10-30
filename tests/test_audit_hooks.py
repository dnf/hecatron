"""The OSV-Scanner audit hooks (audit_osv in languages/_audit.sh): each
language scans only its own ecosystem's lock files, not every one below
it, and a component with none is skipped, not passed. osv-scanner is a
fake on PATH that records how it was called."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

FAKE_OSV = """#!/bin/sh
printf '%s\\n' "$@" >"${OSV_ARGS}"
printf '{"results": []}\\n'
exit "${OSV_STATUS:-0}"
"""


class OSVAuditHookTest(Fixture):
    def setUp(self) -> None:
        super().setUp()
        self.fake = self.path(".fakebin")
        write(os.path.join(self.fake, "osv-scanner"), FAKE_OSV, 0o755)
        self.args_file = self.path(".osv-args")
        self.skip_file = self.path(".skipped")

    def run_hook(self, lang: str, cwd: str | None = None, status: int = 0) -> subprocess.CompletedProcess:
        env = {**os.environ, "PATH": f"{self.fake}:{os.environ['PATH']}", "HECATRON_ROOT": self.root,
               "HECATRON_LANG": lang, "HECATRON_COMPONENT": ".", "OSV_ARGS": self.args_file,
               "OSV_STATUS": str(status), "HECATRON_SKIPPED": self.skip_file}
        env.pop("HECATRON_BINARY", None)
        hook = os.path.join(HOME, "languages", lang, "hooks", "audit.sh")
        return subprocess.run(["bash", hook], cwd=cwd or self.root, env=env, capture_output=True, text=True,
                              stdin=subprocess.DEVNULL)

    def scanned(self) -> list[str]:
        with open(self.args_file, encoding="utf-8") as f:
            args = f.read().split("\n")
        return [args[i + 1] for i, arg in enumerate(args) if arg == "--lockfile"]

    def skipped(self) -> str:
        if not os.path.exists(self.skip_file):
            return ""
        with open(self.skip_file, encoding="utf-8") as f:
            return f.read().strip()

    def ios_sdk(self) -> None:
        """An iOS SDK: Package.swift, a Podfile, a root package-lock.json
        for its commit tooling, and an example app with its own
        Package.resolved; SwiftPM's checkouts in .build have theirs."""
        self.configure(languages=["swift"])
        write(self.path("Package.swift"), "// swift-tools-version:5.9\n")
        write(self.path("Package.resolved"), '{"pins": [], "version": 2}\n')
        write(self.path("Podfile"), "pod 'AFNetworking'\n")
        write(self.path("Podfile.lock"), "PODS:\n  - AFNetworking (2.5.1)\n")
        write(self.path("package-lock.json"), '{"lockfileVersion": 3, "packages": {}}\n')
        write(self.path("Example", "Package.resolved"), '{"pins": [], "version": 2}\n')
        write(self.path(".build", "checkouts", "dep", "Package.resolved"), '{"pins": [], "version": 2}\n')
        write(self.path("node_modules", "x", "package-lock.json"), "{}\n")

    def test_swift_scans_only_swiftpm_lock_files(self):
        self.ios_sdk()
        result = self.run_hook("swift")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.scanned(), ["./Example/Package.resolved", "./Package.resolved"])
        with open(self.args_file, encoding="utf-8") as f:
            self.assertNotIn("--recursive", f.read())
        self.assertEqual(self.skipped(), "")

    def test_a_nested_component_of_the_language_is_left_to_its_own_audit(self):
        self.ios_sdk()
        self.configure(languages=["swift"], components=[".", "Example"], implPath="{component}")
        self.run_hook("swift")
        self.assertEqual(self.scanned(), ["./Package.resolved"])
        self.run_hook("swift", cwd=self.path("Example"))
        self.assertEqual(self.scanned(), ["./Package.resolved"])

    def test_each_ecosystem_reads_its_own_files(self):
        self.configure(languages=["rust", "java"])
        write(self.path("Cargo.lock"), "")
        write(self.path("gradle.lockfile"), "")
        write(self.path("app", "pom.xml"), "<project/>\n")
        write(self.path("package-lock.json"), "{}\n")
        write(self.path("target", "pom.xml"), "<project/>\n")
        self.run_hook("rust")
        self.assertEqual(self.scanned(), ["./Cargo.lock"])
        self.run_hook("java")
        self.assertEqual(self.scanned(), ["./app/pom.xml", "./gradle.lockfile"])

    def test_gradle_without_a_lock_file_is_skipped_with_how_to_write_one(self):
        self.configure(languages=["kotlin"])
        write(self.path("build.gradle.kts"), 'plugins { kotlin("jvm") }\n')
        write(self.path("Gemfile.lock"), "GEM\n")
        result = self.run_hook("kotlin")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(os.path.exists(self.args_file), "osv-scanner ran with nothing to scan")
        self.assertIn("kotlin: audit: nothing to audit: no pom.xml, gradle.lockfile", result.stdout)
        self.assertIn("gradle dependencies --write-locks", self.skipped())
        self.assertTrue(self.skipped().startswith("nothing to audit: "), self.skipped())

    def test_lock_files_without_packages_are_skipped(self):
        self.configure(languages=["rust"])
        write(self.path("Cargo.lock"), "")
        result = self.run_hook("rust", status=128)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("no packages in ./Cargo.lock", self.skipped())

    def test_findings_pass_through_the_reader(self):
        self.configure(languages=["rust"])
        write(self.path("Cargo.lock"), "")
        report = os.path.join(HOME, "tests", "output", "osv-scanner", "2.7.0", "audit-osv-rust-cargo.json")
        with open(report, encoding="utf-8") as f:
            text = f.read()
        write(os.path.join(self.fake, "osv-scanner"),
              f"#!/bin/sh\nprintf '%s\\n' \"$@\" >\"${{OSV_ARGS}}\"\ncat <<'EOF'\n{text}\nEOF\nexit 1\n", 0o755)
        result = self.run_hook("rust")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)  # moderate, below high
        self.assertIn("time 0.1.43  GHSA-wcg3-cvx6-7396", result.stdout)
        self.assertEqual(self.skipped(), "")


class EachComponentSkipTest(Fixture):
    """each-component.sh counts a skipped component apart from those that
    passed, as the binary's runner does."""

    def test_a_skipped_component_isnt_counted_as_passed(self):
        self.configure(languages=["kotlin"], implPath=".")
        write(self.path("build.gradle.kts"), "\n")
        fake = self.path(".fakebin")
        write(os.path.join(fake, "osv-scanner"), "#!/bin/sh\nexit 0\n", 0o755)
        env = {**os.environ, "PATH": f"{fake}:{os.environ['PATH']}", "HECATRON_HOME": HOME,
               "HECATRON_NO_GO": "1"}
        env.pop("HECATRON_BINARY", None)
        result = subprocess.run(["bash", os.path.join(HOME, "libexec", "lib", "each-component.sh"), "audit", "kotlin"],
                                cwd=self.root, env=env, capture_output=True, text=True, stdin=subprocess.DEVNULL)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("kotlin: audit skipped for 1 component(s): .", result.stdout)
        self.assertNotIn("passed for", result.stdout)


if __name__ == "__main__":
    unittest.main()
