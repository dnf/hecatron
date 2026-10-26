#!/usr/bin/env bash
# Install Haskell (GHC + cabal via ghcup)
source "$(dirname "$0")/../_common.sh"

_install() {
    if ! has ghc; then
        curl --proto '=https' --tlsv1.2 -sSf https://get-ghcup.haskell.org | \
            BOOTSTRAP_HASKELL_NONINTERACTIVE=1 \
            BOOTSTRAP_HASKELL_INSTALL_CABAL=1 \
            BOOTSTRAP_HASKELL_INSTALL_GHC=1 \
            BOOTSTRAP_HASKELL_ADJUST_BASHRC=1 \
            sh
    fi
}

install_macos() { _install; }
# GHC's documented build dependencies, which a fresh Debian or Ubuntu lacks.
install_linux() {
    apt_install build-essential libffi-dev libgmp-dev libncurses-dev pkg-config zlib1g-dev
    _install
}

deps() { :; }

log "Installing Haskell..."
"install_${OS}"
deps
