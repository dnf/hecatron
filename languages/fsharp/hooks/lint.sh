#!/usr/bin/env bash
# Default lint hook: Fantomas's check when the repository's dotnet tool
# manifest has it (`dotnet format` doesn't support F#).
source "$(dirname "$0")/../../_hooks.sh"
# shellcheck source=_fantomas.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_fantomas.sh"
fantomas_manifest || skip "no dotnet tool manifest lists fantomas"
run dotnet tool restore
checks_format
run dotnet fantomas --check .
