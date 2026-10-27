#!/usr/bin/env bash
# Install C toolchain (gcc/clang + cmake)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install cmake; }
install_linux() { apt_install build-essential cmake; }

deps() { cmake_dependencies c; }

log "Installing C..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install
