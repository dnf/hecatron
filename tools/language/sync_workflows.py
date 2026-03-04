#!/usr/bin/env python3
"""Re-copy all active language workflows with banners."""

import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from _helpers import load_added, project_root, sync_workflow


def main():
    root = project_root()
    added = load_added(root)
    for name in sorted(added):
        sync_workflow(name, root, "add")
    print(f"Synced {len(added)} workflow files")


if __name__ == "__main__":
    main()
