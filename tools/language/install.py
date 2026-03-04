#!/usr/bin/env python3
"""Install language toolchains."""

import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))
from _helpers import load_added, load_languages, project_root, resolve_name


def main():
    root = project_root()
    languages = load_languages(root)
    added = sorted(load_added(root))

    args = sys.argv[1:]
    install_all = not args
    targets = added if install_all else args

    failed = []
    for lang in targets:
        if install_all:
            name = lang
        else:
            name = resolve_name(lang, languages)
            if name is None:
                print(f"Unknown language: {lang}", file=sys.stderr)
                sys.exit(1)
            if name not in set(added):
                print(
                    f"Language '{name}' is not added. Run 'just add {name}' to add it.",
                    file=sys.stderr,
                )
                sys.exit(1)

        script = os.path.join(root, ".hecatron", "languages", name, "install.sh")
        result = subprocess.run(["bash", script])
        if result.returncode != 0:
            if install_all:
                failed.append(name)
            else:
                sys.exit(result.returncode)

    if failed:
        print(file=sys.stderr)
        print(f"Failed to install: {' '.join(failed)}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
