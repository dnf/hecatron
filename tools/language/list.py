#!/usr/bin/env python3
"""List project languages."""

import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from _helpers import load_added, load_languages, project_root


def main():
    root = project_root()
    languages = load_languages(root)
    added = set(load_added(root))
    show_all = "--all" in sys.argv[1:]

    all_langs = sorted(languages.keys())

    if show_all:
        for lang in all_langs:
            status = "" if lang in added else "  (removed)"
            print(f"  {lang}{status}")
        total = len(all_langs)
        added_count = len(added)
        removed_count = total - added_count
        print()
        print(f"{total} languages: {added_count} added, {removed_count} removed")
    else:
        added_langs = sorted(lang for lang in all_langs if lang in added)
        for lang in added_langs:
            print(f"  {lang}")
        print()
        print(f"{len(added_langs)} languages")


if __name__ == "__main__":
    main()
