#!/usr/bin/env python3
"""Remove language(s) from the project's active set."""

import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from _helpers import (
    load_added,
    load_languages,
    project_root,
    regenerate_dockerfile,
    resolve_name,
    save_added,
    sync_workflow,
)


def main():
    root = project_root()
    languages = load_languages(root)
    added_set = set(load_added(root))

    args = sys.argv[1:]
    if not args:
        print("Usage: remove.py <lang> [lang ...]", file=sys.stderr)
        sys.exit(1)

    changes = []
    for arg in args:
        name = resolve_name(arg, languages)
        if name is None:
            print(f"Unknown language: {arg}", file=sys.stderr)
            sys.exit(1)
        if name not in added_set:
            print(f"Not added: {name}")
        else:
            changes.append(name)

    if changes:
        for name in changes:
            added_set.discard(name)
            sync_workflow(name, root, "remove")
        save_added(root, added_set)
        for name in sorted(changes):
            print(f"Removed: {name}")
        regenerate_dockerfile(root)


if __name__ == "__main__":
    main()
