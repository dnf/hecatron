#!/usr/bin/env bash
# Install the .NET SDK (VB.NET): C#'s installer, which VB.NET shares, with VB.NET's
# components' versions.
source "$(dirname "$0")/../_common.sh"

log "Installing VB.NET..."
bash "$(dirname "$0")/../csharp/install.sh" vbnet
