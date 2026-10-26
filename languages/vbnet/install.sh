#!/usr/bin/env bash
# Install .NET SDK (VB.NET) — shares .NET SDK with C# and F#
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install dotnet; }
# The .NET SDK is C#'s; its installer knows where each distribution gets it.
install_linux() { bash "$(dirname "$0")/../csharp/install.sh"; }

deps() { :; }

log "Installing VB.NET..."
"install_${OS}"
deps
