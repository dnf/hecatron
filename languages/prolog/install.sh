#!/usr/bin/env bash
# Install SWI-Prolog
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install swi-prolog; }
install_linux() { apt_install swi-prolog; }

deps() { :; }

log "Installing Prolog (SWI-Prolog)..."
"install_${OS}"
deps
