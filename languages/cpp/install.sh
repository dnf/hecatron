#!/usr/bin/env bash
# Install C++ toolchain (g++/clang++ + cmake)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install cmake; }
install_linux() { apt_install build-essential cmake; }

deps() { cmake_dependencies cpp; }

log "Installing C++..."
"install_${OS}"
deps
