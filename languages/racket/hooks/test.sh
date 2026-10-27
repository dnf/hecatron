#!/usr/bin/env bash
# Default test hook: raco test on every module here (their test submodules,
# honouring info.rkt's test-omit-paths).
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.rkt'
[ "${#found_files[@]}" -gt 0 ] || unknown "no .rkt files"
run raco test .
