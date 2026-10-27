#!/usr/bin/env bash
# Default build hook: shards build for a shard with targets, else a type check of the library (src/<name>.cr).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/_shards.sh"
[ -f shard.yml ] || unknown "no shard.yml"
shards_deps
if [ -n "$(shard_targets)" ]; then
    run shards build
else
    _main="src/$(shard_name).cr"
    [ -f "${_main}" ] || unknown "no targets in shard.yml and no ${_main}"
    run crystal build --no-codegen "${_main}"
fi
