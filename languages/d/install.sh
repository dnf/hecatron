#!/usr/bin/env bash
# Install D (LDC compiler)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install ldc; }
install_linux() {
    if ! has ldc2; then
        apt_install ldc
    fi
}

deps() { :; }

log "Installing D (LDC)..."
"install_${OS}"
deps
