#!/usr/bin/env bash
# The latest SDK of each .NET release line, newest first, as
# "version<TAB>flags" (see endoflife_releases in _common.sh), from Microsoft's
# release metadata: endoflife.date tracks the runtime's versions, not the
# SDK's. Previews are left out.
source "$(dirname "$0")/../_common.sh"

curl -fsSL https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json | python3 -c '
import json, sys
for channel in json.load(sys.stdin)["releases-index"]:
    phase = channel["support-phase"]
    if phase in ("preview", "go-live"):
        continue
    flags = []
    if channel["release-type"] == "lts" and phase != "eol":
        flags.append("lts")
    if phase == "eol":
        flags.append("eol")
    print(channel["latest-sdk"] + "\t" + ",".join(flags))
'
