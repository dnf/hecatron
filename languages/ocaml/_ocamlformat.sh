# Shared by the OCaml installer and doctor (sourced, not executed).
# shellcheck shell=bash

# ocamlformat where a component configures it (.ocamlformat), at the version
# its "version =" line pins, since ocamlformat refuses to run as any other.
ocamlformat_wanted() {
    local _dir _version
    while IFS= read -r _dir; do
        [ -f "${_dir}/.ocamlformat" ] || continue
        _version="$(sed -n 's/^[[:space:]]*version[[:space:]]*=[[:space:]]*\([^[:space:]]*\).*/\1/p' "${_dir}/.ocamlformat" | head -1)"
        echo "${_version:-(any)}"
    done < <(impl_dirs ocaml)
}
ocamlformat_missing() {
    local _version _installed
    _installed="$(ocamlformat --version 2>/dev/null || true)"
    while IFS= read -r _version; do
        if [ "${_version}" = "(any)" ]; then
            [ -n "${_installed}" ] || echo "${_version}"
        elif [ "${_version}" != "${_installed}" ]; then
            echo "${_version}"
        fi
    done < <(ocamlformat_wanted | sort -u)
}
