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

# Only libraries with a package.json have dependencies to install.
_npm_install() { [ ! -f package.json ] || npm install; }
deps() { each_impl typescript _npm_install; }

log "Installing TypeScript (Node.js)..."
"install_${OS}"
deps
