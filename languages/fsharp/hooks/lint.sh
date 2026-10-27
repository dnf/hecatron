#!/usr/bin/env bash
# Default lint hook: Fantomas's check when the repository's dotnet tool
# manifest has it (`dotnet format` doesn't support F#).
source "$(dirname "$0")/../../_hooks.sh"
fantomas_manifest() {
    local _dir
    _dir="$(pwd)"
    while :; do
        if grep -qs '"fantomas"' "${_dir}/.config/dotnet-tools.json" "${_dir}/dotnet-tools.json"; then
            return 0
        fi
        [ "${_dir}" = / ] || [ "${_dir}" = "${HECATRON_ROOT:-/}" ] && return 1
        _dir="$(dirname "${_dir}")"
    done
}
fantomas_manifest || skip "no dotnet tool manifest lists fantomas"
run dotnet tool restore
run dotnet fantomas --check .
