#!/usr/bin/env bash
# Install Julia
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install julia; }
install_linux() {
    if ! has julia; then
        apt_install julia
    fi
}

deps() { :; }

log "Installing Julia..."
"install_${OS}"
deps
