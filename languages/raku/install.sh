#!/usr/bin/env bash
# Install Raku (Rakudo)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install rakudo; }
install_linux() { apt_install rakudo; }

deps() { :; }

log "Installing Raku (Rakudo)..."
"install_${OS}"
deps
