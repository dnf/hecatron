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

deps() { :; }

log "Installing OCaml..."
"install_${OS}"
deps
