#!/usr/bin/env bash
# Install .NET SDK (VB.NET) — shares .NET SDK with C# and F#
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install dotnet; }
install_linux() {
    if ! dotnet --list-sdks 2>/dev/null | grep -q '^10\.'; then
        $SUDO apt-get update
        apt_install dotnet-sdk-10.0
    fi
}

deps() { :; }

log "Installing VB.NET..."
"install_${OS}"
deps
