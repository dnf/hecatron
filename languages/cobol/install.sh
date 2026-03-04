#!/usr/bin/env bash
# Install COBOL toolchain (GnuCOBOL + cmake)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install gnu-cobol cmake; }
install_linux() { apt_install gnucobol cmake; }

deps() { :; }

log "Installing COBOL..."
"install_${OS}"
deps
