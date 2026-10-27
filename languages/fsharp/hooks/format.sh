#!/usr/bin/env bash
# Default format hook: Fantomas, when the repository's dotnet tool manifest has it.
source "$(dirname "$0")/../../_hooks.sh"
# shellcheck source=_fantomas.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_fantomas.sh"
fantomas_manifest || skip "no dotnet tool manifest lists fantomas"
run dotnet tool restore
if format_checking; then run dotnet fantomas --check .; else run dotnet fantomas .; fi
