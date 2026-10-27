#!/usr/bin/env bash
# Default format hook: Crystal's formatter on src/ and spec/ (not lib/, the
# installed dependencies).
source "$(dirname "$0")/../../_hooks.sh"
[ -f shard.yml ] || unknown "no shard.yml"
_sources=()
for _dir in src spec; do
    [ -d "${_dir}" ] && _sources+=("${_dir}")
done
[ "${#_sources[@]}" -gt 0 ] || unknown "no src/ or spec/"
if format_checking; then run crystal tool format --check "${_sources[@]}"; else run crystal tool format "${_sources[@]}"; fi
