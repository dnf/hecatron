#!/usr/bin/env bash
# Install Node.js (TypeScript runtime)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install node; }
install_linux() {
    if ! has node || ! node -v | grep -q '^v2[2-9]'; then
        curl -fsSL https://deb.nodesource.com/setup_22.x | $SUDO bash -
        apt_install nodejs
    fi
}

deps() { each_impl typescript npm install; }

log "Installing TypeScript (Node.js)..."
"install_${OS}"
deps
