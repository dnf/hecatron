#!/usr/bin/env bash
# Uninstall Fortran: gfortran and the fpm install.sh builds. CMake is shared
# with the other native languages, and Homebrew's gcc (which provides
# gfortran on macOS) with much else, so they stay.
source "$(dirname "$0")/../_common.sh"

# fpm as install.sh builds it: a plain file, not a package manager's link.
_is_built_fpm() { [ -f "$1" ] && [ ! -L "$1" ]; }

uninstall_macos() {
    local fpm
    fpm="$(brew --prefix)/bin/fpm"
    if _is_built_fpm "${fpm}"; then rm -f "${fpm}"; fi
    if brew list --versions gcc >/dev/null 2>&1; then
        left_shared "Homebrew's gcc" "it is all of GCC, not just gfortran, and other formulae use it."
    fi
    leave_cmake
}
uninstall_linux() {
    $SUDO apt-get remove -y gfortran 2>/dev/null || true
    if _is_built_fpm /usr/local/bin/fpm; then ${SUDO} rm -f /usr/local/bin/fpm; fi
    leave_cmake
}

log "Uninstalling Fortran..."
"uninstall_${OS}"
