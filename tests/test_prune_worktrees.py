"""libexec/maintenance/prune-worktrees.sh: removes the worktrees whose work
is in main, and keeps the rest, saying why."""

import os
import subprocess
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCRIPT = os.path.join(HOME, "libexec", "maintenance", "prune-worktrees.sh")
GIT_ENV = {**os.environ, "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
           "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
           "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_SYSTEM": os.devnull}


def git(cwd, *args):
    return subprocess.run(["git", *args], cwd=cwd, env=GIT_ENV, capture_output=True, text=True, check=True).stdout


class PruneWorktreesTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        top = os.path.realpath(self.dir.name)  # git reports macOS's /private/var
        # A gh that knows no pull requests: these branches have none.
        stub = os.path.join(top, "bin")
        os.mkdir(stub)
        with open(os.path.join(stub, "gh"), "w") as f:
            f.write("#!/bin/sh\nexit 1\n")
        os.chmod(os.path.join(stub, "gh"), 0o755)
        self.env = {**GIT_ENV, "PATH": stub + os.pathsep + os.environ["PATH"]}
        origin, self.repo = os.path.join(top, "origin.git"), os.path.join(top, "repo")
        git(top, "init", "-q", "--bare", "-b", "main", origin)
        git(top, "clone", "-q", origin, self.repo)
        git(self.repo, "commit", "-q", "--allow-empty", "-m", "first")
        git(self.repo, "push", "-q", "origin", "main")
        self.paths = {name: os.path.join(top, name) for name in ("merged", "changed", "unmerged", "locked")}
        for name, path in self.paths.items():
            git(self.repo, "worktree", "add", "-q", "-b", name, path)
        with open(os.path.join(self.paths["changed"], "work.txt"), "w") as f:
            f.write("not committed\n")
        git(self.paths["unmerged"], "commit", "-q", "--allow-empty", "-m", "only here")
        git(self.repo, "worktree", "lock", self.paths["locked"])

    def tearDown(self):
        self.dir.cleanup()

    def run_script(self, *args):
        return subprocess.run(["bash", SCRIPT, *args], cwd=self.repo, env=self.env, capture_output=True, text=True,
                              check=True).stdout

    def test_dry_run_removes_nothing(self):
        out = self.run_script("--dry-run")
        self.assertIn("remove  %s (merged)" % self.paths["merged"], out)
        self.assertIn("1 to remove, 4 kept", out)
        self.assertTrue(all(os.path.isdir(path) for path in self.paths.values()))

    def test_removes_only_merged_clean_worktrees(self):
        out = self.run_script()
        self.assertIn("removed %s (merged)" % self.paths["merged"], out)
        self.assertIn("keep    %s (changed, has changes)" % self.paths["changed"], out)
        self.assertIn("keep    %s (unmerged, not merged)" % self.paths["unmerged"], out)
        self.assertIn("keep    %s (locked, locked)" % self.paths["locked"], out)
        self.assertIn("(main, the main worktree)", out)
        self.assertFalse(os.path.exists(self.paths["merged"]))
        self.assertIn("merged", git(self.repo, "branch", "--list", "merged"), "the branch stays")
        for name in ("changed", "unmerged", "locked"):
            self.assertTrue(os.path.isdir(self.paths[name]), name)


if __name__ == "__main__":
    unittest.main()
