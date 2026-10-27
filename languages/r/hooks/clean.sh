#!/usr/bin/env bash
# Default clean hook: the package tarballs R CMD build makes, R CMD check's
# directory, and compiled code in src/.
source "$(dirname "$0")/../../_hooks.sh"
[ -f DESCRIPTION ] || skip "no DESCRIPTION, so no package was built"
_package="$(sed -n 's/^Package:[[:space:]]*//p' DESCRIPTION | tr -d '[:space:]')"
[ -n "${_package}" ] || unknown "DESCRIPTION has no Package field"
run rm -rf "${_package}"_*.tar.gz "${_package}.Rcheck" src/*.o src/*.so src/*.dll
