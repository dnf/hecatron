#!/usr/bin/env bash
# The latest installable release of each Python release line, newest first,
# as "version<TAB>flags" (see endoflife_releases in _common.sh). Installable
# means uv has a build of it: python.org releases reach uv's builds a little
# later, so the newest release isn't always installable yet. Without uv,
# endoflife.date's list.
source "$(dirname "$0")/../_common.sh"

export PATH="${HOME}/.local/bin:${PATH}"
if ! has uv; then
    endoflife_releases python
    exit
fi

eol_lines="$(endoflife_releases python | awk -F'\t' '$2 ~ /eol/ {split($1, v, "."); print v[1] "." v[2]}')"
uv python list --all-versions --only-downloads --output-format json | python3 -c '
import json, re, sys
eol = set(sys.argv[1].split())
latest = {}
for entry in json.load(sys.stdin):
    version = entry.get("version", "")
    if entry.get("implementation") != "cpython" or entry.get("variant", "default") != "default":
        continue
    if not re.fullmatch(r"\d+\.\d+\.\d+", version):
        continue
    parts = tuple(int(p) for p in version.split("."))
    line = parts[:2]
    if line not in latest or parts > latest[line]:
        latest[line] = parts
for line in sorted(latest, reverse=True):
    name = "%d.%d" % line
    print(".".join(map(str, latest[line])) + "\t" + ("eol" if name in eol else ""))
' "${eol_lines}"
