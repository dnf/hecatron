#!/usr/bin/env bash
# Install Racket
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install minimal-racket; }
install_linux() { apt_install racket; }

deps() { :; }

log "Installing Racket..."
"install_${OS}"
deps
