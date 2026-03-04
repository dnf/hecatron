#!/usr/bin/env bash
# Install Perl (system perl on macOS, apt on Linux)
source "$(dirname "$0")/../_common.sh"

install_macos() { log "Using system Perl (no install needed)."; }
install_linux() { apt_install perl; }

deps() { :; }

log "Installing Perl..."
"install_${OS}"
deps
