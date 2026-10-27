"""libexec/maintenance/check_links.py: which links it finds and how it
judges an answer (the weekly workflow runs it against the real sites)."""

import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "libexec", "maintenance"))
import check_links  # noqa: E402


class LinksTest(unittest.TestCase):
    def test_finds_links_with_where_they_are(self):
        with tempfile.TemporaryDirectory() as root:
            with open(os.path.join(root, "a.md"), "w", encoding="utf-8") as f:
                f.write("See [uv](https://docs.astral.sh/uv/), and https://go.dev/dl.\n"
                        "Again: <https://docs.astral.sh/uv/>\n"
                        "Not these: https://example.com/x https://github.com/{owner} http://localhost:8080\n")
            with mock.patch.object(check_links, "ROOT", root):
                found = check_links.links(["a.md"])
        self.assertEqual(found, {"https://docs.astral.sh/uv/": ["a.md:1", "a.md:2"], "https://go.dev/dl": ["a.md:1"]})


class VerdictTest(unittest.TestCase):
    def test_checks_the_advisor_rules_links(self):
        documents = check_links.documents()
        self.assertIn("languages/_advice.json", documents)
        self.assertIn("languages/go/advice.json", documents)

    def test_answers(self):
        self.assertEqual(check_links.verdict(200), "ok")
        self.assertEqual(check_links.verdict(301), "ok")
        self.assertEqual(check_links.verdict(404), "broken")
        self.assertEqual(check_links.verdict(410), "broken")
        self.assertEqual(check_links.verdict(503), "broken")
        self.assertEqual(check_links.verdict(None), "broken")

    def test_refusing_automated_clients_isnt_broken(self):
        for status in (401, 402, 403, 429):
            with self.subTest(status=status):
                self.assertEqual(check_links.verdict(status), "blocked")


if __name__ == "__main__":
    unittest.main()
