#!/usr/bin/env bash
# Default clean hook: what compiling leaves beside the sources: nimcache/
# directories, and the executable next to each tests/t*.nim (nimble test's)
# and each executable nimble build compiled into the package's root.
source "$(dirname "$0")/../../_hooks.sh"
exists '*.nimble' || unknown "no .nimble file"
run find . -name nimcache -type d -prune -exec rm -rf {} +
for _source in tests/t*.nim ./*.nim src/*.nim; do
    [ -f "${_source}" ] || continue
    for _binary in "${_source%.nim}" "./$(basename "${_source}" .nim)"; do
        if [ -f "${_binary}" ] && [ -x "${_binary}" ]; then
            run rm -f "${_binary}"
        fi
    done
done
