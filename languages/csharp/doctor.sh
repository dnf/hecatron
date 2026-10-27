#!/usr/bin/env bash
# Check for the .NET runtimes the components' projects target (see install.sh).
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _major; do
    [ -n "${_major}" ] && problem "the .NET ${_major} runtime is not installed (a project targets net${_major}.0)"
done < <(dotnet_runtimes_missing)
doctor_done
