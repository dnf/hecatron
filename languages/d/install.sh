#!/usr/bin/env bash
# Install D (LDC compiler)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install ldc; }
install_linux() {
    { has ldc2 && has dub; } || apt_install ldc dub
    # LDC links through the system C compiler, which a fresh machine lacks.
    has cc || apt_install build-essential
}

deps() { :; }

log "Installing D (LDC)..."
"install_${OS}"
deps
