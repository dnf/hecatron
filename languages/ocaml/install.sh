#!/usr/bin/env bash
# Install OCaml (opam + dune + alcotest)
source "$(dirname "$0")/../_common.sh"

install_macos() {
    if ! has opam; then
        brew_install opam
    fi
    opam init --auto-setup --disable-sandboxing --yes 2>/dev/null || true
    eval "$(opam env)"
    opam install dune alcotest --yes
}

install_linux() {
    if ! has opam; then
        apt_install opam
    fi
    opam init --auto-setup --disable-sandboxing --yes 2>/dev/null || true
    eval "$(opam env)"
    opam install dune alcotest --yes
}

# shellcheck source=_ocamlformat.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_ocamlformat.sh"
deps() {
    local _version
    while IFS= read -r _version; do
        if [ "${_version}" = "(any)" ]; then
            opam install ocamlformat --yes
        else
            opam install "ocamlformat.${_version}" --yes
        fi </dev/null
    done < <(ocamlformat_missing)
}

log "Installing OCaml..."
"install_${OS}"
deps
