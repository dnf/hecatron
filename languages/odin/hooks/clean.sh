#!/usr/bin/env bash
# Default clean hook: what the build hook leaves, named after the directory
# as Odin names it: the executable (<dir>, or <dir>.bin), the static library
# (<dir>.a) or a shared one (<dir>.so).
source "$(dirname "$0")/../../_hooks.sh"
exists '*.odin' || unknown "no .odin files"
_name="$(basename "$(pwd)")"
run rm -f "${_name}" "${_name}.bin" "${_name}.a" "${_name}.so"
