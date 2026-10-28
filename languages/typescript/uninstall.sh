#!/usr/bin/env bash
# Uninstall TypeScript. Its toolchain is Node.js (TypeScript's compiler is
# each package's own dependency), which JavaScript and Elm share, so it
# stays: Homebrew's node and the builds in ~/.local/share/hecatron/node on
# macOS, the builds in /opt/node and their links in /usr/local/bin on Linux.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { leave_node; }
uninstall_linux() { leave_node; }

log "Uninstalling TypeScript (Node.js)..."
"uninstall_${OS}"
