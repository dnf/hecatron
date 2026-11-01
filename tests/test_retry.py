"""languages/_common.sh's retry: a download through a tool's own client
(go, rustup, uv...) tried again, as curl's are."""

import os
import subprocess
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
COMMON = os.path.join(HOME, "languages", "_common.sh")


def run(script):
    env = {**os.environ, "HECATRON_RETRY_DELAY": "0"}
    return subprocess.run(["bash", "-c", f'set -euo pipefail; source "{COMMON}" >/dev/null; {script}'],
                          env=env, capture_output=True, text=True)


class RetryTest(unittest.TestCase):
    def test_a_command_that_recovers_succeeds(self):
        result = run('n=0; flaky() { n=$((n + 1)); [ "${n}" -ge 3 ]; }; retry flaky; echo "tries ${n}"')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("tries 3", result.stdout)
        self.assertIn("flaky failed (exit 1, try 2 of 5)", result.stderr)

    def test_it_gives_up_after_five_tries_with_the_commands_status(self):
        result = run('n=0; broken() { n=$((n + 1)); return 7; }; retry broken || echo "status $? after ${n}"')
        self.assertIn("status 7 after 5", result.stdout)

    def test_variables_before_it_reach_the_command(self):
        result = run('GOTOOLCHAIN=go9.9 retry sh -c \'echo "${GOTOOLCHAIN}"\'')
        self.assertEqual(result.stdout.strip(), "go9.9")


if __name__ == "__main__":
    unittest.main()
