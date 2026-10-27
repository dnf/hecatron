#!/usr/bin/env bash
# Default deps hook: shards install, --frozen with a shard.lock (see deps_crystal in
# languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/_shards.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_crystal
