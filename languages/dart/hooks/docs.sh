#!/usr/bin/env bash
# Default docs hook: dart doc, into doc/api
# (see docs_dart in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_dart
