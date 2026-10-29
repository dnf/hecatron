#!/usr/bin/env bash
# Default docs hook: dub build -b docs, into docs/
# (see docs_d in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_d
