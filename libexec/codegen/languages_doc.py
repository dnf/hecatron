#!/usr/bin/env python3
"""Write docs/guide/languages.md, the table of every language hecatron supports,
from the language definitions (languages/<lang>/config.json): its name,
homepage and summary, the build systems, test runners and linters the default
hooks use, the default toolchain version, and the platforms with no
toolchain upstream.

    python3 libexec/codegen/languages_doc.py           # write the file
    python3 libexec/codegen/languages_doc.py --check   # fail if it's out of date
"""

from __future__ import annotations

import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
LANGUAGES = os.path.join(ROOT, "languages")
DOC = os.path.join(ROOT, "docs", "guide", "languages.md")
# Each tool the tables name, linked to its own site, so a reader can find out
# what it is.
TOOL_LINKS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "tool-links.json")

PLATFORM_NAMES = {
    "linux-aarch64": "Linux arm64",
    "linux-x86_64": "Linux x86_64",
    "macos-aarch64": "macOS Apple Silicon",
    "macos-x86_64": "Intel Macs",
}

# The languages most developers use: those hecatron
# supports that at least 5% of respondents to the Stack Overflow Developer
# Survey 2025 used (https://survey.stackoverflow.co/2025/technology). Listed
# in a table of their own above the rest, so a reader finds theirs without
# scrolling.
POPULAR = [
    "javascript",
    "python",
    "bash",
    "typescript",
    "java",
    "csharp",
    "cpp",
    "powershell",
    "c",
    "php",
    "go",
    "rust",
    "kotlin",
    "lua",
    "ruby",
    "dart",
    "swift",
]

INTRO = """\
# Languages

Every language hecatron supports, what it is, and how hecatron builds,
tests, lints and formats it by default. A project overrides any default with its own
hook (`.hecatron/languages/<lang>/<action>.sh`).

Generated from each language's `config.json` by
`libexec/codegen/languages_doc.py`; don't edit by hand.
[coverage.md](coverage.md) has which platforms each one installs and passes
on, and [adding-a-language.md](../contributing/adding-a-language.md) covers adding one.
"""

POPULAR_HEADING = """\
## Popular languages

The languages at least 5% of developers use, from the
[Stack Overflow Developer Survey 2025](https://survey.stackoverflow.co/2025/technology).
"""

ADDITIONAL_HEADING = """\
## Additional languages
"""

TABLE_HEADER = """\
| Language | What it is | Build systems | Tests | Linting | Formatting | Default version |
| --- | --- | --- | --- | --- | --- | --- |
"""


def definitions() -> list[tuple[str, dict]]:
    found = []
    for lang in sorted(os.listdir(LANGUAGES)):
        path = os.path.join(LANGUAGES, lang, "config.json")
        if os.path.isfile(path):
            with open(path, encoding="utf-8") as f:
                found.append((lang, json.load(f)))
    return sorted(found, key=lambda item: item[1].get("name", item[0]).lower())


class Notes:
    """The footnotes of the document, numbered in order of first use; the same
    text gets one number wherever it's used."""

    def __init__(self) -> None:
        self.texts: list[str] = []

    def ref(self, text: str) -> str:
        if text not in self.texts:
            self.texts.append(text)
        return f"[^{self.texts.index(text) + 1}]"

    def render(self) -> str:
        return "\n".join(f"[^{number}]: {text}" for number, text in enumerate(self.texts, 1)) + "\n"


def link_tools(text: str, links: dict[str, str]) -> str:
    """The text with each tool it names linked, longest names first, so
    `cargo test` wins over a shorter name inside it. A name is a whole word:
    `Pkg` isn't linked inside `Pkg.test`, but `ESLint` is before `...`."""
    names = sorted(links, key=len, reverse=True)
    pattern = re.compile(r"(?<![\w.:/-])(" + "|".join(re.escape(name) for name in names) + r")(?![\w:/-])(?!\.\w)")
    return pattern.sub(lambda m: f"[{m.group(1)}]({links[m.group(1)]})", text)


def cell(text: str | None, links: dict[str, str], note: str = "") -> str:
    if not text:
        return "–"
    return link_tools(text.replace("|", "\\|"), links) + note


def row(lang: str, config: dict, links: dict[str, str], notes: Notes) -> str:
    name = config.get("name", lang)
    home = config.get("homepage")
    language = f"[{name}]({home})" if home else name
    unsupported = [PLATFORM_NAMES.get(p, p) for p in config.get("unsupported", [])]
    if unsupported:
        language += notes.ref(f"Not on {', '.join(unsupported)}: no toolchain upstream.")
    remarks = config.get("notes") or {}
    note = lambda column: notes.ref(remarks[column]) if remarks.get(column) else ""  # noqa: E731
    builds = ", ".join(config.get("buildSystems") or []) or None
    version = config.get("version", "").lstrip("=") or None
    if version and config.get("track") == "lts":
        version += " (LTS)"
    cells = [language, (config.get("summary") or "–").replace("|", "\\|"),
             cell(builds, links, note("buildSystems")), cell(config.get("testing"), links, note("testing")),
             cell(config.get("linting"), links, note("linting")), cell(config.get("formatting"), links, note("formatting")),
             version or "–"]
    return "| " + " | ".join(cells) + " |"


def table(entries: list[tuple[str, dict]], links: dict[str, str], notes: Notes) -> str:
    return TABLE_HEADER + "\n".join(row(lang, config, links, notes) for lang, config in entries) + "\n"


def render() -> str:
    with open(TOOL_LINKS, encoding="utf-8") as f:
        links = json.load(f)
    found = dict(definitions())
    missing = [lang for lang in POPULAR if lang not in found]
    if missing:
        raise SystemExit(f"languages_doc: POPULAR names languages with no definition: {', '.join(missing)}")
    popular = [(lang, config) for lang, config in definitions() if lang in POPULAR]
    additional = [(lang, config) for lang, config in definitions() if lang not in POPULAR]
    notes = Notes()
    tables = [POPULAR_HEADING, table(popular, links, notes), ADDITIONAL_HEADING, table(additional, links, notes)]
    return "\n".join([INTRO, *tables, notes.render()])


def main() -> int:
    text = render()
    if "--check" in sys.argv[1:]:
        with open(DOC, encoding="utf-8") as f:
            if f.read() != text:
                print("docs/guide/languages.md is out of date: run python3 libexec/codegen/languages_doc.py",
                      file=sys.stderr)
                return 1
        return 0
    with open(DOC, "w", encoding="utf-8") as f:
        f.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
