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
        mkdir -p "$(hecatron_bin)"
        ln -sf "$HOME/v/v" "$(hecatron_bin)/v"
    fi
}

install_macos() { _install; }
# V bootstraps with make and the system C compiler, which a fresh Linux box lacks.
install_linux() {
    { has make && has cc; } || apt_install build-essential
    _install
}

deps() { :; }

log "Installing V..."
"install_${OS}"
deps
