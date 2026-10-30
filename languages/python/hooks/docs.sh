#!/usr/bin/env bash
# Default docs hook: Sphinx where the project has docs/conf.py, else pdoc where it's a dependency
# (see docs_python in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_python
