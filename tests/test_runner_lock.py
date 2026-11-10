"""run-parallel.sh's lock: one run of a command at a time. internal/runner's
TestLock checks the binary's, which takes the same .lock."""

from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
import time
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
RUNNER = os.path.join(HOME, "libexec", "runner", "run-parallel.sh")


class RunnerLockTest(unittest.TestCase):
    def setUp(self):
        self.logs = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, self.logs)
        self.lock = os.path.join(self.logs, "check", ".lock")
        self.env = {**os.environ, "HECATRON_LOGS": self.logs, "HECATRON_JOB_COMMAND": "sleep"}

    def start(self, job: str = "0") -> subprocess.Popen:
        return subprocess.Popen(["bash", RUNNER, "check", job], env=self.env, stdin=subprocess.DEVNULL,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)

    def run_check(self) -> tuple[int, str]:
        job = self.start()
        stderr = job.communicate()[1]
        return job.returncode, stderr

    def test_two_runs_started_together_one_runs(self):
        # mkdir as the lock let both through where it isn't atomic (uutils).
        jobs = [self.start("1"), self.start("1")]
        errors = [job.communicate()[1] for job in jobs]
        self.assertEqual(sorted(job.returncode for job in jobs), [0, 1], errors)
        self.assertIn("is already running", "".join(errors))
        self.assertFalse(os.path.lexists(self.lock))

    def test_a_held_lock_stops_the_run_and_a_stale_one_doesnt(self):
        os.makedirs(os.path.dirname(self.lock))
        for make in (lambda: open(self.lock, "w").close(), lambda: os.mkdir(self.lock)):
            # A file, and an earlier version's directory.
            make()
            status, stderr = self.run_check()
            self.assertEqual(status, 1)
            self.assertIn("check is already running", stderr)
            old = time.time() - 180
            os.utime(self.lock, (old, old))
            status, stderr = self.run_check()
            self.assertEqual(status, 0, stderr)
            self.assertFalse(os.path.lexists(self.lock))


if __name__ == "__main__":
    unittest.main()
