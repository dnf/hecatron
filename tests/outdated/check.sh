#!/usr/bin/env bash
# Check a language's default outdated hook on this machine: install the
# language, copy a fixture that pins an old version of one dependency (the
# deps fixtures, tests/deps/fixtures/<lang>/<fixture>/) into a new project
# as that language's directory, and check that `hecatron outdated` reports
# it and passes, that --json lists it, and that --fail fails. Needs the
# network (package registries).
#
# Runs where hecatron is on PATH, normally a throwaway container:
#   tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/outdated/check.sh go
# HECATRON_OUTDATED_REPORT_DIR saves each tool's raw report.
set -uo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 <lang> [fixture]" >&2
    exit 2
fi
_lang="$1"
_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_fixtures="${_home}/tests/deps/fixtures/${_lang}"
_names=()
if [ "$#" -eq 2 ]; then _names=("$2"); else for _name in "${_fixtures}"/*; do _names+=("$(basename "${_name}")"); done; fi

_failed=0
for _fixture in "${_names[@]}"; do
    _package="$(awk -v key="${_lang}/${_fixture}" '$1 == key { print $2 }' "${_home}/tests/outdated/expected.txt")"
    _project="$(mktemp -d)"
    cp -R "${_fixtures}/${_fixture}" "${_project}/${_lang}"
    (cd "${_project}" && git init -q && git add -A && hecatron init --impl-path '{lang}' >/dev/null &&
        hecatron add "${_lang}" >/dev/null)
    echo "### ${_lang}/${_fixture}: install"
    (cd "${_project}" && hecatron install "${_lang}") || _failed=1
    _results=""
    echo "### ${_lang}/${_fixture}: outdated"
    _output="$(cd "${_project}" && hecatron outdated "${_lang}" 2>&1)"
    _status=$?
    printf '%s\n' "${_output}"
    if [ "${_status}" -eq 0 ] && grep -qF "  ${_package} " <<<"${_output}"; then
        _results="${_results} report=pass"
    else
        _results="${_results} report=FAIL(status ${_status}; expected ${_package} listed)"
        _failed=1
    fi
    echo "### ${_lang}/${_fixture}: outdated --json"
    _json="$(cd "${_project}" && hecatron outdated --json "${_lang}" 2>/dev/null)"
    printf '%s\n' "${_json}"
    if python3 -c 'import json, sys; rows = json.loads(sys.argv[1]); sys.exit(0 if any(r["package"] == sys.argv[2] and r["latest"] for r in rows) else 1)' \
        "${_json}" "${_package}" 2>/dev/null; then
        _results="${_results} json=pass"
    else
        _results="${_results} json=FAIL"
        _failed=1
    fi
    echo "### ${_lang}/${_fixture}: outdated --fail"
    if (cd "${_project}" && hecatron outdated --fail "${_lang}" >/dev/null 2>&1); then
        _results="${_results} fail=FAIL(passed)"
        _failed=1
    else
        _results="${_results} fail=pass"
    fi
    echo "### outdated ${_lang}/${_fixture}:${_results}"
done
exit "${_failed}"
