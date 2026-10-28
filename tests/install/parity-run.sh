#!/usr/bin/env bash
# One side of tests/docker/parity.sh, inside a fresh container: check one
# language as tests/install/check.sh does (install with doctor --fix, doctor
# from a new login shell, the rerun, the smoke test), then print what the
# machine ended up with, for the two sides to be compared: every installed
# package and its version, where the language's commands are, and the
# doctor's report.
#
#   tests/install/parity-run.sh raku
set -uo pipefail

if [ "$#" -ne 1 ]; then
    echo "usage: $0 <lang>" >&2
    exit 2
fi
_lang="$1"
_hecatron_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

bash "${_hecatron_home}/tests/install/check.sh" "${_lang}"
_status=$?

echo "### state packages"
dpkg-query -W -f '${Package} ${Version}\n' 2>/dev/null | sort
echo "### state commands"
python3 -c '
import json, sys
config = json.load(open(sys.argv[1]))
tools = json.load(open(sys.argv[2])) if sys.argv[2] else {}
names = [config.get("dependency"), config.get("lintDependency")]
names += [((tool.get("check") or {}).get("command")) for tool in tools.get("tools", [])]
print("\n".join(sorted({name for name in names if name})))
' "${_hecatron_home}/languages/${_lang}/config.json" \
    "$([ -f "${_hecatron_home}/languages/${_lang}/install.json" ] && echo "${_hecatron_home}/languages/${_lang}/install.json")" |
    while IFS= read -r _command; do
        echo "${_command}: $(bash -lc "command -v ${_command}" 2>/dev/null || echo missing)"
    done
echo "### state doctor"
_project="$(mktemp -d)"
(
    cd "${_project}" && git init -q
    [ -n "${HECATRON_CHECK_SEED:-}" ] && cp -R "${HECATRON_CHECK_SEED}/." .
    hecatron init >/dev/null && hecatron add "${_lang}" >/dev/null
    bash -lc "cd '${_project}' && hecatron doctor ${_lang}" 2>&1
)
echo "### state end"
exit "${_status}"
