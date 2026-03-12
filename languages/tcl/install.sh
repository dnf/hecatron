#!/usr/bin/env bash
# Install Tcl
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install tcl-tk
}
install_linux() {
    apt_install tcl
}

deps() {
    :
}

log "Installing Tcl..."
"install_${OS}"
deps
