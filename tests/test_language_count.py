"""Every place the docs state how many languages hecatron supports carries
the marker <!-- language-count -->, so `grep -rn language-count` finds them
all, and states the current count (CLAUDE.md, "Language count")."""

import os
import re
import subprocess
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MARKER = "<!-- language-count -->"
# A record of the past, not a statement of today's count.
HISTORICAL = {"CHANGELOG.md"}


def marked(line: str) -> bool:
    """Whether the line carries the marker, not just quotes it (`...`)."""
    return MARKER in line.replace(f"`{MARKER}`", "")


def language_count() -> int:
    languages = os.path.join(ROOT, "languages")
    return sum(1 for entry in os.listdir(languages)
               if not entry.startswith("_") and os.path.isfile(os.path.join(languages, entry, "config.json")))


def documents() -> list[str]:
    tracked = subprocess.run(["git", "ls-files", "*.md", "llms.txt"], cwd=ROOT,
                             capture_output=True, text=True, check=True).stdout.split()
    return [path for path in tracked if not path.startswith(("examples/", "tests/")) and path not in HISTORICAL]


class LanguageCountTest(unittest.TestCase):
    def test_marked_lines_state_the_current_count(self):
        count = str(language_count())
        for path in documents():
            with open(os.path.join(ROOT, path), encoding="utf-8") as f:
                for number, line in enumerate(f, 1):
                    if marked(line):
                        with self.subTest(at=f"{path}:{number}"):
                            self.assertRegex(line, rf"(?<![\d.]){count}(?![\d.])",
                                             f"says another count; hecatron has {count} languages")

    def test_lines_stating_the_count_are_marked(self):
        count = language_count()
        states_count = re.compile(rf"(?<![\d.]){count}(?![\d.]).{{0,40}}languages|languages-{count}\b", re.I)
        for path in documents():
            with open(os.path.join(ROOT, path), encoding="utf-8") as f:
                for number, line in enumerate(f, 1):
                    if states_count.search(line):
                        with self.subTest(at=f"{path}:{number}"):
                            self.assertTrue(marked(line), "states the language count without the marker")


if __name__ == "__main__":
    unittest.main()
