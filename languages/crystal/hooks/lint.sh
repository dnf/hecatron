#!/usr/bin/env bash
# Default lint hook: Crystal's formatter check on src/ and spec/ (not lib/,
# the installed dependencies), and Ameba when the shard depends on it.
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/_shards.sh"
[ -f shard.yml ] || unknown "no shard.yml"
_sources=()
for _dir in src spec; do
    [ -d "${_dir}" ] && _sources+=("${_dir}")
done
[ "${#_sources[@]}" -gt 0 ] || unknown "no src/ or spec/"
checks_format
run crystal tool format --check "${_sources[@]}"
if grep -q '^[[:space:]]*ameba:' shard.yml; then
    shards_deps
    run bin/ameba
fi
