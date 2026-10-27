#!/usr/bin/env bash
# Default lint hook: Perl::Critic over the code (lib, bin, script), where the
# project configures it (.perlcriticrc).
source "$(dirname "$0")/../../_hooks.sh"
[ -f .perlcriticrc ] || skip "no .perlcriticrc"
_paths=()
for _path in lib bin script; do
    if [ -e "${_path}" ]; then _paths+=("${_path}"); fi
done
[ "${#_paths[@]}" -gt 0 ] || skip "no Perl sources to criticise"
run perlcritic --profile .perlcriticrc "${_paths[@]}"
