#!/usr/bin/env python3
"""Temporarily disable languages from parallel runs."""

import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from _helpers import disabled_path, load_languages, project_root, read_disabled, resolve_name, write_disabled


def main():
    root = project_root()
    languages = load_languages(root)
    all_langs = set(languages.keys())
    path = disabled_path(root)

    args = sys.argv[1:]

    # No args: show currently disabled
    if not args:
        disabled = read_disabled(path)
        if disabled:
            print("Currently disabled:")
            for lang in disabled:
                print(f"  {lang}")
        else:
            print("No languages are disabled. (try 'just disable help')")
        return

    # Specific languages: add to disabled list
    disabled = read_disabled(path)
    for lang in args:
        name = resolve_name(lang, languages)
        if name is None or name not in all_langs:
            print(f"Unknown language: {lang} (try 'just disable help')", file=sys.stderr)
            sys.exit(1)
        disabled.append(name)

    disabled = sorted(set(disabled))
    write_disabled(path, disabled)
    print(f"Disabled: {', '.join(disabled)}")


if __name__ == "__main__":
    main()
