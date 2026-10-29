#!/usr/bin/env bash
# Default docs hook: the build's docs step, where build.zig defines one
# (see docs_zig in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_zig
