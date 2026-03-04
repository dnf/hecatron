#!/usr/bin/env bash
# Uninstall OCaml
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall opam 2>/dev/null || true
    rm -rf "$HOME/.opam"
}

uninstall_linux() {
    $SUDO apt-get remove -y opam 2>/dev/null || true
    rm -rf "$HOME/.opam"
}

log "Uninstalling OCaml..."
"uninstall_${OS}"
