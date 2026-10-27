#!/usr/bin/env python3
"""Write docs/languages.md, the table of every language hecatron supports,
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
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
LANGUAGES = os.path.join(ROOT, "languages")
DOC = os.path.join(ROOT, "docs", "languages.md")

PLATFORM_NAMES = {
    "linux-aarch64": "Linux arm64",
    "linux-x86_64": "Linux x86_64",
    "macos-aarch64": "macOS Apple Silicon",
    "macos-x86_64": "Intel Macs",
}

HEADER = """\
# Languages

Every language hecatron supports, what it is, and how hecatron builds,
tests and lints it by default. A project overrides any default with its own
hook (`.hecatron/languages/<lang>/<action>.sh`).

Generated from each language's `config.json` by
`libexec/codegen/languages_doc.py`; don't edit by hand.
[coverage.md](coverage.md) has which platforms each one installs and passes
on, and [adding-a-language.md](adding-a-language.md) covers adding one.

| Language | What it is | Build systems | Tests | Linting | Default version |
| --- | --- | --- | --- | --- | --- |
"""


def definitions() -> list[tuple[str, dict]]:
    found = []
    for lang in sorted(os.listdir(LANGUAGES)):
        path = os.path.join(LANGUAGES, lang, "config.json")
        if os.path.isfile(path):
            with open(path, encoding="utf-8") as f:
                found.append((lang, json.load(f)))
    return sorted(found, key=lambda item: item[1].get("name", item[0]).lower())


def cell(text: str | None) -> str:
    return (text or "–").replace("|", "\\|")


def row(lang: str, config: dict) -> str:
    name = config.get("name", lang)
    home = config.get("homepage")
    language = f"[{name}]({home})" if home else name
    summary = config.get("summary", "")
    unsupported = [PLATFORM_NAMES.get(p, p) for p in config.get("unsupported", [])]
    if unsupported:
        summary += f" *Not on {', '.join(unsupported)}: no toolchain upstream.*"
    builds = ", ".join(config.get("buildSystems") or []) or None
    version = config.get("version", "").lstrip("=") or None
    if version and config.get("track") == "lts":
        version += " (LTS)"
    return "| " + " | ".join([language, cell(summary), cell(builds), cell(config.get("testing")),
                              cell(config.get("linting")), cell(version)]) + " |"


def render() -> str:
    rows = [row(lang, config) for lang, config in definitions()]
    return HEADER + "\n".join(rows) + "\n"


def main() -> int:
    text = render()
    if "--check" in sys.argv[1:]:
        with open(DOC, encoding="utf-8") as f:
            if f.read() != text:
                print("docs/languages.md is out of date: run python3 libexec/codegen/languages_doc.py",
                      file=sys.stderr)
                return 1
        return 0
    with open(DOC, "w", encoding="utf-8") as f:
        f.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
