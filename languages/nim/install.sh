#!/usr/bin/env bash
# Install Nim
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install nim; }
install_linux() {
    if ! has nim; then
        apt_install nim
    fi
}

deps() { :; }

log "Installing Nim..."
"install_${OS}"
deps
