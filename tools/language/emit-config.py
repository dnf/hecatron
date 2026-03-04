#!/usr/bin/env python3
"""Emit shell variables + functions from language config for eval.

Usage in helpers.sh:
    eval "$(python3 "${_HELPERS_DIR}/emit-config.py")"
"""

import json
import os
import sys


def main():
    # Four levels up: language/ → tools/ → .hecatron/ → project root
    root = os.path.dirname(
        os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    )
    config_dir = os.path.join(root, ".hecatron", "config")

    # Read languages.json (tools only — added/disabled are separate files)
    config_path = os.path.join(config_dir, "languages.json")
    try:
        with open(config_path) as f:
            config = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError) as e:
        print(f"echo 'ERROR: Failed to read {config_path}: {e}' >&2; exit 1",
              file=sys.stdout)
        sys.exit(1)

    tools = config.get("tools", {})

    # Discover languages from per-language config files
    languages = {}
    langs_dir = os.path.join(root, ".hecatron", "languages")
    for entry in sorted(os.listdir(langs_dir)):
        cfg = os.path.join(langs_dir, entry, "config.json")
        if os.path.isfile(cfg):
            with open(cfg) as f:
                languages[entry] = json.load(f)

    # Read added languages
    added_path = os.path.join(config_dir, "languages", "added.json")
    with open(added_path) as f:
        added = set(json.load(f))

    # Read disabled languages
    disabled_path = os.path.join(config_dir, "languages", "disabled.json")
    disabled = set()
    if os.path.isfile(disabled_path):
        with open(disabled_path) as f:
            disabled = set(json.load(f))

    # All known languages (alphabetical) — used for validation and help text
    all_langs = sorted(languages.keys())
    # Added languages (alphabetical) — the active set
    added_langs = sorted(lang for lang in all_langs if lang in added)
    # Filtered lists (only added languages)
    compiled_langs = sorted(
        lang for lang in added_langs if languages[lang].get("compiled"))
    clean_langs = sorted(
        lang for lang in added_langs if languages[lang].get("hasClean"))
    lint_langs = sorted(
        lang for lang in added_langs if languages[lang].get("hasLint"))
    no_build_langs = sorted(
        lang for lang in added_langs if not languages[lang].get("compiled"))

    # Emit language list variables (newline-separated for compatibility)
    def emit_list(name, langs):
        # Match the original format: newline before first, after last
        if langs:
            value = "\n" + "\n".join(langs) + "\n"
        else:
            value = ""
        print(f'{name}="{value}"')

    # Disabled languages (alphabetical)
    disabled_langs = sorted(lang for lang in added_langs if lang in disabled)

    emit_list("_ALL_LANGS", all_langs)
    emit_list("_ADDED_LANGS", added_langs)
    emit_list("_COMPILED_LANGS", compiled_langs)
    emit_list("_CLEAN_LANGS", clean_langs)
    emit_list("_LINT_LANGS", lint_langs)
    emit_list("_NO_BUILD_LANGS", no_build_langs)
    emit_list("_DISABLED_LANGS", disabled_langs)

    # Emit _is_disabled() function
    print()
    print("_is_disabled() {")
    print('    echo "${_DISABLED_LANGS}" | grep -qx "$1"')
    print("}")

    # Emit _name() function: abbreviation → full name
    print()
    print("_name() {")
    print('    case "$1" in')
    for lang in all_langs:
        abbr = languages[lang].get("abbreviation")
        if abbr:
            print(f"        {abbr}) echo {lang} ;;")
    print('        *) echo "$1" ;;')
    print("    esac")
    print("}")

    # Emit _abbr() function: full name → abbreviation
    print()
    print("_abbr() {")
    print('    case "$1" in')
    for lang in all_langs:
        abbr = languages[lang].get("abbreviation")
        if abbr:
            # Pad the lang name for alignment
            print(f'        {lang}) echo "{abbr}" ;;')
    print("    esac")
    print("}")

    # Emit _dep() function: language/tool → build dependency
    print()
    print("_dep() {")
    print('    case "$1" in')
    # Group languages that share the same dependency for compact output
    dep_groups = {}
    for lang in all_langs:
        dep = languages[lang].get("dependency")
        if dep:
            dep_groups.setdefault(dep, []).append(lang)
    # Add tools
    for tool_name, tool_data in sorted(tools.items()):
        dep = tool_data.get("dependency")
        if dep:
            dep_groups.setdefault(dep, []).append(tool_name)
    # Emit grouped
    emitted = set()
    for lang in all_langs:
        dep = languages[lang].get("dependency")
        if dep and dep not in emitted:
            emitted.add(dep)
            members = sorted(dep_groups[dep])
            pattern = "|".join(members)
            # Pad for alignment
            print(f"        {pattern})", end="")
            print(f" echo {dep} ;;")
    # Emit any tool-only deps not yet emitted
    for tool_name, tool_data in sorted(tools.items()):
        dep = tool_data.get("dependency")
        if dep and dep not in emitted:
            emitted.add(dep)
            print(f"        {tool_name}) echo {dep} ;;")
    print("    esac")
    print("}")

    # Emit _lint_dep() function: language → lint-specific dependency
    print()
    print("_lint_dep() {")
    print('    case "$1" in')
    for lang in all_langs:
        if not languages[lang].get("hasLint"):
            continue
        lint_dep = languages[lang].get("lintDependency")
        dep = lint_dep if lint_dep else languages[lang].get("dependency")
        if dep:
            print(f"        {lang}) echo {dep} ;;")
    print("    esac")
    print("}")


if __name__ == "__main__":
    main()
