#!/usr/bin/env bash
# Install the .NET SDK (F#): C#'s installer, which F# shares, with F#'s
# components' versions.
source "$(dirname "$0")/../_common.sh"

log "Installing F#..."
bash "$(dirname "$0")/../csharp/install.sh" fsharp
