#!/usr/bin/env python3
"""Keep the Agent Skill (plugins/hecatron/skills/hecatron/) in step with hecatron: SKILL.md's
hecatron-version from VERSION, references/reference.md a copy of
docs/agents.md (the reference `hct help agents` prints), and
references/languages.md each language's default tools from its definition.
Also checks SKILL.md's frontmatter against the Agent Skills specification
(https://agentskills.io/specification), which other agents read too.

    python3 libexec/codegen/skill.py           # write the files
    python3 libexec/codegen/skill.py --check   # fail if any is out of date or invalid
"""

from __future__ import annotations

import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
SKILL = os.path.join(ROOT, "plugins", "hecatron", "skills", "hecatron")
PLUGIN = os.path.join(ROOT, "plugins", "hecatron")
LANGUAGES = os.path.join(ROOT, "languages")

# The frontmatter keys the specification allows; claude.ai and the Claude
# API refuse a skill with any other.
ALLOWED_KEYS = {"name", "description", "license", "compatibility", "metadata", "allowed-tools"}
VERSION_LINE = re.compile(r'^(  hecatron-version: )"[^"]*"$', re.M)


def read(path: str) -> str:
    with open(path, encoding="utf-8") as f:
        return f.read()


def skill_md() -> str:
    version = read(os.path.join(ROOT, "VERSION")).strip()
    return VERSION_LINE.sub(lambda m: f'{m.group(1)}"{version}"', read(os.path.join(SKILL, "SKILL.md")), count=1)


def reference_md() -> str:
    return ("<!-- Generated from docs/agents.md by libexec/codegen/skill.py; edit that. -->\n\n"
            + read(os.path.join(ROOT, "docs", "agents.md")))


def languages_md() -> str:
    lines = ["<!-- Generated from languages/*/config.json by libexec/codegen/skill.py. -->",
             "",
             "# Each language's default tools",
             "",
             "What `hct test`, `lint`, `format` and `build` run for a language when the",
             "project has no hook of its own (`.hecatron/languages/<lang>/<action>.sh`).",
             "The key is what to type (`hct test csharp`).",
             "",
             "| Language | Key | Build | Test | Lint | Format |",
             "| --- | --- | --- | --- | --- | --- |"]
    found = []
    for lang in sorted(os.listdir(LANGUAGES)):
        path = os.path.join(LANGUAGES, lang, "config.json")
        if os.path.isfile(path):
            found.append((lang, json.loads(read(path))))
    for lang, config in sorted(found, key=lambda item: item[1].get("name", item[0]).lower()):
        cells = [config.get("name", lang), f"`{lang}`", ", ".join(config.get("buildSystems") or []),
                 config.get("testing"), config.get("linting"), config.get("formatting")]
        lines.append("| " + " | ".join((cell or "–").replace("|", "\\|") for cell in cells) + " |")
    return "\n".join(lines) + "\n"


def problems(text: str) -> list[str]:
    """What's wrong with SKILL.md's frontmatter, by the specification."""
    match = re.match(r"---\n(.*?)\n---\n", text, re.S)
    if not match:
        return ["SKILL.md has no frontmatter"]
    keys, fields = [], {}
    for line in match.group(1).splitlines():
        if line and not line.startswith(" "):
            key, _, value = line.partition(":")
            keys.append(key)
            fields[key] = value.strip()
    found = []
    if set(keys) - ALLOWED_KEYS:
        found.append(f"keys the specification doesn't allow: {', '.join(sorted(set(keys) - ALLOWED_KEYS))}")
    name = fields.get("name", "")
    if not re.fullmatch(r"[a-z0-9]+(-[a-z0-9]+)*", name) or len(name) > 64 or name != os.path.basename(SKILL):
        found.append(f"name {name!r}: 1-64 lowercase letters, digits and single hyphens, the same as the directory")
    if not 0 < len(fields.get("description", "")) <= 1024:
        found.append("description must be 1-1024 characters")
    if "compatibility" in fields and not 0 < len(fields["compatibility"]) <= 500:
        found.append("compatibility must be 1-500 characters")
    if len(text.splitlines()) > 500:
        found.append("SKILL.md is over 500 lines; move detail to references/")
    return found


def plugin_json() -> str:
    """The Claude Code plugin's manifest; its version is hecatron's, so an
    installed plugin updates with each release, not each commit."""
    manifest = {
        "name": "hecatron",
        "version": read(os.path.join(ROOT, "VERSION")).strip(),
        "description": "Build, test, lint and format code in almost any language through hecatron's hct, "
                       "and know when to ask before installing toolchains.",
        "author": {"name": "dnf", "url": "https://github.com/dnf"},
        "homepage": "https://github.com/dnf/hecatron",
        "repository": "https://github.com/dnf/hecatron",
        "license": "MIT",
    }
    return json.dumps(manifest, indent=2) + "\n"


def outputs() -> dict[str, str]:
    return {
        os.path.join(PLUGIN, ".claude-plugin", "plugin.json"): plugin_json(),
        os.path.join(SKILL, "SKILL.md"): skill_md(),
        os.path.join(SKILL, "references", "reference.md"): reference_md(),
        os.path.join(SKILL, "references", "languages.md"): languages_md(),
    }


def main(argv: list[str]) -> int:
    wanted = outputs()
    found = problems(wanted[os.path.join(SKILL, "SKILL.md")])
    for problem in found:
        print(f"plugins/hecatron/skills/hecatron/SKILL.md: {problem}", file=sys.stderr)
    if "--check" in argv:
        stale = [os.path.relpath(path, ROOT) for path, text in wanted.items()
                 if not os.path.isfile(path) or read(path) != text]
        for path in stale:
            print(f"{path} is out of date: run python3 libexec/codegen/skill.py", file=sys.stderr)
        return 1 if stale or found else 0
    for path, text in wanted.items():
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
