#!/usr/bin/env bash
# Default outdated hook: dotnet list package --outdated (see outdated_dotnet in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_dotnet
