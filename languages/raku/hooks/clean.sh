#!/usr/bin/env bash
# Default clean hook: the .precomp/ caches Rakudo writes beside the modules
# it compiles.
source "$(dirname "$0")/../../_hooks.sh"
run find . -type d -name .precomp -prune -exec rm -rf {} +
