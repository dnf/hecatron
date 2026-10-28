#!/usr/bin/env bash
# Install Tcl
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install tcl-tk
}
install_linux() {
    # make runs a TEA extension's tests (the default test hook's make check).
    apt_install tcl make
}

deps() {
    :
}

log "Installing Tcl..."
"install_${OS}"
deps
