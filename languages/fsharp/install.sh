#!/usr/bin/env bash
# Install .NET SDK (F#) — shares .NET SDK with C#
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install dotnet; }
# The .NET SDK is C#'s; its installer knows where each distribution gets it.
install_linux() { bash "$(dirname "$0")/../csharp/install.sh"; }

deps() { :; }

log "Installing F#..."
"install_${OS}"
deps
