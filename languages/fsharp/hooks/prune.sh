#!/usr/bin/env bash
# Default prune hook: .NET's build configurations unused for 7d; with
# --caches, old package versions in NuGet's packages folder (prune_dotnet in
# languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
prune_dotnet
