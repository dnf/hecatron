"""docs/languages.md matches the language definitions, and every definition
has what the table shows: a name, a homepage and a summary."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GENERATOR = os.path.join(ROOT, "libexec", "codegen", "languages_doc.py")


class LanguagesDocTest(unittest.TestCase):
    def test_doc_is_current(self):
        result = subprocess.run([sys.executable, GENERATOR, "--check"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_every_definition_describes_itself(self):
        languages = os.path.join(ROOT, "languages")
        for lang in sorted(os.listdir(languages)):
            path = os.path.join(languages, lang, "config.json")
            if not os.path.isfile(path):
                continue
            with open(path, encoding="utf-8") as f:
                config = json.load(f)
            with self.subTest(lang=lang):
                for key in ("name", "homepage", "summary", "testing"):
                    self.assertTrue(config.get(key), f"{lang}: no {key}")
                self.assertTrue(config["homepage"].startswith("https://"), config["homepage"])
                self.assertIn("buildSystems", config)
                self.assertIn("linting", config)


if __name__ == "__main__":
    unittest.main()
