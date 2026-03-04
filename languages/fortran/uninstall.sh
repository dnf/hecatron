#!/usr/bin/env bash
# Uninstall Fortran toolchain
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall gcc 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y gfortran 2>/dev/null || true; }

log "Uninstalling Fortran..."
"uninstall_${OS}"
