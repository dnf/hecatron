#!/usr/bin/env bash
# Uninstall Elixir
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall elixir 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y elixir 2>/dev/null || true; }

log "Uninstalling Elixir..."
"uninstall_${OS}"
