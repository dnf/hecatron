#!/usr/bin/env bash
# The .NET SDKs installed on this machine: C#'s list, since F# shares them.
exec bash "$(dirname "$0")/../csharp/installed.sh"
