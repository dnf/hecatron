#!/usr/bin/env bash
# Install Rust via rustup
source "$(dirname "$0")/../_common.sh"

_install() {
    if ! has rustc; then
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
        # shellcheck source=/dev/null
        source "$HOME/.cargo/env"
    fi
}

install_macos() { _install; }
install_linux() { _install; }

deps() { :; }

log "Installing Rust..."
"install_${OS}"
deps
