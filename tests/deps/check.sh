#!/usr/bin/env bash
# Check a language's default deps hook on this machine: install the
# language, copy a fixture project with a lock file
# (tests/deps/fixtures/<lang>/<fixture>/) into a new project as that
# language's directory, and check that `hecatron deps` fetches its
# dependencies and leaves the lock file as it was. Where the hook holds the
# lock file to the manifest, tests/deps/stale/<lang>/<fixture>/ has a
# manifest that disagrees with it, and deps must then fail.
#
# Runs where hecatron is on PATH, normally a throwaway container:
#   tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/deps/check.sh go
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
    _project="$(mktemp -d)"
    cp -R "${_fixtures}/${_fixture}" "${_project}/${_lang}"
    (cd "${_project}" && git init -q && git add -A && hecatron init --impl-path '{lang}' >/dev/null &&
        hecatron add "${_lang}" >/dev/null)
    echo "### ${_lang}/${_fixture}: install"
    (cd "${_project}" && hecatron install "${_lang}") || _failed=1
    _results=""
    echo "### ${_lang}/${_fixture}: deps"
    if ! (cd "${_project}" && hecatron deps "${_lang}"); then
        _results="${_results} deps=FAIL"
        _failed=1
    elif diff -r -q "${_fixtures}/${_fixture}" "${_project}/${_lang}" | grep -v '^Only in'; then
        _results="${_results} deps=FAIL(changed the files above)"
        _failed=1
    else
        _results="${_results} deps=pass"
    fi
    _stale="${_home}/tests/deps/stale/${_lang}/${_fixture}"
    if [ -d "${_stale}" ]; then
        cp -R "${_stale}/." "${_project}/${_lang}/"
        echo "### ${_lang}/${_fixture}: deps with a stale lock file"
        if (cd "${_project}" && hecatron deps "${_lang}"); then
            _results="${_results} stale-lock=FAIL(deps passed)"
            _failed=1
        else
            _results="${_results} stale-lock=pass(refused)"
        fi
    fi
    echo "### deps ${_lang}/${_fixture}:${_results}"
done
exit "${_failed}"
