#!/usr/bin/env bash
# Default bench hook: the package's bench or benchmark script.
source "$(dirname "$0")/../../_hooks.sh"
node_hook bench
