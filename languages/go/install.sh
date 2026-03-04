#!/usr/bin/env bash
# Install Go
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install go; }
install_linux() {
    if ! has go; then
        ARCH="$(dpkg --print-architecture)"
        curl -fsSL "https://go.dev/dl/go1.21.13.linux-${ARCH}.tar.gz" | $SUDO tar -C /usr/local -xz
    fi
}

deps() { :; }

log "Installing Go..."
"install_${OS}"
deps
