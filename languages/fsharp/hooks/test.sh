#!/usr/bin/env bash
# Default test hook: the directory's .NET solution or project (see dotnet_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
dotnet_hook test
