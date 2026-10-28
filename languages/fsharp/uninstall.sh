#!/usr/bin/env bash
# Uninstall F#. Its toolchain is the .NET SDK, which C#, F# and VB.NET share
# (csharp/install.sh installs it for all three), so it stays: the SDKs and
# their views in ~/.local/share/hecatron on macOS, in /opt/dotnet and
# /opt/dotnet-views on Linux.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { leave_dotnet; }
uninstall_linux() { leave_dotnet; }

log "Uninstalling F#..."
"uninstall_${OS}"
