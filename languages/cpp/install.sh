#!/usr/bin/env bash
# Install C++ toolchain (g++/clang++ + cmake)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install cmake; }
install_linux() { apt_install build-essential cmake pkg-config; }

deps() {
    cmake_dependencies cpp
    if meson_missing cpp; then meson_install; fi
    # uv runs the pinned clang-format hecatron format uses (languages/_format.sh).
    if clang_format_missing cpp; then uv_install; fi
}

log "Installing C++..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install
