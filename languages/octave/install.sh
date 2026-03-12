#!/usr/bin/env bash
# Install GNU Octave
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install octave; }
install_linux() { apt_install octave; }

log "Installing Octave..."
"install_${OS}"
