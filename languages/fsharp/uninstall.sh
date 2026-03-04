#!/usr/bin/env bash
# Uninstall .NET SDK (F#)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall dotnet 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y dotnet-sdk-10.0 2>/dev/null || true; }

log "Uninstalling F#..."
"uninstall_${OS}"
