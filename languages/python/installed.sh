#!/usr/bin/env bash
# The CPython versions installed on this machine, as "version<TAB>home" (home
# holds bin/python3, which UV_PYTHON names): uv's own and the system's, as uv
# finds them. Offline. Homebrew's versioned formulas (python@3.11) have only
# bin/python3.11 there, so they don't count: uv installs its own.
source "$(dirname "$0")/../_common.sh"

_list() {
    if has uv; then
        uv python list --only-installed --output-format json 2>/dev/null
    else
        python3 -c 'import json, sys; print(json.dumps([{"implementation": "cpython", "version": "%d.%d.%d" % sys.version_info[:3], "path": sys.executable}]))'
    fi
}

_list | python3 -c '
import json, os, sys
seen = set()
for entry in json.load(sys.stdin):
    if entry.get("implementation") != "cpython" or not entry.get("path"):
        continue
    home = os.path.dirname(os.path.dirname(os.path.realpath(entry["path"])))
    if not os.access(os.path.join(home, "bin", "python3"), os.X_OK):
        continue
    line = entry["version"] + "\t" + home
    if line not in seen:
        seen.add(line)
        print(line)
'
