"""bin/hecatron's launcher: which binary it runs (docs/contributing/design.md §19)."""

import os
import shutil
import subprocess
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


class LauncherTest(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.home = self._tmp.name
        os.makedirs(os.path.join(self.home, "bin"))
        os.makedirs(os.path.join(self.home, ".git"))
        shutil.copy(os.path.join(HOME, "bin", "hecatron"), os.path.join(self.home, "bin", "hecatron"))
        with open(os.path.join(self.home, "VERSION"), "w") as f:
            f.write("9.9.9\n")
        # A branch checkout: .git/HEAD names the branch, not a commit.
        with open(os.path.join(self.home, ".git", "HEAD"), "w") as f:
            f.write("ref: refs/heads/work\n")
        binary = os.path.join(self.home, "bin", ".hecatron-go")
        with open(binary, "w") as f:
            f.write("#!/bin/sh\necho from-the-binary\n")
        os.chmod(binary, 0o755)

    def tearDown(self):
        self._tmp.cleanup()

    def built_for(self, commit):
        with open(os.path.join(self.home, "bin", ".hecatron-go.commit"), "w") as f:
            f.write(commit + "\n")

    def version(self, **env):
        environment = {k: v for k, v in os.environ.items() if k not in ("HECATRON_HOME", "HECATRON_NO_GO")}
        environment.update(env)
        result = subprocess.run(["bash", os.path.join(self.home, "bin", "hecatron"), "--version"],
                                env=environment, capture_output=True, text=True)
        return result.stdout.strip()

    def test_a_build_from_self_build_runs_on_any_branch(self):
        self.built_for("dev")
        self.assertEqual(self.version(), "from-the-binary")

    def test_a_release_binary_runs_only_at_its_commit(self):
        self.built_for("0123456789abcdef0123456789abcdef01234567")
        self.assertEqual(self.version(), "9.9.9")

    def in_a_worktree(self, gitdir):
        """Make the checkout a git worktree: .git is a file naming its git
        directory, which holds HEAD."""
        shutil.rmtree(os.path.join(self.home, ".git"))
        os.makedirs(os.path.join(self.home, gitdir) if not os.path.isabs(gitdir) else gitdir, exist_ok=True)
        where = gitdir if os.path.isabs(gitdir) else os.path.join(self.home, gitdir)
        with open(os.path.join(where, "HEAD"), "w") as f:
            f.write("0123456789abcdef0123456789abcdef01234567\n")
        with open(os.path.join(self.home, ".git"), "w") as f:
            f.write("gitdir: " + gitdir + "\n")

    def test_a_worktree_runs_its_self_built_binary(self):
        main = tempfile.TemporaryDirectory()
        self.addCleanup(main.cleanup)
        self.in_a_worktree(os.path.join(main.name, ".git", "worktrees", "hecatron-260"))
        self.built_for("dev")
        self.assertEqual(self.version(), "from-the-binary")

    def test_a_worktree_with_a_relative_gitdir(self):
        self.in_a_worktree(os.path.join("meta", "worktrees", "wt"))
        self.built_for("0123456789abcdef0123456789abcdef01234567")
        self.assertEqual(self.version(), "from-the-binary")

    def test_no_go_keeps_to_the_scripts(self):
        self.built_for("dev")
        self.assertEqual(self.version(HECATRON_NO_GO="1"), "9.9.9")


if __name__ == "__main__":
    unittest.main()
