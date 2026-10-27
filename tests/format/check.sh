#!/usr/bin/env bash
# Check a language's default format hook on this machine: install the
# language, copy a fixture project with a misformatted file
# (tests/format/fixtures/<lang>/<fixture>/, the file named messy.*) into a new
# project as that language's directory, and check that `hecatron format
# --check` and `hecatron lint` fail naming it, `hecatron format` fixes it,
# and both then pass.
#
# Runs where hecatron is on PATH, normally a throwaway container:
#   tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/format/check.sh go
set -uo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 <lang> [fixture]" >&2
    exit 2
fi
_lang="$1"
_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_fixtures="${_home}/tests/format/fixtures/${_lang}"
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
    _results=""
    _step() {
        local _name="$1" _want="$2" _output _status
        shift 2
        echo "### ${_lang}/${_fixture}: ${_name}"
        _output="$(cd "${_project}" && "$@" 2>&1)"
        _status=$?
        printf '%s\n' "${_output}"
        if [ "${_want}" = fail ] && { [ "${_status}" -eq 0 ] || ! grep -qi messy <<<"${_output}"; }; then
            _results="${_results} ${_name}=FAIL(expected a failure naming messy)"
            _failed=1
        elif [ "${_want}" = pass ] && [ "${_status}" -ne 0 ]; then
            _results="${_results} ${_name}=FAIL(status ${_status})"
            _failed=1
        else
            _results="${_results} ${_name}=pass"
        fi
    }
    _step check-before fail hecatron format --check "${_lang}"
    _step lint-before fail hecatron lint "${_lang}"
    _step format pass hecatron format "${_lang}"
    _step check-after pass hecatron format --check "${_lang}"
    _step lint-after pass hecatron lint "${_lang}"
    echo "### format ${_lang}/${_fixture}:${_results}"
done
exit "${_failed}"
