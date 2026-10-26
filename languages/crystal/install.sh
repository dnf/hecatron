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

# Only libraries with a shard.yml have dependencies to install.
_shards_install() { [ ! -f shard.yml ] || shards install; }
deps() {
    if has crystal; then
        each_impl crystal _shards_install
    fi
}

log "Installing Crystal..."
"install_${OS}"
deps
