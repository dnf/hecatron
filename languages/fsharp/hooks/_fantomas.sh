# Shared by the F# lint and format hooks (sourced, after _hooks.sh): Fantomas
# when the repository's dotnet tool manifest has it (`dotnet format` doesn't
# support F#).
# shellcheck shell=bash

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
