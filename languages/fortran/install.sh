#!/usr/bin/env bash
# Install Fortran toolchain (gfortran + cmake)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install gcc cmake; }
install_linux() { apt_install gfortran cmake; }

deps() { :; }

log "Installing Fortran..."
"install_${OS}"
deps
