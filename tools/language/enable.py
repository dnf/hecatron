#!/usr/bin/env python3
"""Re-enable previously disabled languages."""

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
            print()
            print("Use 'just enable <lang>' or 'just enable --all' to re-enable.")
        else:
            print("No languages are disabled.")
        return

    # --all: remove the file entirely
    if args == ["--all"]:
        write_disabled(path, [])
        print("All languages re-enabled.")
        return

    # Specific languages: remove from disabled list
    disabled = read_disabled(path)
    if not disabled:
        print("No languages are currently disabled.")
        return

    for lang in args:
        name = resolve_name(lang, languages)
        if name is None or name not in all_langs:
            print(f"Unknown language: {lang} (try 'just enable help')", file=sys.stderr)
            sys.exit(1)
        disabled = [d for d in disabled if d != name]

    write_disabled(path, disabled)
    if disabled:
        print(f"Still disabled: {', '.join(disabled)}")
    else:
        print("All languages re-enabled.")


if __name__ == "__main__":
    main()
