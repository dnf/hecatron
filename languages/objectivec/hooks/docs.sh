#!/usr/bin/env bash
# Default docs hook: Doxygen, where the project has a Doxyfile
# (see docs_doxygen in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_doxygen
