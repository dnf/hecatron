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
# rustc links through the system C compiler, which a fresh Linux box lacks.
install_linux() {
    has cc || apt_install build-essential
    _install
}

deps() { :; }

log "Installing Rust..."
"install_${OS}"
deps
