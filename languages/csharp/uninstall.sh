#!/usr/bin/env bash
# Uninstall the .NET SDK (C#): install.sh's SDKs and their link, and the
# packages earlier versions installed.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    rm -rf "${HOME}/.local/share/hecatron/dotnet"
    brew uninstall dotnet 2>/dev/null || true
}
uninstall_linux() {
    [ "$(readlink /usr/local/bin/dotnet 2>/dev/null)" = /opt/dotnet/dotnet ] && $SUDO rm -f /usr/local/bin/dotnet
    $SUDO rm -rf /opt/dotnet
    $SUDO apt-get remove -y dotnet-sdk-10.0 2>/dev/null || true
}

log "Uninstalling C#..."
"uninstall_${OS}"
