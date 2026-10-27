#!/usr/bin/env bash
# Default build hook: nimble build for a package with executables (bin), else nim check of a library'\''s main module, the one a user imports.
source "$(dirname "$0")/../../_hooks.sh"
exists '*.nimble' || unknown "no .nimble file"
if grep -Eq '^[[:space:]]*(bin|namedBin)[[:space:]]*(=|\[)' ./*.nimble; then
    run nimble build -y
else
    _name="$(basename ./*.nimble .nimble)"
    _module=""
    for _candidate in "src/${_name}.nim" "${_name}.nim"; do
        [ -f "${_candidate}" ] && { _module="${_candidate}"; break; }
    done
    [ -n "${_module}" ] || unknown "no bin in the .nimble file, and no src/${_name}.nim or ${_name}.nim"
    run nim check --hints:off "${_module}"
fi
