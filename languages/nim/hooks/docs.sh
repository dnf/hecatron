#!/usr/bin/env bash
# Default docs hook: nim doc for the package's main module, into htmldocs/
# (see docs_nim in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_nim
