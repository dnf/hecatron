#!/usr/bin/env bash
# Default docs hook: the package's docs script, else TypeDoc where it's a dependency.
source "$(dirname "$0")/../../_hooks.sh"
node_hook docs
