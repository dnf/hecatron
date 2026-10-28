"""A shell function defined twice where both definitions are sourced together:
the later one silently replaces the earlier (a second default_version in
languages/_common.sh broke every installer that called the first form)."""

import collections
import glob
import os
import re
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEFINITION = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*)\s*\(\)")
SOURCED = re.compile(r"^\s*(?:source|\.)\s+.*/(_[a-z]+\.sh)\b")


def shared_files():
    paths = glob.glob(os.path.join(HOME, "languages", "_*.sh")) + glob.glob(os.path.join(HOME, "libexec", "lib", "*.sh"))
    return {os.path.basename(path): path for path in sorted(paths)}


def sourced_together(name, files, seen=None):
    seen = seen if seen is not None else []
    if name in seen or name not in files:
        return seen
    seen.append(name)
    with open(files[name], encoding="utf-8") as f:
        for line in f:
            match = SOURCED.match(line)
            if match:
                sourced_together(match.group(1), files, seen)
    return seen


class ShellFunctionsTest(unittest.TestCase):
    def test_no_function_is_defined_twice_where_sourced_together(self):
        files = shared_files()
        problems = []
        for name in files:
            definitions = collections.defaultdict(list)
            for member in sourced_together(name, files):
                with open(files[member], encoding="utf-8") as f:
                    for number, line in enumerate(f, 1):
                        match = DEFINITION.match(line)
                        if match:
                            definitions[match.group(1)].append(f"{member}:{number}")
            problems += [f"{function}: {', '.join(where)}" for function, where in definitions.items() if len(where) > 1]
        self.assertEqual(sorted(set(problems)), [])

    def test_the_check_finds_a_duplicate(self):
        self.assertTrue(DEFINITION.match("default_version() {"))
        self.assertEqual(SOURCED.match('source "$(dirname "$0")/../_common.sh"').group(1), "_common.sh")


if __name__ == "__main__":
    unittest.main()
