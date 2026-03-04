#!/usr/bin/env python3
"""Uninstall language toolchains."""

import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))
from _helpers import load_added, load_languages, project_root, resolve_name


def main():
    root = project_root()
    languages = load_languages(root)
    added = set(load_added(root))

    args = sys.argv[1:]
    if not args:
        print(
            "Usage: just uninstall <lang> [lang...] (try 'just uninstall help')",
            file=sys.stderr,
        )
        sys.exit(1)

    for lang in args:
        name = resolve_name(lang, languages)
        if name is None:
            print(f"Unknown language: {lang}", file=sys.stderr)
            sys.exit(1)
        if name not in added:
            print(
                f"Language '{name}' is not added. Run 'just add {name}' to add it.",
                file=sys.stderr,
            )
            sys.exit(1)

        script = os.path.join(root, ".hecatron", "languages", name, "uninstall.sh")
        result = subprocess.run(["bash", script])
        if result.returncode != 0:
            sys.exit(result.returncode)


if __name__ == "__main__":
    main()
