#!/usr/bin/env bash
# Default docs hook: gleam docs build, into build/dev/docs/
# (see docs_gleam in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_gleam
