#!/usr/bin/env bash
# Install Free Pascal Compiler
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install fpc; }
install_linux() { apt_install fpc; }

deps() { :; }

log "Installing Pascal (Free Pascal Compiler)..."
"install_${OS}"
deps
