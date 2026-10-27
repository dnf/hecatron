#!/usr/bin/env bash
# Default build hook: raco make compiles every module here into compiled/.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.rkt'
[ "${#found_files[@]}" -gt 0 ] || unknown "no .rkt files"
run raco make -v "${found_files[@]}"
