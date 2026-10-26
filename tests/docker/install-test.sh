#!/usr/bin/env bash
# Install one language's toolchain on a fresh container and check it: run
# doctor --fix, then doctor again from a new login shell (so the PATH the
# installer persisted is what counts), then rerun the installer to show it is
# idempotent, and last build and run a hello world with it (smoke/<lang>.sh),
# because a command being on PATH doesn't mean it works.
#
#   tests/docker/install-test.sh ubuntu:24.04 go
set -euo pipefail

if [ "$#" -ne 2 ]; then
    echo "usage: $0 <base-image> <lang>" >&2
    exit 2
fi
_base="$1"
_lang="$2"
_here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck disable=SC2016  # expanded inside the container, not here
exec "${_here}/run.sh" "${_base}" bash -c '
set -uo pipefail
_lang="$1"
mkdir -p ~/project && cd ~/project && git init -q
hecatron init >/dev/null && hecatron add "${_lang}" >/dev/null
echo "### doctor --fix ${_lang}"
hecatron doctor --fix "${_lang}"; _fix=$?
echo "### doctor ${_lang} (new login shell)"
bash -lc "cd ~/project && hecatron doctor ${_lang}"; _after=$?
echo "### doctor --fix ${_lang} (rerun)"
bash -lc "cd ~/project && hecatron doctor --fix ${_lang}" >/dev/null 2>&1; _rerun=$?
echo "### smoke ${_lang}"
bash -lc "cd \"\$(mktemp -d)\" && bash /hecatron/tests/docker/smoke/${_lang}.sh"; _smoke=$?
echo "### result ${_lang}: fix=${_fix} doctor=${_after} rerun=${_rerun} smoke=${_smoke}"
[ "${_fix}" -eq 0 ] && [ "${_after}" -eq 0 ] && [ "${_rerun}" -eq 0 ] && [ "${_smoke}" -eq 0 ]
' _ "${_lang}"
