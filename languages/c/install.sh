#!/usr/bin/env bash
# Install C toolchain (gcc/clang + cmake)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install cmake; }
install_linux() { apt_install build-essential cmake; }

deps() { :; }

log "Installing C..."
"install_${OS}"
deps
