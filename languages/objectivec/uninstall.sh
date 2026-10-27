#!/usr/bin/env bash
# Uninstall Objective-C (GNUstep and GCC's Objective-C; the C toolchain and
# CMake stay, which other languages share)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { echo "Xcode's clang — not removing."; }
uninstall_linux() { $SUDO apt-get remove -y gobjc libgnustep-base-dev gnustep-make 2>/dev/null || true; }

log "Uninstalling Objective-C..."
"uninstall_${OS}"
