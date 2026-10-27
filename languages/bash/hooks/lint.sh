#!/usr/bin/env bash
# Default lint hook: ShellCheck on every .sh and .bash file, following what
# they source (and with the project's .shellcheckrc, if any).
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.sh' -o -name '*.bash'
[ "${#found_files[@]}" -gt 0 ] || skip "no .sh or .bash files"
run shellcheck -x "${found_files[@]}"
