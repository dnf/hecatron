#!/usr/bin/env bash
# Default format hook: dotnet format (see format_dotnet in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_dotnet
