#!/usr/bin/env bash
# Default prune hook: with --caches, npm's (and pnpm's) own trim of its
# cache (prune_node in languages/_hooks.sh). node_modules is the installed
# dependencies, which prune leaves.
source "$(dirname "$0")/../../_hooks.sh"
prune_node
