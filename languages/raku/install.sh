#!/usr/bin/env bash
# Install Raku (Rakudo)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install rakudo; }
install_linux() { apt_install rakudo; }

# zef, for the default hooks where a component is a distribution
# (META6.json).
deps() {
    impl_has raku -name META6.json || return 0
    has zef && return
    if [ "${OS}" = "macos" ]; then brew_install zef; else apt_install raku-zef; fi
}

log "Installing Raku (Rakudo)..."
"install_${OS}"
deps
