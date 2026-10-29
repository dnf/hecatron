#!/usr/bin/env bash
# Default docs hook: crystal docs, into docs/ (see docs_crystal in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/_shards.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_crystal
