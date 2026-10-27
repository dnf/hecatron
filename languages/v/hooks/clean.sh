#!/usr/bin/env bash
# Default clean hook: what the build hook leaves, named after the directory
# as V names it: the executable (<dir>) or the shared library (<dir>.so,
# .dylib or .dll).
source "$(dirname "$0")/../../_hooks.sh"
[ -f v.mod ] || unknown "no v.mod"
_name="$(basename "$(pwd)")"
run rm -f "${_name}" "${_name}.so" "${_name}.dylib" "${_name}.dll"
