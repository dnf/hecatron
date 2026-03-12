#!/usr/bin/env bash
# Install Mojo via the Modular CLI
source "$(dirname "$0")/../_common.sh"

_install() {
    if ! has mojo; then
        curl -s https://get.modular.com | sh -
        modular install mojo
    fi
}

install_macos() { _install; }
install_linux() { _install; }

deps() { :; }

log "Installing Mojo..."
"install_${OS}"
deps
