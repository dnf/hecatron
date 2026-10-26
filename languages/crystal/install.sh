#!/usr/bin/env bash
# Install Crystal
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install crystal; }
install_linux() {
    if ! has crystal; then
        apt_install curl gnupg
        curl -fsSL https://crystal-lang.org/install.sh | ${SUDO} bash
    fi
}

deps() {
    if has crystal; then
        each_impl crystal shards install
    fi
}

log "Installing Crystal..."
"install_${OS}"
deps
