#!/usr/bin/env bash
# Default test hook: crystal spec, after installing the shard's dependencies.
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/_shards.sh"
[ -f shard.yml ] || unknown "no shard.yml"
shards_deps
run crystal spec
