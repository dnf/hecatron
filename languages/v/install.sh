#!/usr/bin/env bash
# Install V
source "$(dirname "$0")/../_common.sh"

_install() {
    if ! has v; then
        if [ -d "$HOME/v" ]; then
            cd "$HOME/v" && git pull --depth 1 && make
        else
            git clone --depth 1 https://github.com/vlang/v.git "$HOME/v"
            cd "$HOME/v" && make
        fi
        $SUDO ln -sf "$HOME/v/v" /usr/local/bin/v
    fi
}

install_macos() { _install; }
install_linux() { _install; }

deps() { :; }

log "Installing V..."
"install_${OS}"
deps
