#!/usr/bin/env bash
# Check a language's default audit hook on this machine: install the
# language, copy a fixture project with a dependency that has a known
# vulnerability (tests/audit/fixtures/<lang>/<fixture>/) into a new project
# as that language's directory, and run hecatron audit, which must find it
# and fail. Needs the network (advisory databases).
#
# Runs where hecatron is on PATH, normally a throwaway container:
#   tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/audit/check.sh go
set -uo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 <lang> [fixture]" >&2
    exit 2
fi
_lang="$1"
_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_fixtures="${_home}/tests/audit/fixtures/${_lang}"
_names=()
if [ "$#" -eq 2 ]; then _names=("$2"); else for _name in "${_fixtures}"/*; do _names+=("$(basename "${_name}")"); done; fi

_failed=0
for _fixture in "${_names[@]}"; do
    _project="$(mktemp -d)"
    cp -R "${_fixtures}/${_fixture}" "${_project}/${_lang}"
    (cd "${_project}" && git init -q && git add -A && hecatron init --impl-path '{lang}' >/dev/null &&
        hecatron add "${_lang}" >/dev/null)
    echo "### ${_lang}/${_fixture}: install"
    (cd "${_project}" && hecatron install "${_lang}") || _failed=1
    echo "### ${_lang}/${_fixture}: audit"
    _output="$(cd "${_project}" && hecatron audit --fail-on low "${_lang}" 2>&1)"
    _status=$?
    printf '%s\n' "${_output}"
    if [ "${_status}" -ne 0 ] && grep -qE ' [1-9][0-9]* vulnerabilit' <<<"${_output}"; then
        echo "### audit ${_lang}/${_fixture}: pass (found and failed)"
    else
        echo "### audit ${_lang}/${_fixture}: FAIL (status ${_status}; expected findings and a failure)"
        _failed=1
    fi
done
exit "${_failed}"
