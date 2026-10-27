#!/usr/bin/env bash
# Check one language's toolchain on this machine, as a fresh developer would
# meet it: install it with doctor --fix, check it again from a new login
# shell (so the PATH the installer persisted is what counts), rerun the
# installer to show it is idempotent, then run its smoke test
# (tests/docker/smoke/<lang>.sh, which builds and runs a small program) with
# the language's PATH entries, as its hooks see them, because a command being
# on PATH doesn't mean it works.
#
# Runs anywhere hecatron is on PATH: inside a container (tests/docker/) or
# directly on a machine, e.g. a CI runner. Prints a "### result" line for
# people and a "### coverage {json}" line for tests/coverage/collect.py.
#
#   tests/install/check.sh go
set -uo pipefail

if [ "$#" -ne 1 ]; then
    echo "usage: $0 <lang>" >&2
    exit 2
fi
_lang="$1"
_hecatron_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_smoke="${_hecatron_home}/tests/docker/smoke/${_lang}.sh"
_project="$(mktemp -d)"

_outcome() { if [ "$1" -eq 0 ]; then echo pass; else echo fail; fi; }

# A definition lists the platforms it has no toolchain for ("linux-aarch64").
_platform_key="$(uname -s | tr '[:upper:]' '[:lower:]' | sed 's/darwin/macos/')-$(uname -m | sed 's/arm64/aarch64/')"
_unsupported="$(python3 -c '
import json, sys
print(" ".join(json.load(open(sys.argv[1])).get("unsupported", [])))
' "${_hecatron_home}/languages/${_lang}/config.json")"

# The platform as the coverage matrix names it: macos-arm64,
# linux-x86_64-ubuntu24.04, linux-aarch64-debian13...
_platform() {
    local platform="${_platform_key}"
    if [ -f /etc/os-release ]; then
        platform="${platform}-$(. /etc/os-release && echo "${ID}${VERSION_ID:-}")"
    fi
    echo "${platform}"
}
_write_result() {
    python3 -c '
import json, sys, time
language, platform, install, doctor, rerun, smoke = sys.argv[1:7]
print("### coverage " + json.dumps({
    "language": language, "platform": platform, "evidence": "fresh",
    "install": install, "doctor": doctor, "rerun": rerun, "smoke": smoke,
    "date": time.strftime("%Y-%m-%d"),
}))
' "$1" "$(_platform)" "$2" "$3" "$4" "$5"
}

case " ${_unsupported} " in
    *" ${_platform_key} "*)
        echo "### result ${_lang}: unsupported on ${_platform_key}"
        _write_result "${_lang}" unsupported unsupported unsupported unsupported
        exit 0 ;;
esac

cd "${_project}" && git init -q
hecatron init >/dev/null && hecatron add "${_lang}" >/dev/null
echo "### doctor --fix ${_lang}"
hecatron doctor --fix "${_lang}"; _fix=$?
echo "### doctor ${_lang} (new login shell)"
bash -lc "cd '${_project}' && hecatron doctor ${_lang}"; _after=$?
echo "### doctor --fix ${_lang} (rerun)"
bash -lc "cd '${_project}' && hecatron doctor --fix ${_lang}" >/dev/null 2>&1; _rerun=$?
_smoke_result=none
if [ -f "${_smoke}" ]; then
    echo "### smoke ${_lang}"
    bash -lc "cd '${_project}' && eval \"\$(hecatron env ${_lang})\" && cd \"\$(mktemp -d)\" && bash '${_smoke}'"
    _smoke_result="$(_outcome $?)"
fi
echo "### result ${_lang}: install=$(_outcome "${_fix}") doctor=$(_outcome "${_after}") rerun=$(_outcome "${_rerun}") smoke=${_smoke_result}"
_write_result "${_lang}" "$(_outcome "${_fix}")" "$(_outcome "${_after}")" "$(_outcome "${_rerun}")" "${_smoke_result}"
[ "${_fix}" -eq 0 ] && [ "${_after}" -eq 0 ] && [ "${_rerun}" -eq 0 ] && [ "${_smoke_result}" != fail ]
