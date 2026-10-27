#!/usr/bin/env bash
# Default clean hook: the package's npm scripts (see node_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
node_hook clean
