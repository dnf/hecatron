#!/usr/bin/env bash
# Uninstall C. Nothing C's installer adds is C's alone: the compiler is the
# system's (build-essential, Xcode's clang), and CMake is shared with the other
# native languages, so it stays.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { leave_cmake; }
uninstall_linux() { leave_cmake; }

log "Uninstalling C..."
"uninstall_${OS}"
