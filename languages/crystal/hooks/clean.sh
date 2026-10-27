#!/usr/bin/env bash
# Default clean hook: the executables `shards build` made in bin/ (crystal
# spec builds in Crystal's cache, outside the shard; lib/ holds installed
# dependencies, so it stays).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/_shards.sh"
[ -f shard.yml ] || unknown "no shard.yml"
_targets="$(shard_targets)"
[ -n "${_targets}" ] || skip "no targets in shard.yml, so nothing is built in the shard"
while IFS= read -r _target; do
    run rm -f "bin/${_target}" "bin/${_target}.dwarf"
done <<< "${_targets}"
