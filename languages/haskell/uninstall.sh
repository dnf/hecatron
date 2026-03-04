#!/usr/bin/env bash
# Uninstall Haskell
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { rm -rf "$HOME/.ghcup"; }
uninstall_linux() { rm -rf "$HOME/.ghcup"; }

log "Uninstalling Haskell..."
"uninstall_${OS}"
