#!/usr/bin/env bash
# Install Objective-C with Foundation (Xcode's clang on macOS, GCC's
# Objective-C and GNUstep Base on Linux)
source "$(dirname "$0")/../_common.sh"

# Xcode's (or the Command Line Tools') clang has Foundation.
install_macos() {
    if xcrun --find clang >/dev/null 2>&1; then
        log "Using Xcode's clang and Foundation (no install needed)."
    else
        log "Installing the Xcode Command Line Tools..."
        xcode-select --install || true
    fi
}

# Debian and Ubuntu build GNUstep with GCC, so GCC's Objective-C (gobjc) is
# the compiler that matches its runtime; gnustep-config gives the flags.
install_linux() {
    apt_install build-essential gobjc libgnustep-base-dev gnustep-make
}

# CMake, where a component builds with it and it isn't installed.
objectivec_cmake_missing() {
    local _dir
    has cmake && return 1
    while IFS= read -r _dir; do
        [ -f "${_dir}/CMakeLists.txt" ] && return 0
    done < <(impl_dirs objectivec)
    return 1
}
deps() {
    if objectivec_cmake_missing; then
        log "Installing CMake..."
        if [ "${OS}" = "macos" ]; then brew_install cmake; else apt_install cmake; fi
    fi
    if meson_missing objectivec; then meson_install objectivec; fi
    # uv runs the pinned clang-format hecatron format uses (languages/_format.sh).
    if clang_format_missing objectivec; then uv_install; fi
}

log "Installing Objective-C..."
"install_${OS}"
deps
