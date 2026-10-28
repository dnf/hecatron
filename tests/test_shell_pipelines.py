"""`writer | grep -q PATTERN` under pipefail fails now and then: grep exits
at its first match, and a writer with more to write dies of SIGPIPE, so the
pipeline's status is the writer's 141. hecatron's scripts run with
`set -o pipefail`, so `printf '%s\\n' ${LIST} | grep -qx NAME` said a known
language was unknown, or an added one wasn't, once in a few thousand runs
under load (#169): a doctor that checked a language the project hadn't
added, `hecatron deps` failing, an installer not deferring its
dependencies. Lists are searched with _in_list (helpers.sh) or in_list
(_common.sh), and a command's output with `grep -q PATTERN <<<"$(command)"`."""

from __future__ import annotations

import glob
import os
import re
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture  # noqa: E402

# A pipe into grep -q (or -qx, -qF...), except from `head -1`, which writes
# its one line at once and is done.
EARLY_EXIT_GREP = re.compile(r"\|\s*grep\s+-[A-Za-z]*q")
ONE_LINE_WRITER = re.compile(r"\bhead\s+(?:-1|-n\s*1)\b[^|]*\|\s*grep\s+-[A-Za-z]*q")

# Far more than a pipe holds: the writer is still writing when grep exits.
LONG_LIST = "$(seq 100000)"


def shell_files():
    for pattern in ("bin/*", "libexec/**/*.sh", "languages/**/*.sh", "plugins/**/*.sh", "install.sh"):
        for path in sorted(glob.glob(os.path.join(HOME, pattern), recursive=True)):
            if os.path.isfile(path):
                yield path


class PipelinesTest(unittest.TestCase):
    def test_no_script_pipes_into_an_early_exiting_grep(self):
        problems = []
        for path in shell_files():
            with open(path, encoding="utf-8", errors="replace") as f:
                for number, line in enumerate(f, 1):
                    if line.lstrip().startswith("#"):
                        continue
                    if EARLY_EXIT_GREP.search(line) and not ONE_LINE_WRITER.search(line):
                        problems.append(f"{os.path.relpath(path, HOME)}:{number}: {line.strip()}")
        self.assertEqual(problems, [])

    def test_the_check_finds_one(self):
        self.assertTrue(EARLY_EXIT_GREP.search("printf '%s\\n' ${_ALL_LANGS} | grep -qx \"$1\""))
        self.assertTrue(EARLY_EXIT_GREP.search("dotnet --list-runtimes | grep -q x"))
        self.assertTrue(ONE_LINE_WRITER.search("head -1 \"$1\" 2>/dev/null | grep -qE '^#!'"))
        self.assertFalse(EARLY_EXIT_GREP.search("grep -q x <<<\"$(dotnet --list-runtimes)\""))


class ListMembershipTest(Fixture):
    """The searches find a name at the head of a long list, where piping the
    list into grep -q always failed."""

    def bash(self, script: str) -> subprocess.CompletedProcess:
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root}
        return subprocess.run(["bash", "-c", f"set -euo pipefail\n{script}"], cwd=self.root, env=env,
                              capture_output=True, text=True)

    def test_helpers_find_a_language_at_the_head_of_a_long_list(self):
        self.configure(languages=["go"])
        result = self.bash('source "${HECATRON_HOME}/libexec/lib/helpers.sh"\n'
                           f'_ALL_LANGS="go {LONG_LIST}" _ADDED_LANGS="go {LONG_LIST}" _TOOLS="shell {LONG_LIST}"\n'
                           "_is_known go && _is_tool shell && ! _is_removed go && echo found\n"
                           "_ADDED_LANGS=rust; _is_removed go && echo removed\n"
                           "_is_known nope || echo unknown")
        self.assertEqual(result.stdout.split(), ["found", "removed", "unknown"], result.stderr)

    def test_installers_find_a_name_at_the_head_of_a_long_list(self):
        result = self.bash(f'source "${{HECATRON_HOME}}/languages/_common.sh"\nin_list go go {LONG_LIST} && echo found\n'
                           "in_list go rust python || echo absent")
        self.assertEqual(result.stdout.split(), ["found", "absent"], result.stderr)


if __name__ == "__main__":
    unittest.main()
