#!/usr/bin/env bash
# Check that the installed Mojo is the version the code is pinned to.
source "$(dirname "$0")/../_common.sh"

_pinned="$(sed -n 's/^MOJO_VERSION="\(.*\)"$/\1/p' "$(dirname "$0")/install.sh")"
if has mojo && ! mojo --version 2>/dev/null | grep -qF "Mojo ${_pinned} "; then
    problem "mojo is $(mojo --version 2>/dev/null | head -1), not the pinned ${_pinned}"
fi
doctor_done
