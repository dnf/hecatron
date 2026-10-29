"""Which of builds.hex.pm's OTP builds Erlang's and Elixir's installers pick
(otp_build_matching in languages/_common.sh): a major's newest final
release, or exactly the version pinned, never a release candidate or a
build of another major that shares its digits."""

from __future__ import annotations

import os
import subprocess
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
COMMON = os.path.join(HOME, "languages", "_common.sh")

# builds.txt's columns: name, git commit, build date, SHA-256.
BUILDS = """\
OTP-27.3.4 c1 2025-05-01 sha-27.3.4
OTP-27.3.4.9 c2 2026-02-01 sha-27.3.4.9
OTP-28.0-rc1 c3 2025-02-01 sha-28.0-rc1
OTP-28.0 c4 2025-05-21 sha-28.0
OTP-28.4.1 c5 2026-04-01 sha-28.4.1
OTP-29.0-rc3 c6 2026-04-20 sha-29.0-rc3
OTP-29.1.2 c7 2026-10-01 sha-29.1.2
OTP-29.1 c8 2026-09-01 sha-29.1
maint-28 c9 2026-10-01 sha-maint
"""


class OtpBuildMatchingTest(unittest.TestCase):
    def match(self, spec: str) -> str:
        with tempfile.TemporaryDirectory() as home:
            result = subprocess.run(["bash", "-c", 'source "$1"; otp_build_matching "$2"', "_", COMMON, spec],
                                    input=BUILDS, capture_output=True, text=True, timeout=60,
                                    env={**os.environ, "HOME": home, "HECATRON_ROOT": home})
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def test_a_major_takes_its_newest_final_release(self):
        self.assertEqual(self.match("27"), "27.3.4.9 sha-27.3.4.9")
        self.assertEqual(self.match("28"), "28.4.1 sha-28.4.1")
        self.assertEqual(self.match("29"), "29.1.2 sha-29.1.2")

    def test_an_exact_version_takes_only_that_build(self):
        self.assertEqual(self.match("=29.1"), "29.1 sha-29.1")
        self.assertEqual(self.match("=29.1.2"), "29.1.2 sha-29.1.2")
        self.assertEqual(self.match("=29.0"), "")

    def test_no_build_of_another_major_or_branch(self):
        self.assertEqual(self.match("2"), "")
        self.assertEqual(self.match("30"), "")


if __name__ == "__main__":
    unittest.main()
