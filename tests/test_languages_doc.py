"""docs/guide/languages.md matches the language definitions, and every definition
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

    def test_every_language_is_listed_once(self):
        with open(os.path.join(ROOT, "docs", "guide", "languages.md"), encoding="utf-8") as f:
            text = f.read()
        popular, additional = text.split("## Popular languages")[1].split("## Additional languages")
        rows = [line for section in (popular, additional) for line in section.splitlines() if line.startswith("| [")]
        definitions = [d for d in os.listdir(os.path.join(ROOT, "languages"))
                       if os.path.isfile(os.path.join(ROOT, "languages", d, "config.json"))]
        self.assertEqual(len(rows), len(definitions))
        self.assertEqual(len(rows), len(set(rows)))

    def test_every_tool_link_is_used_and_https(self):
        with open(os.path.join(ROOT, "libexec", "codegen", "tool-links.json"), encoding="utf-8") as f:
            links = json.load(f)
        with open(os.path.join(ROOT, "docs", "guide", "languages.md"), encoding="utf-8") as f:
            text = f.read()
        for name, url in links.items():
            with self.subTest(tool=name):
                self.assertTrue(url.startswith("https://"), url)
                self.assertIn(f"[{name}]({url})", text, "named by no language definition")

    def test_notes_name_a_tool_column(self):
        languages = os.path.join(ROOT, "languages")
        for lang in sorted(os.listdir(languages)):
            path = os.path.join(languages, lang, "config.json")
            if os.path.isfile(path):
                with open(path, encoding="utf-8") as f:
                    notes = json.load(f).get("notes") or {}
                with self.subTest(lang=lang):
                    self.assertLessEqual(set(notes), {"buildSystems", "testing", "linting", "formatting"})

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
